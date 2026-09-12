# RFC 4301 - Security Architecture for the Internet Protocol

Partial. Every requirement this repository extracted from RFC 4301, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 30.0% | 6 of 20 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 35.0% | 7 of 20 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 20 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 30.6% | 11 of 36 tagged units, 0 escaped and 1 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 20 | of 23 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 6 | of 20 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 30.0% | 6 of 20 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 20 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 20 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 5.0% | 1 of 20 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 20 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 23 |
| Gated MUST-level | 20 |
| Not applicable, so out of scope | 6 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 36 |
| Tagged units | 36 |
| Recorded audit verdicts | 0 |
| Discrimination records | 12 |
| Summary | `rfc/short/rfc4301.md` |
| Requirement shard | `rfc/requirements/rfc4301.md` |
| RFC text | `rfc/full/rfc4301.txt` |

## Enrolment

Enrolled: Security Architecture for IP (RFC 4301): native control-plane SPD/SAD projected to kernel XFRM; 1 MET (no null-enc/no-integrity ESP) + 7 single-polarity positive (tunnel/transport modes, SPD present, negotiated selectors, manual+automated keying) + 1 gap (port/ICMP selectors) + 6 not-applicable (per-packet datapath kernel-delegated)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Native control-plane SPD/SAD model projected to kernel XFRM. The SPD carries all three dispositions of Section 4.4.1: a negotiated Child SA produces the PROTECT entries, and the operator `vpn ipsec policy` list produces the BYPASS and DISCARD entries, each with a selector, a direction and a total order. A selector holds the local and remote prefixes, the next-layer protocol, and a local and a remote port as an any-or-one-exact value. IKEv2 Child SAs install tunnel mode, or transport mode when USE_TRANSPORT_MODE is negotiated and echoed, and the RFC 4552 OSPFv3 path installs manual transport-mode ESP or AH. SAD entries carry the SPI, the address pair, the mode, the protocol, the algorithms, the replay window and a time-based lifetime. Per-packet SPD and SAD processing is kernel-delegated to XFRM.

**What the ledger says remains**

Lowered from `Supported` on 2026-08-30, after an extraction walk of [`rfc/full/rfc4301.txt`](https://github.com/ze-software/ze/blob/main/rfc/full/rfc4301.txt) found architecture obligations Ze does not meet. The implementation work is [`plan/immediate/spec-rfc4301-architecture-gaps.md`](https://github.com/ze-software/ze/blob/main/plan/immediate/spec-rfc4301-architecture-gaps.md), one phase per block.

- **Section 6, ICMP processing:** no control lets an administrator accept or reject unauthenticated ICMP error messages per ICMP type, and no check compares a protected transit ICMP error message payload header against the traffic selectors of the SA that carried it.
- **Section 8, DF bit and PMTU:** the DF treatment of a tunnel-mode SA is not configurable, and no per-SA PMTU value is held or aged. Section 4.4.2.1, SAD lifetimes: `newLifetimeState` ([`internal/component/ike/engine/rekey.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey.go)) assigns a time lifetime only, `softBytes` is never assigned, and the byte-count arm of `softExpired` is unreachable in a running daemon; [`plan/spec-ipsec-lifetime-volume.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-lifetime-volume.md) designs the byte-count lifetime. Section 5.1.2.1, outer header: the DSCP value of the outer tunnel header is not mapped for the domain the packet enters. Section 4.4.1.1, selectors: a port selector holds any port or one exact port rather than the range the section defines, and `SPParams` carries no ICMP type or code field. Only that last item is gated in [`rfc/short/rfc4301.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4301.md), as [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1), and its reason text is stale about ports.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 14 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **20** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC4301-4.2-1`](#rfc4301-4.2-1), [`RFC4301-4.4.3.1-1`](#rfc4301-4.4.3.1-1), [`RFC4301-4.4.3.1-2`](#rfc4301-4.4.3.1-2), [`RFC4301-4.4.1-4`](#rfc4301-4.4.1-4), [`RFC4301-7.4-1`](#rfc4301-7.4-1), [`RFC4301-7.4-2`](#rfc4301-7.4-2)

**Annotated instead of tested (14):** [`RFC4301-4.1-1`](#rfc4301-4.1-1), [`RFC4301-4.1-2`](#rfc4301-4.1-2), [`RFC4301-4.1-3`](#rfc4301-4.1-3), [`RFC4301-4.1-4`](#rfc4301-4.1-4), [`RFC4301-4.1-5`](#rfc4301-4.1-5), [`RFC4301-4.4.1-1`](#rfc4301-4.4.1-1), [`RFC4301-4.4.1-2`](#rfc4301-4.4.1-2), [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1), [`RFC4301-4.4.2-1`](#rfc4301-4.4.2-1), [`RFC4301-4.5-1`](#rfc4301-4.5-1), [`RFC4301-5.2-1`](#rfc4301-5.2-1), [`RFC4301-5.2-2`](#rfc4301-5.2-2), [`RFC4301-4.1-6`](#rfc4301-4.1-6), [`RFC4301-7-1`](#rfc4301-7-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4301-4.1-1` | Host implementations MUST support both transport and tunnel mode (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L177). **positive:** `unit/verify` [`TestIPsecSAIsWildcardWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L149). **negative:** no negative test. **{single-polarity}:** ze projects both modes -- IKE child SAs install tunnel-mode ESP and OSPFv3 RFC 4552 installs transport-mode ESP/AH; a capability-presence MUST has no meaningful negative (internal/component/ike/engine/child.go:224, :253, internal/plugins/ospf/ipsec_install.go:413, :441) |
| `RFC4301-4.1-2` | Security gateways MUST support tunnel mode (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L192). **negative:** no negative test. **{single-polarity}:** ze (a security gateway) installs tunnel-mode ESP for every IKE-negotiated peer SA and policy (internal/component/ike/engine/child.go:224, :253, :281, :295) |
| `RFC4301-4.1-3` | SAs between a security gateway and any peer MUST use tunnel mode (two narrow exceptions for gateway-as-host) (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L179). **negative:** no negative test. **{single-polarity}:** the IKE child-SA path hardcodes tunnel mode for every peer SA, so a peer SA can never be transport (internal/component/ike/engine/child.go:40, :224, :253) |
| `RFC4301-4.1-4` | IKE-created SA pairs MUST use the same mode (both tunnel or both transport) (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L181). **negative:** no negative test. **{single-polarity}:** the inbound and outbound child SAs of a pair are both built with modeTunnel, so the pair is always same-mode (internal/component/ike/engine/child.go:224, :253) |
| `RFC4301-4.1-5` | Implementation MUST permit multiple SAs between same endpoints with same selectors (QoS differentiation) (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** multiple SAs with identical selectors coexist by SPI in the kernel XFRM SAD and ze's SPI-keyed installs do not forbid this; the DSCP classification that selects among them for QoS is a datapath function ze does not model (internal/component/ike/dataplane/dataplane.go:81-108) |
| `RFC4301-4.2-1` | MUST NOT instantiate an ESP SA with both NULL encryption and no integrity algorithm (§4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L124). **negative:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L102) |
| `RFC4301-4.4.1-1` | Implementation MUST have at least one SPD (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L158). **negative:** no negative test. **{single-polarity}:** ze maintains an SPD policy model (SPParams) and installs at least the PROTECT entries into the kernel SPD via both the IKE and OSPFv3 paths (internal/component/ike/dataplane/dataplane.go:145, engine/child.go:276, :290, internal/plugins/ospf/ipsec_install.go:432-452) |
| `RFC4301-4.4.1-2` | SPD MUST be consulted for ALL traffic crossing IPsec boundary, including IKE management traffic (§4.4.1, §5) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** per-packet SPD consultation for every crossing packet is a kernel XFRM datapath function; ze populates the kernel SPD but does not process packets |
| `RFC4301-4.4.1.1-1` | All implementations MUST support the defined selectors: remote/local IP, next-layer protocol, ports, ICMP type/code (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's policy model carries IP-prefix and next-layer-protocol selectors only; the IKE traffic-selector negotiation discards ports/protocol into an address-only net.IPNet, and SPParams has no port or ICMP-type/code field (internal/component/ike/dataplane/dataplane.go:110-130 exists; ports/ICMP missing at internal/component/ike/engine/sa.go:128-129, engine/initiator.go:325) |
| `RFC4301-4.4.2-1` | Inbound SAD entries MUST be populated with negotiated selector values for packet verification (§4.4.2) | MUST | 4.4.2 | **positive:** `unit/verify` [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L420). **positive:** `unit/verify` [`TestNarrowedSelectorsReachTheInstalledPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L423). **negative:** no negative test. **{single-polarity}:** ze captures the RFC 7296-narrowed negotiated traffic selectors and projects them into the inbound require-policy; the per-packet check against them is kernel-enforced (internal/component/ike/engine/child.go:155-164, :276-288) |
| `RFC4301-4.5-1` | Implementations MUST support both manual and automated (IKEv2) key management (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L152). **positive:** `unit/verify` [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L62). **negative:** no negative test. **{single-polarity}:** ze implements automated keying via its native IKEv2 engine and manual keying via the OSPFv3 RFC 4552 config, both installing SAs through the same dataplane seam (internal/component/ike/engine/child.go:199-307, internal/plugins/ospf/ipsec_install.go:295-342) |
| `RFC4301-5.2-1` | Inbound packets not matching SPD-I MUST be discarded (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** dropping inbound packets that fail the SPD-I match is a kernel XFRM datapath function; ze installs the inbound require-policies but does not process packets (internal/component/ike/engine/child.go:276) |
| `RFC4301-5.2-2` | After decapsulation, inner packet selectors MUST be verified against SAD traffic selectors (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** post-decapsulation inner-selector verification against the SAD is a kernel XFRM datapath function; the kernel verifies the inner packet against the inbound policy ze installs |
| `RFC4301-4.1-6` | Multicast-capable implementations MUST support multicast SAD lookup (three-step: SPI+dst+src, SPI+dst, SPI alone) (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the three-step inbound SAD lookup (SPI+dst+src / SPI+dst / SPI) is a per-packet kernel XFRM function; ze performs no inbound SAD lookups |
| `RFC4301-7-1` | AH/ESP MUST NOT be applied in transport mode to IPv4 fragments; use tunnel mode (§7) | MUST NOT | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies transport mode only to IPv6 OSPFv3 traffic (IPv4-family IPsec is rejected at config), so transport-mode IPsec on an IPv4 fragment structurally cannot arise, and per-packet fragment handling is kernel-delegated (internal/plugins/ospf/config_ipsec.go:21-22) |
| `RFC4301-4.4.1-3` | SPD SHOULD have a default final entry that discards unmatched traffic (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7-2` | Encapsulator SHOULD perform Path MTU Discovery and adjust tunnel MTU (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.1-7` | Security gateways MAY support transport mode only when acting as a host (e.g., SNMP management traffic) (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.3.1-1` | Sub-tree matching MUST be supported for distinguished names, domain names and RFC 822 e-mail addresses in PAD entries (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** `unit/verify` [`TestPadSubtreeAdmitsAPeerBeneathIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L35). **positive:** `unit/verify` [`TestPadSubtreeStillBindsTheCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L185). **positive:** `unit/verify` [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/validate_identity_test.go#L166). **negative:** `unit/verify` [`TestPadKeyIDStaysExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L111). **negative:** `unit/verify` [`TestPadSubtreeRefusesAPeerOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L69) |
| `RFC4301-4.4.3.1-2` | For IPv4 and IPv6 addresses in PAD entries, the same address range syntax used for SPD entries MUST be supported (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** `unit/verify` [`TestPadAddressRangeAdmitsAnAddressInsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L129). **positive:** `unit/verify` [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/validate_identity_test.go#L167). **negative:** `unit/verify` [`TestPadAddressRangeRefusesWhatIsOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L155) |
| `RFC4301-4.4.1-4` | A user or administrator MUST be able to order the SPD entries, and the management interface MUST support (total) ordering of them as seen via that interface (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestSPDPolicyMirrorsTheInboundSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L94). **positive:** `unit/verify` [`TestSpdOperatorOrdersOverlappingPeers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L20). **negative:** `unit/verify` [`TestSPDPolicyOrderInsideBypassBandRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L123). **negative:** `unit/verify` [`TestSpdOrderCannotCaptureTheIKEControlPlane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L107). **negative:** `unit/verify` [`TestSpdUnstatedOrderTakesTheDefaultRank`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L67) |
| `RFC4301-7.4-1` | All implementations MUST support DISCARDing of fragments using the normal SPD packet classification mechanisms (§7.4) | MUST | 7.4 | **positive:** `unit/verify` [`TestDiscardPolicyReachesTheBackend`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L45). **positive:** `unit/verify` [`TestDiscardPolicyReachesTheKernelAsBlock`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L41). **positive:** `unit/verify` [`TestPolicyReadbackReportsDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L98). **positive:** `unit/verify` [`TestSPDPolicyCarriesTheDiscardDisposition`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L52). **positive:** `unit/verify` [`TestVPPSPDActionCarriesEveryDisposition`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_vpp_test.go#L23). **negative:** `unit/verify` [`TestBypassPolicyIsNotBlocked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L64). **negative:** `unit/verify` [`TestBypassPolicyReachesTheBackendAsBypass`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L75). **negative:** `unit/verify` [`TestSPDPolicyBypassIsNotADiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L83) |
| `RFC4301-7.4-2` | All implementations MUST support stateful fragment checking to accommodate BYPASS traffic for which a non-trivial port range is specified (§7.4; Appendix D.4 restates it as "implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified") | MUST | 7.4 | **positive:** `unit/verify` [`TestPortScopedBypassNeverReachesTheForwardPath`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_test.go#L74). **positive:** `unit/verify` [`TestXFRMPortScopedBypassIsNeverStoredOnTheForwardPath`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_fragment_integration_linux_test.go#L69). **negative:** `unit/verify` [`TestIKEBypassIsPortScopedSoSection74Binds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_test.go#L133) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4301-4.1-5`](#rfc4301-4.1-5) Implementation MUST permit multiple SAs between same endpoints with same selectors (QoS differentiation) (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: multiple SAs with identical selectors coexist by SPI in the kernel XFRM SAD and ze's SPI-keyed installs do not forbid this; the DSCP classification that selects among them for QoS is a datapath function ze does not model (internal/component/ike/dataplane/dataplane.go:81-108) |
| [`RFC4301-4.4.1-2`](#rfc4301-4.4.1-2) SPD MUST be consulted for ALL traffic crossing IPsec boundary, including IKE management traffic (§4.4.1, §5) | no test | no test carries this requirement id; annotated {not-applicable}: per-packet SPD consultation for every crossing packet is a kernel XFRM datapath function; ze populates the kernel SPD but does not process packets |
| [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1) All implementations MUST support the defined selectors: remote/local IP, next-layer protocol, ports, ICMP type/code (§4.4.1.1) | {gap}, no test | ze's policy model carries IP-prefix and next-layer-protocol selectors only; the IKE traffic-selector negotiation discards ports/protocol into an address-only net.IPNet, and SPParams has no port or ICMP-type/code field (internal/component/ike/dataplane/dataplane.go:110-130 exists; ports/ICMP missing at internal/component/ike/engine/sa.go:128-129, engine/initiator.go:325) |
| [`RFC4301-5.2-1`](#rfc4301-5.2-1) Inbound packets not matching SPD-I MUST be discarded (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: dropping inbound packets that fail the SPD-I match is a kernel XFRM datapath function; ze installs the inbound require-policies but does not process packets (internal/component/ike/engine/child.go:276) |
| [`RFC4301-5.2-2`](#rfc4301-5.2-2) After decapsulation, inner packet selectors MUST be verified against SAD traffic selectors (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: post-decapsulation inner-selector verification against the SAD is a kernel XFRM datapath function; the kernel verifies the inner packet against the inbound policy ze installs |
| [`RFC4301-4.1-6`](#rfc4301-4.1-6) Multicast-capable implementations MUST support multicast SAD lookup (three-step: SPI+dst+src, SPI+dst, SPI alone) (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: the three-step inbound SAD lookup (SPI+dst+src / SPI+dst / SPI) is a per-packet kernel XFRM function; ze performs no inbound SAD lookups |
| [`RFC4301-7-1`](#rfc4301-7-1) AH/ESP MUST NOT be applied in transport mode to IPv4 fragments; use tunnel mode (§7) | no test | no test carries this requirement id; annotated {not-applicable}: ze applies transport mode only to IPv6 OSPFv3 traffic (IPv4-family IPsec is rejected at config), so transport-mode IPsec on an IPv4 fragment structurally cannot arise, and per-packet fragment handling is kernel-delegated (internal/plugins/ospf/config_ipsec.go:21-22) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4301-4.1-1`](#rfc4301-4.1-1)

Host implementations MUST support both transport and tunnel mode (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L177) | unit/verify | unproven |
| positive | [`TestIPsecSAIsWildcardWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L149) | unit/verify | unproven |

### [`RFC4301-4.1-2`](#rfc4301-4.1-2)

Security gateways MUST support tunnel mode (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L192) | unit/verify | unproven |

### [`RFC4301-4.1-3`](#rfc4301-4.1-3)

SAs between a security gateway and any peer MUST use tunnel mode (two narrow exceptions for gateway-as-host) (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L179) | unit/verify | unproven |

### [`RFC4301-4.1-4`](#rfc4301-4.1-4)

IKE-created SA pairs MUST use the same mode (both tunnel or both transport) (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L181) | unit/verify | unproven |

### [`RFC4301-4.1-5`](#rfc4301-4.1-5)

Implementation MUST permit multiple SAs between same endpoints with same selectors (QoS differentiation) (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-5, so no unit is bound to it.

### [`RFC4301-4.2-1`](#rfc4301-4.2-1)

MUST NOT instantiate an ESP SA with both NULL encryption and no integrity algorithm (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L102) | unit/verify | unproven |
| positive | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L124) | unit/verify | unproven |

### [`RFC4301-4.4.1-1`](#rfc4301-4.4.1-1)

Implementation MUST have at least one SPD (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L158) | unit/verify | unproven |

### [`RFC4301-4.4.1-2`](#rfc4301-4.4.1-2)

SPD MUST be consulted for ALL traffic crossing IPsec boundary, including IKE management traffic (§4.4.1, §5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-2, so no unit is bound to it.

### [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1)

All implementations MUST support the defined selectors: remote/local IP, next-layer protocol, ports, ICMP type/code (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-1, so no unit is bound to it.

### [`RFC4301-4.4.2-1`](#rfc4301-4.4.2-1)

Inbound SAD entries MUST be populated with negotiated selector values for packet verification (§4.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L420) | unit/verify | unproven |
| positive | [`TestNarrowedSelectorsReachTheInstalledPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L423) | unit/verify | unproven |

### [`RFC4301-4.5-1`](#rfc4301-4.5-1)

Implementations MUST support both manual and automated (IKEv2) key management (§4.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L152) | unit/verify | unproven |
| positive | [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L62) | unit/verify | unproven |

### [`RFC4301-5.2-1`](#rfc4301-5.2-1)

Inbound packets not matching SPD-I MUST be discarded (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.2-1, so no unit is bound to it.

### [`RFC4301-5.2-2`](#rfc4301-5.2-2)

After decapsulation, inner packet selectors MUST be verified against SAD traffic selectors (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.2-2, so no unit is bound to it.

### [`RFC4301-4.1-6`](#rfc4301-4.1-6)

Multicast-capable implementations MUST support multicast SAD lookup (three-step: SPI+dst+src, SPI+dst, SPI alone) (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-6, so no unit is bound to it.

### [`RFC4301-7-1`](#rfc4301-7-1)

AH/ESP MUST NOT be applied in transport mode to IPv4 fragments; use tunnel mode (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7-1, so no unit is bound to it.

### [`RFC4301-4.4.3.1-1`](#rfc4301-4.4.3.1-1)

Sub-tree matching MUST be supported for distinguished names, domain names and RFC 822 e-mail addresses in PAD entries (§4.4.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPadKeyIDStaysExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L111) | unit/verify | unproven |
| negative | [`TestPadSubtreeRefusesAPeerOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L69) | unit/verify | unproven |
| positive | [`TestPadSubtreeAdmitsAPeerBeneathIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L35) | unit/verify | unproven |
| positive | [`TestPadSubtreeStillBindsTheCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L185) | unit/verify | unproven |
| positive | [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/validate_identity_test.go#L166) | unit/verify | unproven |

### [`RFC4301-4.4.3.1-2`](#rfc4301-4.4.3.1-2)

For IPv4 and IPv6 addresses in PAD entries, the same address range syntax used for SPD entries MUST be supported (§4.4.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPadAddressRangeRefusesWhatIsOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L155) | unit/verify | unproven |
| positive | [`TestPadAddressRangeAdmitsAnAddressInsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L129) | unit/verify | unproven |
| positive | [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/validate_identity_test.go#L167) | unit/verify | unproven |

### [`RFC4301-4.4.1-4`](#rfc4301-4.4.1-4)

A user or administrator MUST be able to order the SPD entries, and the management interface MUST support (total) ordering of them as seen via that interface (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSpdOrderCannotCaptureTheIKEControlPlane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L107) | unit/verify | unproven |
| negative | [`TestSpdUnstatedOrderTakesTheDefaultRank`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L67) | unit/verify | unproven |
| negative | [`TestSPDPolicyOrderInsideBypassBandRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L123) | unit/verify | revert, verified |
| positive | [`TestSPDPolicyMirrorsTheInboundSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestSpdOperatorOrdersOverlappingPeers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L20) | unit/verify | unproven |

### [`RFC4301-7.4-1`](#rfc4301-7.4-1)

All implementations MUST support DISCARDing of fragments using the normal SPD packet classification mechanisms (§7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBypassPolicyIsNotBlocked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L64) | unit/verify | revert, verified |
| negative | [`TestBypassPolicyReachesTheBackendAsBypass`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L75) | unit/verify | revert, verified |
| negative | [`TestSPDPolicyBypassIsNotADiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestDiscardPolicyReachesTheKernelAsBlock`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestPolicyReadbackReportsDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L98) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestVPPSPDActionCarriesEveryDisposition`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_vpp_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestDiscardPolicyReachesTheBackend`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestSPDPolicyCarriesTheDiscardDisposition`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L52) | unit/verify | revert, verified |

### [`RFC4301-7.4-2`](#rfc4301-7.4-2)

All implementations MUST support stateful fragment checking to accommodate BYPASS traffic for which a non-trivial port range is specified (§7.4; Appendix D.4 restates it as "implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified")

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIKEBypassIsPortScopedSoSection74Binds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_test.go#L133) | unit/verify | revert, verified |
| positive | [`TestXFRMPortScopedBypassIsNeverStoredOnTheForwardPath`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_fragment_integration_linux_test.go#L69) | unit/verify | unproven |
| positive | [`TestPortScopedBypassNeverReachesTheForwardPath`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_test.go#L74) | unit/verify | revert, verified |

## Extraction sign-off

No extraction sign-off exists for RFC 4301, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4301, so its obligations are stated where they were written.
