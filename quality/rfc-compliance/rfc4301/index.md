# RFC 4301 - Security Architecture for the Internet Protocol

Partial. Every requirement this repository extracted from RFC 4301, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7.7% | 6 of 78 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 9.0% | 7 of 78 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 78 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 38.9% | 14 of 36 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 78 | of 81 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 6 | of 78 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 7.7% | 6 of 78 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 78 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 78 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 75.6% | 59 of 78 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 78 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 81 |
| Gated MUST-level | 78 |
| Not applicable, so out of scope | 6 |
| Declared gaps | 1 |
| Gated with no test | 58 |
| Nightly-only evidence | 0 |
| Test tags | 36 |
| Tagged units | 36 |
| Recorded audit verdicts | 0 |
| Discrimination records | 14 |
| Summary | `rfc/short/rfc4301.md` |
| Requirement shard | `rfc/requirements/rfc4301.md` |
| RFC text | `rfc/full/rfc4301.txt` |

## Enrolment

Enrolled: Security Architecture for IP (RFC 4301): native control-plane SPD/SAD projected to kernel XFRM. The extraction walk of 2026-09-21 read all 89 normative sites in `rfc/full/rfc4301.txt` and grew the checklist from 24 rows to 81 (72 MUST, 6 MUST NOT, 2 SHOULD, 1 MAY). 14 rows carry an annotation from the 2026-08-30 pass: 1 gap (port/ICMP selectors), 7 single-polarity positive (tunnel/transport modes, SPD present, negotiated selectors, manual+automated keying), 6 not-applicable (per-packet datapath kernel-delegated). The 57 rows added on 2026-09-21 carry no annotation and no test, so they read as untested obligations on the ledger.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Native control-plane SPD/SAD model projected to kernel XFRM. The SPD carries all three dispositions of Section 4.4.1: a negotiated Child SA produces the PROTECT entries, and the operator `vpn ipsec policy` list produces the BYPASS and DISCARD entries, each with a selector, a direction and a total order. A selector holds the local and remote prefixes, the next-layer protocol, and a local and a remote port as an any-or-one-exact value. IKEv2 Child SAs install tunnel mode, or transport mode when USE_TRANSPORT_MODE is negotiated and echoed, and the RFC 4552 OSPFv3 path installs manual transport-mode ESP or AH. SAD entries carry the SPI, the address pair, the mode, the protocol, the algorithms, the replay window and a time-based lifetime. Per-packet SPD and SAD processing is kernel-delegated to XFRM. The 2026-09-21 walk widened the checklist to the whole document, so this paragraph now describes the covered part of a much larger row set rather than most of it.

**What the ledger says remains**

Lowered from `Supported` on 2026-08-30, after an extraction walk of [`rfc/full/rfc4301.txt`](https://github.com/ze-software/ze/blob/main/rfc/full/rfc4301.txt) found architecture obligations Ze does not meet. The implementation work is [`plan/immediate/spec-rfc4301-architecture-gaps.md`](https://github.com/ze-software/ze/blob/main/plan/immediate/spec-rfc4301-architecture-gaps.md), one phase per block.

- **Section 6, ICMP processing:** no control lets an administrator accept or reject unauthenticated ICMP error messages per ICMP type, and no check compares a protected transit ICMP error message payload header against the traffic selectors of the SA that carried it.
- **Section 8, DF bit and PMTU:** the DF treatment of a tunnel-mode SA is not configurable, and no per-SA PMTU value is held or aged. Section 4.4.2.1, SAD lifetimes: `newLifetimeState` ([`internal/component/ike/engine/rekey.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey.go)) assigns a time lifetime only, `softBytes` is never assigned, and the byte-count arm of `softExpired` is unreachable in a running daemon; [`plan/spec-ipsec-lifetime-volume.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-lifetime-volume.md) designs the byte-count lifetime. Section 5.1.2.1, outer header: the DSCP value of the outer tunnel header is not mapped for the domain the packet enters. Section 4.4.1.1, selectors: a port selector holds any port or one exact port rather than the range the section defines, and `SPParams` carries no ICMP type or code field. Each of those blocks is now gated by a checklist row of its own, added by the 2026-09-21 extraction walk: [`RFC4301-6.1.1-1`](#rfc4301-6.1.1-1) and [`RFC4301-6.1.1-2`](#rfc4301-6.1.1-2) for the unauthenticated-ICMP control, [`RFC4301-6.2-1`](#rfc4301-6.2-1) for the ICMP error payload check, [`RFC4301-8.1-1`](#rfc4301-8.1-1) and [`RFC4301-8.1-2`](#rfc4301-8.1-2) for the DF bit, [`RFC4301-8.2.2-1`](#rfc4301-8.2.2-1) for PMTU ageing, [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2) for the byte-count lifetime, and [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1) for the outer-header DSCP mapping. [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1) remains the gated selector row and its reason text is stale about ports. The same walk added 49 further rows, for obligations no earlier pass had recorded at all: the Section 3 connectivity and ESP-support rows, the Section 4.4.1 SPD management interface and decorrelation rows, the Section 4.4.1.1 OPAQUE and ICMP-range selector rows, the Section 4.4.2.1 SAD data items, the Section 4.4.3 PAD rows, the Section 4.5.2 key-splitting rule, the Section 5 processing-step rows, and the Section 7 fragment-handling rows. None of them carries a test.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 14 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 58 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **78** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC4301-4.2-1`](#rfc4301-4.2-1), [`RFC4301-4.4.3.1-1`](#rfc4301-4.4.3.1-1), [`RFC4301-4.4.3.1-2`](#rfc4301-4.4.3.1-2), [`RFC4301-4.4.1-4`](#rfc4301-4.4.1-4), [`RFC4301-7.4-1`](#rfc4301-7.4-1), [`RFC4301-7.4-2`](#rfc4301-7.4-2)

**Annotated instead of tested (14):** [`RFC4301-4.1-1`](#rfc4301-4.1-1), [`RFC4301-4.1-2`](#rfc4301-4.1-2), [`RFC4301-4.1-3`](#rfc4301-4.1-3), [`RFC4301-4.1-4`](#rfc4301-4.1-4), [`RFC4301-4.1-5`](#rfc4301-4.1-5), [`RFC4301-4.4.1-1`](#rfc4301-4.4.1-1), [`RFC4301-4.4.1-2`](#rfc4301-4.4.1-2), [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1), [`RFC4301-4.4.2-1`](#rfc4301-4.4.2-1), [`RFC4301-4.5-1`](#rfc4301-4.5-1), [`RFC4301-5.2-1`](#rfc4301-5.2-1), [`RFC4301-5.2-2`](#rfc4301-5.2-2), [`RFC4301-4.1-6`](#rfc4301-4.1-6), [`RFC4301-7-1`](#rfc4301-7-1)

**No test and no annotation (58):** [`RFC4301-3.1-1`](#rfc4301-3.1-1), [`RFC4301-3.2-1`](#rfc4301-3.2-1), [`RFC4301-4-1`](#rfc4301-4-1), [`RFC4301-4.1-8`](#rfc4301-4.1-8), [`RFC4301-4.1-9`](#rfc4301-4.1-9), [`RFC4301-4.1-10`](#rfc4301-4.1-10), [`RFC4301-4.1-11`](#rfc4301-4.1-11), [`RFC4301-4.4-1`](#rfc4301-4.4-1), [`RFC4301-4.4.1-5`](#rfc4301-4.4.1-5), [`RFC4301-4.4.1-6`](#rfc4301-4.4.1-6), [`RFC4301-4.4.1-7`](#rfc4301-4.4.1-7), [`RFC4301-4.4.1-8`](#rfc4301-4.4.1-8), [`RFC4301-4.4.1-9`](#rfc4301-4.4.1-9), [`RFC4301-4.4.1-10`](#rfc4301-4.4.1-10), [`RFC4301-4.4.1.1-2`](#rfc4301-4.4.1.1-2), [`RFC4301-4.4.1.1-3`](#rfc4301-4.4.1.1-3), [`RFC4301-4.4.1.1-4`](#rfc4301-4.4.1.1-4), [`RFC4301-4.4.1.1-5`](#rfc4301-4.4.1.1-5), [`RFC4301-4.4.1.2-1`](#rfc4301-4.4.1.2-1), [`RFC4301-4.4.2.1-1`](#rfc4301-4.4.2.1-1), [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2), [`RFC4301-4.4.2.1-3`](#rfc4301-4.4.2.1-3), [`RFC4301-4.4.3.1-3`](#rfc4301-4.4.3.1-3), [`RFC4301-4.4.3.2-1`](#rfc4301-4.4.3.2-1), [`RFC4301-4.4.3.3-1`](#rfc4301-4.4.3.3-1), [`RFC4301-4.5.2-1`](#rfc4301-4.5.2-1), [`RFC4301-4.5.3-1`](#rfc4301-4.5.3-1), [`RFC4301-5-1`](#rfc4301-5-1), [`RFC4301-5.1-1`](#rfc4301-5.1-1), [`RFC4301-5.1-2`](#rfc4301-5.1-2), [`RFC4301-5.1-3`](#rfc4301-5.1-3), [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1), [`RFC4301-5.2-3`](#rfc4301-5.2-3), [`RFC4301-5.2-4`](#rfc4301-5.2-4), [`RFC4301-5.2-5`](#rfc4301-5.2-5), [`RFC4301-6-1`](#rfc4301-6-1), [`RFC4301-6.1.1-1`](#rfc4301-6.1.1-1), [`RFC4301-6.1.1-2`](#rfc4301-6.1.1-2), [`RFC4301-6.1.2-1`](#rfc4301-6.1.2-1), [`RFC4301-6.2-1`](#rfc4301-6.2-1), [`RFC4301-6.2-2`](#rfc4301-6.2-2), [`RFC4301-6.2-3`](#rfc4301-6.2-3), [`RFC4301-6.2-4`](#rfc4301-6.2-4), [`RFC4301-6.2-5`](#rfc4301-6.2-5), [`RFC4301-7.1-1`](#rfc4301-7.1-1), [`RFC4301-7.1-2`](#rfc4301-7.1-2), [`RFC4301-7.1-3`](#rfc4301-7.1-3), [`RFC4301-7.2-1`](#rfc4301-7.2-1), [`RFC4301-7.2-2`](#rfc4301-7.2-2), [`RFC4301-7.3-1`](#rfc4301-7.3-1), [`RFC4301-7.3-2`](#rfc4301-7.3-2), [`RFC4301-7.3-3`](#rfc4301-7.3-3), [`RFC4301-7.3-4`](#rfc4301-7.3-4), [`RFC4301-7.3-5`](#rfc4301-7.3-5), [`RFC4301-8.1-1`](#rfc4301-8.1-1), [`RFC4301-8.1-2`](#rfc4301-8.1-2), [`RFC4301-8.2.1-1`](#rfc4301-8.2.1-1), [`RFC4301-8.2.2-1`](#rfc4301-8.2.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4301-4.1-1` | Host implementations MUST support both transport and tunnel mode (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L169). **positive:** `unit/verify` [`TestIPsecSAPerDestinationWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L226). **negative:** no negative test. **{single-polarity}:** ze projects both modes -- IKE child SAs install tunnel-mode ESP and OSPFv3 RFC 4552 installs transport-mode ESP/AH; a capability-presence MUST has no meaningful negative (internal/component/ike/engine/child.go:224, :253, internal/plugins/ospf/ipsec_install.go:413, :441) |
| `RFC4301-4.1-2` | Security gateways MUST support tunnel mode (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L184). **negative:** no negative test. **{single-polarity}:** ze (a security gateway) installs tunnel-mode ESP for every IKE-negotiated peer SA and policy (internal/component/ike/engine/child.go:224, :253, :281, :295) |
| `RFC4301-4.1-3` | SAs between a security gateway and any peer MUST use tunnel mode (two narrow exceptions for gateway-as-host) (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L171). **negative:** no negative test. **{single-polarity}:** the IKE child-SA path hardcodes tunnel mode for every peer SA, so a peer SA can never be transport (internal/component/ike/engine/child.go:40, :224, :253) |
| `RFC4301-4.1-4` | IKE-created SA pairs MUST use the same mode (both tunnel or both transport) (§4.1). The source sentence is indicative: "IKE creates pairs of SAs, so for simplicity, we choose to require that both SAs in a pair be of the same mode, transport or tunnel." | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L173). **negative:** no negative test. **{single-polarity}:** the inbound and outbound child SAs of a pair are both built with modeTunnel, so the pair is always same-mode (internal/component/ike/engine/child.go:224, :253) |
| `RFC4301-4.1-5` | Implementation MUST permit multiple SAs between same endpoints with same selectors (QoS differentiation) (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** multiple SAs with identical selectors coexist by SPI in the kernel XFRM SAD and ze's SPI-keyed installs do not forbid this; the DSCP classification that selects among them for QoS is a datapath function ze does not model (internal/component/ike/dataplane/dataplane.go:81-108) |
| `RFC4301-4.2-1` | MUST NOT instantiate an ESP SA with both NULL encryption and no integrity algorithm (§4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L124). **negative:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L102) |
| `RFC4301-4.4.1-1` | Implementation MUST have at least one SPD (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L150). **negative:** no negative test. **{single-polarity}:** ze maintains an SPD policy model (SPParams) and installs at least the PROTECT entries into the kernel SPD via both the IKE and OSPFv3 paths (internal/component/ike/dataplane/dataplane.go:145, engine/child.go:276, :290, internal/plugins/ospf/ipsec_install.go:432-452) |
| `RFC4301-4.4.1-2` | SPD MUST be consulted for ALL traffic crossing IPsec boundary, including IKE management traffic (§4.4.1, §5) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** per-packet SPD consultation for every crossing packet is a kernel XFRM datapath function; ze populates the kernel SPD but does not process packets |
| `RFC4301-4.4.1.1-1` | All implementations MUST support the defined selectors: remote/local IP, next-layer protocol, ports, ICMP type/code (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's policy model carries IP-prefix and next-layer-protocol selectors only; the IKE traffic-selector negotiation discards ports/protocol into an address-only net.IPNet, and SPParams has no port or ICMP-type/code field (internal/component/ike/dataplane/dataplane.go:110-130 exists; ports/ICMP missing at internal/component/ike/engine/sa.go:128-129, engine/initiator.go:325) |
| `RFC4301-4.4.2-1` | Inbound SAD entries MUST be populated with negotiated selector values for packet verification (§4.4.2) | MUST | 4.4.2 | **positive:** `unit/verify` [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L412). **positive:** `unit/verify` [`TestNarrowedSelectorsReachTheInstalledPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L423). **negative:** no negative test. **{single-polarity}:** ze captures the RFC 7296-narrowed negotiated traffic selectors and projects them into the inbound require-policy; the per-packet check against them is kernel-enforced (internal/component/ike/engine/child.go:155-164, :276-288) |
| `RFC4301-4.5-1` | Implementations MUST support both manual and automated (IKEv2) key management (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L144). **positive:** `unit/verify` [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L110). **negative:** no negative test. **{single-polarity}:** ze implements automated keying via its native IKEv2 engine and manual keying via the OSPFv3 RFC 4552 config, both installing SAs through the same dataplane seam (internal/component/ike/engine/child.go:199-307, internal/plugins/ospf/ipsec_install.go:295-342) |
| `RFC4301-5.2-1` | Inbound packets not matching SPD-I MUST be discarded (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** dropping inbound packets that fail the SPD-I match is a kernel XFRM datapath function; ze installs the inbound require-policies but does not process packets (internal/component/ike/engine/child.go:276) |
| `RFC4301-5.2-2` | After decapsulation, inner packet selectors MUST be verified against SAD traffic selectors (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** post-decapsulation inner-selector verification against the SAD is a kernel XFRM datapath function; the kernel verifies the inner packet against the inbound policy ze installs |
| `RFC4301-4.1-6` | Multicast-capable implementations MUST support multicast SAD lookup (three-step: SPI+dst+src, SPI+dst, SPI alone) (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the three-step inbound SAD lookup (SPI+dst+src / SPI+dst / SPI) is a per-packet kernel XFRM function; ze performs no inbound SAD lookups |
| `RFC4301-7-1` | AH/ESP MUST NOT be applied in transport mode to IPv4 fragments; use tunnel mode (§7). The obligation is stated indicatively and the cited section only restates it: Section 4.1 says "AH and ESP cannot be applied using transport mode to IPv4 packets that are fragments. Only tunnel mode can be employed in such cases", and Section 7 recalls it as "In Section 4.1, transport mode SAs have been defined to not carry fragments (IPv4 or IPv6)" | MUST NOT | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies transport mode only to IPv6 OSPFv3 traffic (IPv4-family IPsec is rejected at config), so transport-mode IPsec on an IPv4 fragment structurally cannot arise, and per-packet fragment handling is kernel-delegated (internal/plugins/ospf/config_ipsec.go:21-22) |
| `RFC4301-4.4.1-3` | SPD SHOULD have a default final entry that discards unmatched traffic (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7-2` | Encapsulator SHOULD perform Path MTU Discovery and adjust tunnel MTU (§7). The sentence behind this row is in Section 8.2.2 of RFC 4301, not in Section 7 of it: "Implementations SHOULD use the approach described in the Path MTU Discovery document ..., which suggests periodically resetting the PMTU to the first-hop data-link MTU and then letting the normal PMTU Discovery processes update the PMTU as necessary. The period SHOULD be configurable." (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.1-7` | Security gateways MAY support transport mode only when acting as a host (e.g., SNMP management traffic) (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.3.1-1` | Sub-tree matching MUST be supported for distinguished names, domain names and RFC 822 e-mail addresses in PAD entries (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** `unit/verify` [`TestPadSubtreeAdmitsAPeerBeneathIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L35). **positive:** `unit/verify` [`TestPadSubtreeStillBindsTheCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L185). **positive:** `unit/verify` [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/validate_identity_test.go#L166). **negative:** `unit/verify` [`TestPadKeyIDStaysExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L111). **negative:** `unit/verify` [`TestPadSubtreeRefusesAPeerOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L69) |
| `RFC4301-4.4.3.1-2` | For IPv4 and IPv6 addresses in PAD entries, the same address range syntax used for SPD entries MUST be supported (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** `unit/verify` [`TestPadAddressRangeAdmitsAnAddressInsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L129). **positive:** `unit/verify` [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/validate_identity_test.go#L167). **negative:** `unit/verify` [`TestPadAddressRangeRefusesWhatIsOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L155) |
| `RFC4301-4.4.1-4` | A user or administrator MUST be able to order the SPD entries, and the management interface MUST support (total) ordering of them as seen via that interface (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestSPDPolicyMirrorsTheInboundSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L94). **positive:** `unit/verify` [`TestSpdOperatorOrdersOverlappingPeers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L20). **negative:** `unit/verify` [`TestSPDPolicyOrderInsideBypassBandRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L123). **negative:** `unit/verify` [`TestSpdOrderCannotCaptureTheIKEControlPlane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L107). **negative:** `unit/verify` [`TestSpdUnstatedOrderTakesTheDefaultRank`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L67) |
| `RFC4301-7.4-1` | All implementations MUST support DISCARDing of fragments using the normal SPD packet classification mechanisms (§7.4) | MUST | 7.4 | **positive:** `unit/verify` [`TestDiscardPolicyReachesTheBackend`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L45). **positive:** `unit/verify` [`TestDiscardPolicyReachesTheKernelAsBlock`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L41). **positive:** `unit/verify` [`TestPolicyReadbackReportsDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L98). **positive:** `unit/verify` [`TestSPDPolicyCarriesTheDiscardDisposition`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L52). **positive:** `unit/verify` [`TestVPPSPDActionCarriesEveryDisposition`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_vpp_test.go#L23). **negative:** `unit/verify` [`TestBypassPolicyIsNotBlocked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L64). **negative:** `unit/verify` [`TestBypassPolicyReachesTheBackendAsBypass`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L75). **negative:** `unit/verify` [`TestSPDPolicyBypassIsNotADiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L83) |
| `RFC4301-7.4-2` | All implementations MUST support stateful fragment checking to accommodate BYPASS traffic for which a non-trivial port range is specified (§7.4; Appendix D.4 restates it as "implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified") | MUST | 7.4 | **positive:** `unit/verify` [`TestPortScopedBypassNeverReachesTheForwardPath`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_test.go#L74). **positive:** `unit/verify` [`TestXFRMPortScopedBypassIsNeverStoredOnTheForwardPath`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_fragment_integration_linux_test.go#L69). **negative:** `unit/verify` [`TestIKEBypassIsPortScopedSoSection74Binds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_test.go#L133) |
| `RFC4301-3.1-1` | A compliant host implementation MUST support (a) and (c) and a compliant security gateway must support all three of these forms of connectivity, since under certain circumstances a security gateway acts as a host (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-3.2-1` | IPsec implementations MUST support ESP and MAY support AH (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4-1` | All implementations of AH or ESP MUST support the concept of an SA as described below (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.1-8` | The indication of whether source and destination address matching is required to map inbound IPsec traffic to SAs MUST be set either as a side effect of manual SA configuration or via negotiation using an SA management protocol, e.g., IKE or GDOI (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.1-9` | The receiver MUST process the packets from the different SAs (multiple SAs with the same selectors) without prejudice (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.1-10` | The DSCP value MUST NOT be checked as part of SA/packet validation, because it is not employed for SA selection and can change en route in transport mode (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.1-11` | In IPv6 the security protocol header MUST appear before next layer protocols (e.g., TCP, UDP, SCTP) (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4-1` | The external behavior of implementations MUST correspond to the externally observable characteristics of the processing model of Section 4.4 in order to be compliant (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1-5` | If an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1-6` | The SPD MUST permit a user or administrator to specify policy entries as SPD-I, SPD-O and SPD-S, covering the DISCARD, BYPASS and PROTECT dispositions (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1-7` | For every IPsec implementation, there MUST be a management interface that allows a user or system administrator to manage the SPD (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1-8` | The system administrator MUST be able to specify whether or not a user or application can override (default) system policies (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1-9` | When an SPD entry is decorrelated all the resulting entries MUST be linked together, so that all members of the group derived from an individual SPD entry can all be placed into caches and into the SAD at the same time (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1-10` | If the SPD is not decorrelated, caching is not allowed and an ordered search of the SPD MUST be performed to verify that inbound traffic arriving on an SA is consistent with the access control policy expressed in the SPD (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1.1-2` | A value of OPAQUE also MUST be supported for the Local and Remote port selectors, because ports may be unavailable on a fragment or encrypted by IPsec (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1.1-3` | If the SA requires a port value other than ANY or OPAQUE, an arriving fragment without ports MUST be discarded (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1.1-4` | Given a policy entry with a range of ICMP Types and a range of Codes, an implementation MUST test for a match using the Type and Code of the arriving ICMP packet as the stated composite comparison (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1.1-5` | All IPsec implementations MUST support the use of names (DNS name, RFC 822 address, X.500 distinguished name, other ID type) as a Local or Remote identity selector (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.1.2-1` | Until IKE provides a facility that conveys the semantics expressed in the SPD via selector sets, users MUST NOT include multiple selector sets in a single SPD entry unless the access control intent aligns with the IKE "mix and match" semantics (§4.4.1.2) | MUST NOT | 4.4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.2.1-1` | The data items listed in Section 4.4.2.1 (SPI, sequence number counter, anti-replay window, AH and ESP algorithm state, lifetime, mode, tunnel header, PMTU) MUST be in the SAD (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.2.1-2` | A compliant implementation MUST support both types of SA lifetime (time and byte count) and MUST support a simultaneous use of both (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.2.1-3` | Implementations MUST be able to handle having the counters at the ends of an SA get out of synch, e.g., because of packet loss (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.3.1-3` | For the "other ID type" PAD name type, only exact-match syntax MUST be supported, since there is no explicit structure for this ID type (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.3.2-1` | Implementations MUST provide a means for an administrator to require a match between an asserted IKE ID and the subject name or subject alt name in a certificate (§4.4.3.2) | MUST | 4.4.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.3.3-1` | A peer may be authorized for both address types, so there MUST be provision for both a v4 and a v6 address range in the PAD child SA authorization data (§4.4.3.3) | MUST | 4.4.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.5.2-1` | The encryption keys MUST be taken from the first (left-most, high-order) bits of the keying material string and the integrity keys MUST be taken from the remaining bits (§4.5.2) | MUST | 4.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.5.3-1` | An IPsec-supporting host or security gateway MUST have an administrative interface that allows the user/administrator to configure the address of one or more security gateways for ranges of destination addresses that require its use (§4.5.3) | MUST | 4.5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-5-1` | If no policy is found in the SPD that matches a packet (for either inbound or outbound traffic), the packet MUST be discarded (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-5.1-1` | IPsec MUST perform the steps of Section 5.1 when processing outbound packets (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-5.1-2` | For an outbound packet matched to a BYPASS entry whose return traffic must also bypass, there MUST be an entry in the SPD-I database that permits inbound bypassing of the packet, otherwise the packet will be discarded (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-5.1-3` | With regard to determining and enforcing the PMTU of an SA, the IPsec system MUST follow the steps described in Section 8.2 (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-5.1.2.1-1` | If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain (§5.1.2.1) | MUST | 5.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-5.2-3` | IPsec MUST perform the steps of Section 5.2 when processing inbound packets (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-5.2-4` | IKE traffic MUST have an explicit BYPASS entry in the SPD (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-5.2-5` | As with all outbound traffic that is to be bypassed, an inbound-bypassed packet whose return traffic is also bypassed MUST be matched against an SPD-O entry (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-6-1` | Disposition of non-error ICMP messages that are not addressed to the IPsec implementation itself MUST be explicitly accounted for using SPD entries (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-6.1.1-1` | A compliant IPsec implementation MUST permit a local administrator to configure it to accept or reject unauthenticated ICMP traffic (§6.1.1) | MUST | 6.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-6.1.1-2` | That accept/reject control MUST be at the granularity of ICMP type and MAY be at the granularity of ICMP type and code (§6.1.1) | MUST | 6.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-6.1.2-1` | Implementers MUST provide controls to allow local administrators to constrain the processing of ICMP error messages received on the protected side of the boundary and directed to the IPsec implementation (§6.1.2) | MUST | 6.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-6.2-1` | An IPsec implementation MUST be configurable to check that the ICMP error message payload header information is consistent with the SA via which it arrives (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-6.2-2` | IPsec senders and receivers MUST support the Section 6.2 processing for ICMP error messages that are sent and received via SAs (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-6.2-3` | If no SA and no SPD entry would carry an outbound ICMP error message, an IPsec implementation MUST map the message to the SA that would carry the return traffic associated with the packet that triggered it (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-6.2-4` | If an inbound ICMP error message arrives on an SA and its IP and ICMP headers do not match the traffic selectors for that SA, the receiver MUST process the received message in the special fashion Section 6.2 describes (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-6.2-5` | If the ICMP error message payload check fails, the IPsec implementation MUST NOT forward the ICMP message to the destination (§6.2) | MUST NOT | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7.1-1` | All implementations MUST support tunnel mode SAs that are configured to pass traffic without regard to port field (or ICMP type/code or Mobility Header type) values (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7.1-2` | If such an SA will carry traffic for specified protocols, the selector set for the SA MUST specify the port fields (or ICMP type/code or Mobility Header type) as ANY (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7.1-3` | If such an SA will carry traffic without regard to a specific protocol value, the port field values are undefined and MUST be set to ANY as well (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7.2-1` | Receivers MUST perform a minimum offset check on IPv4 non-initial fragments to protect against overlapping fragment attacks when separate tunnel mode SAs for non-initial fragments are employed (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7.2-2` | These separate fragment-carrying SAs MUST have non-trivial protocol selector values, otherwise approach #1 (Section 7.1) MUST be used (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7.3-1` | Implementations that will transmit non-initial fragments on a tunnel mode SA that makes use of non-trivial port (or ICMP type/code or MH type) selectors MUST notify a peer via the IKE NOTIFY NON_FIRST_FRAGMENTS_ALSO payload (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7.3-2` | The peer MUST reject the NON_FIRST_FRAGMENTS_ALSO proposal if it will not accept non-initial fragments in this context (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7.3-3` | If an implementation does not successfully negotiate transmission of non-initial fragments for such an SA, it MUST NOT send such fragments over the SA (§7.3) | MUST NOT | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7.3-4` | A receiver MUST discard non-initial fragments that arrive on an SA with non-trivial port (or ICMP type/code or MH type) selector values unless this feature has been negotiated (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7.3-5` | The receiver MUST discard non-initial fragments that do not comply with the security policy applied to the overall packet (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-8.1-1` | All IPsec implementations MUST support the option of copying the DF bit from an outbound packet to the tunnel mode header that it emits, when traffic is carried via a tunnel mode SA (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-8.1-2` | It MUST be possible to configure the implementation's treatment of the DF bit (set, clear, copy from inner header) for each SA (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-8.2.1-1` | When traffic arrives that would exceed the updated PMTU value, the traffic MUST be handled as Section 8.2.1 describes (fragment before or after IPsec, or discard and send a PMTU ICMP message, per the DF bit) (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-8.2.2-1` | In all IPsec implementations, the PMTU associated with an SA MUST be "aged" and some mechanism is required to update the PMTU in a timely manner (§8.2.2) | MUST | 8.2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4301-4.1-5`](#rfc4301-4.1-5) Implementation MUST permit multiple SAs between same endpoints with same selectors (QoS differentiation) (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: multiple SAs with identical selectors coexist by SPI in the kernel XFRM SAD and ze's SPI-keyed installs do not forbid this; the DSCP classification that selects among them for QoS is a datapath function ze does not model (internal/component/ike/dataplane/dataplane.go:81-108) |
| [`RFC4301-4.4.1-2`](#rfc4301-4.4.1-2) SPD MUST be consulted for ALL traffic crossing IPsec boundary, including IKE management traffic (§4.4.1, §5) | no test | no test carries this requirement id; annotated {not-applicable}: per-packet SPD consultation for every crossing packet is a kernel XFRM datapath function; ze populates the kernel SPD but does not process packets |
| [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1) All implementations MUST support the defined selectors: remote/local IP, next-layer protocol, ports, ICMP type/code (§4.4.1.1) | {gap}, no test | ze's policy model carries IP-prefix and next-layer-protocol selectors only; the IKE traffic-selector negotiation discards ports/protocol into an address-only net.IPNet, and SPParams has no port or ICMP-type/code field (internal/component/ike/dataplane/dataplane.go:110-130 exists; ports/ICMP missing at internal/component/ike/engine/sa.go:128-129, engine/initiator.go:325) |
| [`RFC4301-5.2-1`](#rfc4301-5.2-1) Inbound packets not matching SPD-I MUST be discarded (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: dropping inbound packets that fail the SPD-I match is a kernel XFRM datapath function; ze installs the inbound require-policies but does not process packets (internal/component/ike/engine/child.go:276) |
| [`RFC4301-5.2-2`](#rfc4301-5.2-2) After decapsulation, inner packet selectors MUST be verified against SAD traffic selectors (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: post-decapsulation inner-selector verification against the SAD is a kernel XFRM datapath function; the kernel verifies the inner packet against the inbound policy ze installs |
| [`RFC4301-4.1-6`](#rfc4301-4.1-6) Multicast-capable implementations MUST support multicast SAD lookup (three-step: SPI+dst+src, SPI+dst, SPI alone) (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: the three-step inbound SAD lookup (SPI+dst+src / SPI+dst / SPI) is a per-packet kernel XFRM function; ze performs no inbound SAD lookups |
| [`RFC4301-7-1`](#rfc4301-7-1) AH/ESP MUST NOT be applied in transport mode to IPv4 fragments; use tunnel mode (§7). The obligation is stated indicatively and the cited section only restates it: Section 4.1 says "AH and ESP cannot be applied using transport mode to IPv4 packets that are fragments. Only tunnel mode can be employed in such cases", and Section 7 recalls it as "In Section 4.1, transport mode SAs have been defined to not carry fragments (IPv4 or IPv6)" | no test | no test carries this requirement id; annotated {not-applicable}: ze applies transport mode only to IPv6 OSPFv3 traffic (IPv4-family IPsec is rejected at config), so transport-mode IPsec on an IPv4 fragment structurally cannot arise, and per-packet fragment handling is kernel-delegated (internal/plugins/ospf/config_ipsec.go:21-22) |
| [`RFC4301-3.1-1`](#rfc4301-3.1-1) A compliant host implementation MUST support (a) and (c) and a compliant security gateway must support all three of these forms of connectivity, since under certain circumstances a security gateway acts as a host (§3.1) | no test | no test carries this requirement id |
| [`RFC4301-3.2-1`](#rfc4301-3.2-1) IPsec implementations MUST support ESP and MAY support AH (§3.2) | no test | no test carries this requirement id |
| [`RFC4301-4-1`](#rfc4301-4-1) All implementations of AH or ESP MUST support the concept of an SA as described below (§4) | no test | no test carries this requirement id |
| [`RFC4301-4.1-8`](#rfc4301-4.1-8) The indication of whether source and destination address matching is required to map inbound IPsec traffic to SAs MUST be set either as a side effect of manual SA configuration or via negotiation using an SA management protocol, e.g., IKE or GDOI (§4.1) | no test | no test carries this requirement id |
| [`RFC4301-4.1-9`](#rfc4301-4.1-9) The receiver MUST process the packets from the different SAs (multiple SAs with the same selectors) without prejudice (§4.1) | no test | no test carries this requirement id |
| [`RFC4301-4.1-10`](#rfc4301-4.1-10) The DSCP value MUST NOT be checked as part of SA/packet validation, because it is not employed for SA selection and can change en route in transport mode (§4.1) | no test | no test carries this requirement id |
| [`RFC4301-4.1-11`](#rfc4301-4.1-11) In IPv6 the security protocol header MUST appear before next layer protocols (e.g., TCP, UDP, SCTP) (§4.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4-1`](#rfc4301-4.4-1) The external behavior of implementations MUST correspond to the externally observable characteristics of the processing model of Section 4.4 in order to be compliant (§4.4) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1-5`](#rfc4301-4.4.1-5) If an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing (§4.4.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1-6`](#rfc4301-4.4.1-6) The SPD MUST permit a user or administrator to specify policy entries as SPD-I, SPD-O and SPD-S, covering the DISCARD, BYPASS and PROTECT dispositions (§4.4.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1-7`](#rfc4301-4.4.1-7) For every IPsec implementation, there MUST be a management interface that allows a user or system administrator to manage the SPD (§4.4.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1-8`](#rfc4301-4.4.1-8) The system administrator MUST be able to specify whether or not a user or application can override (default) system policies (§4.4.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1-9`](#rfc4301-4.4.1-9) When an SPD entry is decorrelated all the resulting entries MUST be linked together, so that all members of the group derived from an individual SPD entry can all be placed into caches and into the SAD at the same time (§4.4.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1-10`](#rfc4301-4.4.1-10) If the SPD is not decorrelated, caching is not allowed and an ordered search of the SPD MUST be performed to verify that inbound traffic arriving on an SA is consistent with the access control policy expressed in the SPD (§4.4.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1.1-2`](#rfc4301-4.4.1.1-2) A value of OPAQUE also MUST be supported for the Local and Remote port selectors, because ports may be unavailable on a fragment or encrypted by IPsec (§4.4.1.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1.1-3`](#rfc4301-4.4.1.1-3) If the SA requires a port value other than ANY or OPAQUE, an arriving fragment without ports MUST be discarded (§4.4.1.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1.1-4`](#rfc4301-4.4.1.1-4) Given a policy entry with a range of ICMP Types and a range of Codes, an implementation MUST test for a match using the Type and Code of the arriving ICMP packet as the stated composite comparison (§4.4.1.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1.1-5`](#rfc4301-4.4.1.1-5) All IPsec implementations MUST support the use of names (DNS name, RFC 822 address, X.500 distinguished name, other ID type) as a Local or Remote identity selector (§4.4.1.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.1.2-1`](#rfc4301-4.4.1.2-1) Until IKE provides a facility that conveys the semantics expressed in the SPD via selector sets, users MUST NOT include multiple selector sets in a single SPD entry unless the access control intent aligns with the IKE "mix and match" semantics (§4.4.1.2) | no test | no test carries this requirement id |
| [`RFC4301-4.4.2.1-1`](#rfc4301-4.4.2.1-1) The data items listed in Section 4.4.2.1 (SPI, sequence number counter, anti-replay window, AH and ESP algorithm state, lifetime, mode, tunnel header, PMTU) MUST be in the SAD (§4.4.2.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2) A compliant implementation MUST support both types of SA lifetime (time and byte count) and MUST support a simultaneous use of both (§4.4.2.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.2.1-3`](#rfc4301-4.4.2.1-3) Implementations MUST be able to handle having the counters at the ends of an SA get out of synch, e.g., because of packet loss (§4.4.2.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.3.1-3`](#rfc4301-4.4.3.1-3) For the "other ID type" PAD name type, only exact-match syntax MUST be supported, since there is no explicit structure for this ID type (§4.4.3.1) | no test | no test carries this requirement id |
| [`RFC4301-4.4.3.2-1`](#rfc4301-4.4.3.2-1) Implementations MUST provide a means for an administrator to require a match between an asserted IKE ID and the subject name or subject alt name in a certificate (§4.4.3.2) | no test | no test carries this requirement id |
| [`RFC4301-4.4.3.3-1`](#rfc4301-4.4.3.3-1) A peer may be authorized for both address types, so there MUST be provision for both a v4 and a v6 address range in the PAD child SA authorization data (§4.4.3.3) | no test | no test carries this requirement id |
| [`RFC4301-4.5.2-1`](#rfc4301-4.5.2-1) The encryption keys MUST be taken from the first (left-most, high-order) bits of the keying material string and the integrity keys MUST be taken from the remaining bits (§4.5.2) | no test | no test carries this requirement id |
| [`RFC4301-4.5.3-1`](#rfc4301-4.5.3-1) An IPsec-supporting host or security gateway MUST have an administrative interface that allows the user/administrator to configure the address of one or more security gateways for ranges of destination addresses that require its use (§4.5.3) | no test | no test carries this requirement id |
| [`RFC4301-5-1`](#rfc4301-5-1) If no policy is found in the SPD that matches a packet (for either inbound or outbound traffic), the packet MUST be discarded (§5) | no test | no test carries this requirement id |
| [`RFC4301-5.1-1`](#rfc4301-5.1-1) IPsec MUST perform the steps of Section 5.1 when processing outbound packets (§5.1) | no test | no test carries this requirement id |
| [`RFC4301-5.1-2`](#rfc4301-5.1-2) For an outbound packet matched to a BYPASS entry whose return traffic must also bypass, there MUST be an entry in the SPD-I database that permits inbound bypassing of the packet, otherwise the packet will be discarded (§5.1) | no test | no test carries this requirement id |
| [`RFC4301-5.1-3`](#rfc4301-5.1-3) With regard to determining and enforcing the PMTU of an SA, the IPsec system MUST follow the steps described in Section 8.2 (§5.1) | no test | no test carries this requirement id |
| [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1) If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain (§5.1.2.1) | no test | no test carries this requirement id |
| [`RFC4301-5.2-3`](#rfc4301-5.2-3) IPsec MUST perform the steps of Section 5.2 when processing inbound packets (§5.2) | no test | no test carries this requirement id |
| [`RFC4301-5.2-4`](#rfc4301-5.2-4) IKE traffic MUST have an explicit BYPASS entry in the SPD (§5.2) | no test | no test carries this requirement id |
| [`RFC4301-5.2-5`](#rfc4301-5.2-5) As with all outbound traffic that is to be bypassed, an inbound-bypassed packet whose return traffic is also bypassed MUST be matched against an SPD-O entry (§5.2) | no test | no test carries this requirement id |
| [`RFC4301-6-1`](#rfc4301-6-1) Disposition of non-error ICMP messages that are not addressed to the IPsec implementation itself MUST be explicitly accounted for using SPD entries (§6) | no test | no test carries this requirement id |
| [`RFC4301-6.1.1-1`](#rfc4301-6.1.1-1) A compliant IPsec implementation MUST permit a local administrator to configure it to accept or reject unauthenticated ICMP traffic (§6.1.1) | no test | no test carries this requirement id |
| [`RFC4301-6.1.1-2`](#rfc4301-6.1.1-2) That accept/reject control MUST be at the granularity of ICMP type and MAY be at the granularity of ICMP type and code (§6.1.1) | no test | no test carries this requirement id |
| [`RFC4301-6.1.2-1`](#rfc4301-6.1.2-1) Implementers MUST provide controls to allow local administrators to constrain the processing of ICMP error messages received on the protected side of the boundary and directed to the IPsec implementation (§6.1.2) | no test | no test carries this requirement id |
| [`RFC4301-6.2-1`](#rfc4301-6.2-1) An IPsec implementation MUST be configurable to check that the ICMP error message payload header information is consistent with the SA via which it arrives (§6.2) | no test | no test carries this requirement id |
| [`RFC4301-6.2-2`](#rfc4301-6.2-2) IPsec senders and receivers MUST support the Section 6.2 processing for ICMP error messages that are sent and received via SAs (§6.2) | no test | no test carries this requirement id |
| [`RFC4301-6.2-3`](#rfc4301-6.2-3) If no SA and no SPD entry would carry an outbound ICMP error message, an IPsec implementation MUST map the message to the SA that would carry the return traffic associated with the packet that triggered it (§6.2) | no test | no test carries this requirement id |
| [`RFC4301-6.2-4`](#rfc4301-6.2-4) If an inbound ICMP error message arrives on an SA and its IP and ICMP headers do not match the traffic selectors for that SA, the receiver MUST process the received message in the special fashion Section 6.2 describes (§6.2) | no test | no test carries this requirement id |
| [`RFC4301-6.2-5`](#rfc4301-6.2-5) If the ICMP error message payload check fails, the IPsec implementation MUST NOT forward the ICMP message to the destination (§6.2) | no test | no test carries this requirement id |
| [`RFC4301-7.1-1`](#rfc4301-7.1-1) All implementations MUST support tunnel mode SAs that are configured to pass traffic without regard to port field (or ICMP type/code or Mobility Header type) values (§7.1) | no test | no test carries this requirement id |
| [`RFC4301-7.1-2`](#rfc4301-7.1-2) If such an SA will carry traffic for specified protocols, the selector set for the SA MUST specify the port fields (or ICMP type/code or Mobility Header type) as ANY (§7.1) | no test | no test carries this requirement id |
| [`RFC4301-7.1-3`](#rfc4301-7.1-3) If such an SA will carry traffic without regard to a specific protocol value, the port field values are undefined and MUST be set to ANY as well (§7.1) | no test | no test carries this requirement id |
| [`RFC4301-7.2-1`](#rfc4301-7.2-1) Receivers MUST perform a minimum offset check on IPv4 non-initial fragments to protect against overlapping fragment attacks when separate tunnel mode SAs for non-initial fragments are employed (§7.2) | no test | no test carries this requirement id |
| [`RFC4301-7.2-2`](#rfc4301-7.2-2) These separate fragment-carrying SAs MUST have non-trivial protocol selector values, otherwise approach #1 (Section 7.1) MUST be used (§7.2) | no test | no test carries this requirement id |
| [`RFC4301-7.3-1`](#rfc4301-7.3-1) Implementations that will transmit non-initial fragments on a tunnel mode SA that makes use of non-trivial port (or ICMP type/code or MH type) selectors MUST notify a peer via the IKE NOTIFY NON_FIRST_FRAGMENTS_ALSO payload (§7.3) | no test | no test carries this requirement id |
| [`RFC4301-7.3-2`](#rfc4301-7.3-2) The peer MUST reject the NON_FIRST_FRAGMENTS_ALSO proposal if it will not accept non-initial fragments in this context (§7.3) | no test | no test carries this requirement id |
| [`RFC4301-7.3-3`](#rfc4301-7.3-3) If an implementation does not successfully negotiate transmission of non-initial fragments for such an SA, it MUST NOT send such fragments over the SA (§7.3) | no test | no test carries this requirement id |
| [`RFC4301-7.3-4`](#rfc4301-7.3-4) A receiver MUST discard non-initial fragments that arrive on an SA with non-trivial port (or ICMP type/code or MH type) selector values unless this feature has been negotiated (§7.3) | no test | no test carries this requirement id |
| [`RFC4301-7.3-5`](#rfc4301-7.3-5) The receiver MUST discard non-initial fragments that do not comply with the security policy applied to the overall packet (§7.3) | no test | no test carries this requirement id |
| [`RFC4301-8.1-1`](#rfc4301-8.1-1) All IPsec implementations MUST support the option of copying the DF bit from an outbound packet to the tunnel mode header that it emits, when traffic is carried via a tunnel mode SA (§8.1) | no test | no test carries this requirement id |
| [`RFC4301-8.1-2`](#rfc4301-8.1-2) It MUST be possible to configure the implementation's treatment of the DF bit (set, clear, copy from inner header) for each SA (§8.1) | no test | no test carries this requirement id |
| [`RFC4301-8.2.1-1`](#rfc4301-8.2.1-1) When traffic arrives that would exceed the updated PMTU value, the traffic MUST be handled as Section 8.2.1 describes (fragment before or after IPsec, or discard and send a PMTU ICMP message, per the DF bit) (§8.2.1) | no test | no test carries this requirement id |
| [`RFC4301-8.2.2-1`](#rfc4301-8.2.2-1) In all IPsec implementations, the PMTU associated with an SA MUST be "aged" and some mechanism is required to update the PMTU in a timely manner (§8.2.2) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4301-4.1-1`](#rfc4301-4.1-1)

Host implementations MUST support both transport and tunnel mode (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L169) | unit/verify | unproven |
| positive | [`TestIPsecSAPerDestinationWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L226) | unit/verify | revert, verified |

### [`RFC4301-4.1-2`](#rfc4301-4.1-2)

Security gateways MUST support tunnel mode (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L184) | unit/verify | unproven |

### [`RFC4301-4.1-3`](#rfc4301-4.1-3)

SAs between a security gateway and any peer MUST use tunnel mode (two narrow exceptions for gateway-as-host) (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L171) | unit/verify | unproven |

### [`RFC4301-4.1-4`](#rfc4301-4.1-4)

IKE-created SA pairs MUST use the same mode (both tunnel or both transport) (§4.1). The source sentence is indicative: "IKE creates pairs of SAs, so for simplicity, we choose to require that both SAs in a pair be of the same mode, transport or tunnel."

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L173) | unit/verify | unproven |

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
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L150) | unit/verify | unproven |

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
| positive | [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L412) | unit/verify | unproven |
| positive | [`TestNarrowedSelectorsReachTheInstalledPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L423) | unit/verify | unproven |

### [`RFC4301-4.5-1`](#rfc4301-4.5-1)

Implementations MUST support both manual and automated (IKEv2) key management (§4.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L144) | unit/verify | unproven |
| positive | [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L110) | unit/verify | mutant, verified |

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

AH/ESP MUST NOT be applied in transport mode to IPv4 fragments; use tunnel mode (§7). The obligation is stated indicatively and the cited section only restates it: Section 4.1 says "AH and ESP cannot be applied using transport mode to IPv4 packets that are fragments. Only tunnel mode can be employed in such cases", and Section 7 recalls it as "In Section 4.1, transport mode SAs have been defined to not carry fragments (IPv4 or IPv6)"

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
| positive | [`TestPolicyReadbackReportsDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L98) | unit/verify | revert, verified |
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

### [`RFC4301-3.1-1`](#rfc4301-3.1-1)

A compliant host implementation MUST support (a) and (c) and a compliant security gateway must support all three of these forms of connectivity, since under certain circumstances a security gateway acts as a host (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-3.1-1, so no unit is bound to it.

### [`RFC4301-3.2-1`](#rfc4301-3.2-1)

IPsec implementations MUST support ESP and MAY support AH (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-3.2-1, so no unit is bound to it.

### [`RFC4301-4-1`](#rfc4301-4-1)

All implementations of AH or ESP MUST support the concept of an SA as described below (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4-1, so no unit is bound to it.

### [`RFC4301-4.1-8`](#rfc4301-4.1-8)

The indication of whether source and destination address matching is required to map inbound IPsec traffic to SAs MUST be set either as a side effect of manual SA configuration or via negotiation using an SA management protocol, e.g., IKE or GDOI (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-8, so no unit is bound to it.

### [`RFC4301-4.1-9`](#rfc4301-4.1-9)

The receiver MUST process the packets from the different SAs (multiple SAs with the same selectors) without prejudice (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-9, so no unit is bound to it.

### [`RFC4301-4.1-10`](#rfc4301-4.1-10)

The DSCP value MUST NOT be checked as part of SA/packet validation, because it is not employed for SA selection and can change en route in transport mode (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-10, so no unit is bound to it.

### [`RFC4301-4.1-11`](#rfc4301-4.1-11)

In IPv6 the security protocol header MUST appear before next layer protocols (e.g., TCP, UDP, SCTP) (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-11, so no unit is bound to it.

### [`RFC4301-4.4-1`](#rfc4301-4.4-1)

The external behavior of implementations MUST correspond to the externally observable characteristics of the processing model of Section 4.4 in order to be compliant (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4-1, so no unit is bound to it.

### [`RFC4301-4.4.1-5`](#rfc4301-4.4.1-5)

If an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-5, so no unit is bound to it.

### [`RFC4301-4.4.1-6`](#rfc4301-4.4.1-6)

The SPD MUST permit a user or administrator to specify policy entries as SPD-I, SPD-O and SPD-S, covering the DISCARD, BYPASS and PROTECT dispositions (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-6, so no unit is bound to it.

### [`RFC4301-4.4.1-7`](#rfc4301-4.4.1-7)

For every IPsec implementation, there MUST be a management interface that allows a user or system administrator to manage the SPD (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-7, so no unit is bound to it.

### [`RFC4301-4.4.1-8`](#rfc4301-4.4.1-8)

The system administrator MUST be able to specify whether or not a user or application can override (default) system policies (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-8, so no unit is bound to it.

### [`RFC4301-4.4.1-9`](#rfc4301-4.4.1-9)

When an SPD entry is decorrelated all the resulting entries MUST be linked together, so that all members of the group derived from an individual SPD entry can all be placed into caches and into the SAD at the same time (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-9, so no unit is bound to it.

### [`RFC4301-4.4.1-10`](#rfc4301-4.4.1-10)

If the SPD is not decorrelated, caching is not allowed and an ordered search of the SPD MUST be performed to verify that inbound traffic arriving on an SA is consistent with the access control policy expressed in the SPD (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-10, so no unit is bound to it.

### [`RFC4301-4.4.1.1-2`](#rfc4301-4.4.1.1-2)

A value of OPAQUE also MUST be supported for the Local and Remote port selectors, because ports may be unavailable on a fragment or encrypted by IPsec (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-2, so no unit is bound to it.

### [`RFC4301-4.4.1.1-3`](#rfc4301-4.4.1.1-3)

If the SA requires a port value other than ANY or OPAQUE, an arriving fragment without ports MUST be discarded (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-3, so no unit is bound to it.

### [`RFC4301-4.4.1.1-4`](#rfc4301-4.4.1.1-4)

Given a policy entry with a range of ICMP Types and a range of Codes, an implementation MUST test for a match using the Type and Code of the arriving ICMP packet as the stated composite comparison (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-4, so no unit is bound to it.

### [`RFC4301-4.4.1.1-5`](#rfc4301-4.4.1.1-5)

All IPsec implementations MUST support the use of names (DNS name, RFC 822 address, X.500 distinguished name, other ID type) as a Local or Remote identity selector (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-5, so no unit is bound to it.

### [`RFC4301-4.4.1.2-1`](#rfc4301-4.4.1.2-1)

Until IKE provides a facility that conveys the semantics expressed in the SPD via selector sets, users MUST NOT include multiple selector sets in a single SPD entry unless the access control intent aligns with the IKE "mix and match" semantics (§4.4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.2-1, so no unit is bound to it.

### [`RFC4301-4.4.2.1-1`](#rfc4301-4.4.2.1-1)

The data items listed in Section 4.4.2.1 (SPI, sequence number counter, anti-replay window, AH and ESP algorithm state, lifetime, mode, tunnel header, PMTU) MUST be in the SAD (§4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.2.1-1, so no unit is bound to it.

### [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2)

A compliant implementation MUST support both types of SA lifetime (time and byte count) and MUST support a simultaneous use of both (§4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.2.1-2, so no unit is bound to it.

### [`RFC4301-4.4.2.1-3`](#rfc4301-4.4.2.1-3)

Implementations MUST be able to handle having the counters at the ends of an SA get out of synch, e.g., because of packet loss (§4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.2.1-3, so no unit is bound to it.

### [`RFC4301-4.4.3.1-3`](#rfc4301-4.4.3.1-3)

For the "other ID type" PAD name type, only exact-match syntax MUST be supported, since there is no explicit structure for this ID type (§4.4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.3.1-3, so no unit is bound to it.

### [`RFC4301-4.4.3.2-1`](#rfc4301-4.4.3.2-1)

Implementations MUST provide a means for an administrator to require a match between an asserted IKE ID and the subject name or subject alt name in a certificate (§4.4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.3.2-1, so no unit is bound to it.

### [`RFC4301-4.4.3.3-1`](#rfc4301-4.4.3.3-1)

A peer may be authorized for both address types, so there MUST be provision for both a v4 and a v6 address range in the PAD child SA authorization data (§4.4.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.3.3-1, so no unit is bound to it.

### [`RFC4301-4.5.2-1`](#rfc4301-4.5.2-1)

The encryption keys MUST be taken from the first (left-most, high-order) bits of the keying material string and the integrity keys MUST be taken from the remaining bits (§4.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.5.2-1, so no unit is bound to it.

### [`RFC4301-4.5.3-1`](#rfc4301-4.5.3-1)

An IPsec-supporting host or security gateway MUST have an administrative interface that allows the user/administrator to configure the address of one or more security gateways for ranges of destination addresses that require its use (§4.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.5.3-1, so no unit is bound to it.

### [`RFC4301-5-1`](#rfc4301-5-1)

If no policy is found in the SPD that matches a packet (for either inbound or outbound traffic), the packet MUST be discarded (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5-1, so no unit is bound to it.

### [`RFC4301-5.1-1`](#rfc4301-5.1-1)

IPsec MUST perform the steps of Section 5.1 when processing outbound packets (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.1-1, so no unit is bound to it.

### [`RFC4301-5.1-2`](#rfc4301-5.1-2)

For an outbound packet matched to a BYPASS entry whose return traffic must also bypass, there MUST be an entry in the SPD-I database that permits inbound bypassing of the packet, otherwise the packet will be discarded (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.1-2, so no unit is bound to it.

### [`RFC4301-5.1-3`](#rfc4301-5.1-3)

With regard to determining and enforcing the PMTU of an SA, the IPsec system MUST follow the steps described in Section 8.2 (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.1-3, so no unit is bound to it.

### [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1)

If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain (§5.1.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.1.2.1-1, so no unit is bound to it.

### [`RFC4301-5.2-3`](#rfc4301-5.2-3)

IPsec MUST perform the steps of Section 5.2 when processing inbound packets (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.2-3, so no unit is bound to it.

### [`RFC4301-5.2-4`](#rfc4301-5.2-4)

IKE traffic MUST have an explicit BYPASS entry in the SPD (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.2-4, so no unit is bound to it.

### [`RFC4301-5.2-5`](#rfc4301-5.2-5)

As with all outbound traffic that is to be bypassed, an inbound-bypassed packet whose return traffic is also bypassed MUST be matched against an SPD-O entry (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.2-5, so no unit is bound to it.

### [`RFC4301-6-1`](#rfc4301-6-1)

Disposition of non-error ICMP messages that are not addressed to the IPsec implementation itself MUST be explicitly accounted for using SPD entries (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6-1, so no unit is bound to it.

### [`RFC4301-6.1.1-1`](#rfc4301-6.1.1-1)

A compliant IPsec implementation MUST permit a local administrator to configure it to accept or reject unauthenticated ICMP traffic (§6.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.1.1-1, so no unit is bound to it.

### [`RFC4301-6.1.1-2`](#rfc4301-6.1.1-2)

That accept/reject control MUST be at the granularity of ICMP type and MAY be at the granularity of ICMP type and code (§6.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.1.1-2, so no unit is bound to it.

### [`RFC4301-6.1.2-1`](#rfc4301-6.1.2-1)

Implementers MUST provide controls to allow local administrators to constrain the processing of ICMP error messages received on the protected side of the boundary and directed to the IPsec implementation (§6.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.1.2-1, so no unit is bound to it.

### [`RFC4301-6.2-1`](#rfc4301-6.2-1)

An IPsec implementation MUST be configurable to check that the ICMP error message payload header information is consistent with the SA via which it arrives (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.2-1, so no unit is bound to it.

### [`RFC4301-6.2-2`](#rfc4301-6.2-2)

IPsec senders and receivers MUST support the Section 6.2 processing for ICMP error messages that are sent and received via SAs (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.2-2, so no unit is bound to it.

### [`RFC4301-6.2-3`](#rfc4301-6.2-3)

If no SA and no SPD entry would carry an outbound ICMP error message, an IPsec implementation MUST map the message to the SA that would carry the return traffic associated with the packet that triggered it (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.2-3, so no unit is bound to it.

### [`RFC4301-6.2-4`](#rfc4301-6.2-4)

If an inbound ICMP error message arrives on an SA and its IP and ICMP headers do not match the traffic selectors for that SA, the receiver MUST process the received message in the special fashion Section 6.2 describes (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.2-4, so no unit is bound to it.

### [`RFC4301-6.2-5`](#rfc4301-6.2-5)

If the ICMP error message payload check fails, the IPsec implementation MUST NOT forward the ICMP message to the destination (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.2-5, so no unit is bound to it.

### [`RFC4301-7.1-1`](#rfc4301-7.1-1)

All implementations MUST support tunnel mode SAs that are configured to pass traffic without regard to port field (or ICMP type/code or Mobility Header type) values (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.1-1, so no unit is bound to it.

### [`RFC4301-7.1-2`](#rfc4301-7.1-2)

If such an SA will carry traffic for specified protocols, the selector set for the SA MUST specify the port fields (or ICMP type/code or Mobility Header type) as ANY (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.1-2, so no unit is bound to it.

### [`RFC4301-7.1-3`](#rfc4301-7.1-3)

If such an SA will carry traffic without regard to a specific protocol value, the port field values are undefined and MUST be set to ANY as well (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.1-3, so no unit is bound to it.

### [`RFC4301-7.2-1`](#rfc4301-7.2-1)

Receivers MUST perform a minimum offset check on IPv4 non-initial fragments to protect against overlapping fragment attacks when separate tunnel mode SAs for non-initial fragments are employed (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.2-1, so no unit is bound to it.

### [`RFC4301-7.2-2`](#rfc4301-7.2-2)

These separate fragment-carrying SAs MUST have non-trivial protocol selector values, otherwise approach #1 (Section 7.1) MUST be used (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.2-2, so no unit is bound to it.

### [`RFC4301-7.3-1`](#rfc4301-7.3-1)

Implementations that will transmit non-initial fragments on a tunnel mode SA that makes use of non-trivial port (or ICMP type/code or MH type) selectors MUST notify a peer via the IKE NOTIFY NON_FIRST_FRAGMENTS_ALSO payload (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.3-1, so no unit is bound to it.

### [`RFC4301-7.3-2`](#rfc4301-7.3-2)

The peer MUST reject the NON_FIRST_FRAGMENTS_ALSO proposal if it will not accept non-initial fragments in this context (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.3-2, so no unit is bound to it.

### [`RFC4301-7.3-3`](#rfc4301-7.3-3)

If an implementation does not successfully negotiate transmission of non-initial fragments for such an SA, it MUST NOT send such fragments over the SA (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.3-3, so no unit is bound to it.

### [`RFC4301-7.3-4`](#rfc4301-7.3-4)

A receiver MUST discard non-initial fragments that arrive on an SA with non-trivial port (or ICMP type/code or MH type) selector values unless this feature has been negotiated (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.3-4, so no unit is bound to it.

### [`RFC4301-7.3-5`](#rfc4301-7.3-5)

The receiver MUST discard non-initial fragments that do not comply with the security policy applied to the overall packet (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.3-5, so no unit is bound to it.

### [`RFC4301-8.1-1`](#rfc4301-8.1-1)

All IPsec implementations MUST support the option of copying the DF bit from an outbound packet to the tunnel mode header that it emits, when traffic is carried via a tunnel mode SA (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-8.1-1, so no unit is bound to it.

### [`RFC4301-8.1-2`](#rfc4301-8.1-2)

It MUST be possible to configure the implementation's treatment of the DF bit (set, clear, copy from inner header) for each SA (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-8.1-2, so no unit is bound to it.

### [`RFC4301-8.2.1-1`](#rfc4301-8.2.1-1)

When traffic arrives that would exceed the updated PMTU value, the traffic MUST be handled as Section 8.2.1 describes (fragment before or after IPsec, or discard and send a PMTU ICMP message, per the DF bit) (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-8.2.1-1, so no unit is bound to it.

### [`RFC4301-8.2.2-1`](#rfc4301-8.2.2-1)

In all IPsec implementations, the PMTU associated with an SA MUST be "aged" and some mechanism is required to update the PMTU in a timely manner (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-8.2.2-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc4301.txt |
| Source fingerprint | 263fd9a78175c888 |
| Record | rfc/extraction/rfc4301.json |
| Mapped sentences | 77 |
| Declined as scope | 12 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | the RFC title block, status-of-memo and copyright notice | 0 | skipped (front-matter) | the RFC title block, status-of-memo and copyright notice |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `4.1` | not stated | 11 | walked | not stated |
| `4.2` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.4` | not stated | 1 | walked | not stated |
| `4.4.1` | not stated | 11 | walked | not stated |
| `4.4.1.1` | not stated | 5 | walked | not stated |
| `4.4.1.2` | not stated | 2 | walked | not stated |
| `4.4.1.3` | not stated | 0 | walked | not stated |
| `4.4.2` | not stated | 2 | walked | not stated |
| `4.4.2.1` | not stated | 3 | walked | not stated |
| `4.4.2.2` | not stated | 0 | walked | not stated |
| `4.4.3` | not stated | 0 | walked | not stated |
| `4.4.3.1` | not stated | 3 | walked | not stated |
| `4.4.3.2` | not stated | 1 | walked | not stated |
| `4.4.3.3` | not stated | 1 | walked | not stated |
| `4.4.3.4` | not stated | 0 | walked | not stated |
| `4.5` | not stated | 1 | walked | not stated |
| `4.5.1` | not stated | 0 | walked | not stated |
| `4.5.2` | not stated | 1 | walked | not stated |
| `4.5.3` | not stated | 1 | walked | not stated |
| `4.6` | not stated | 0 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `5.1` | not stated | 3 | walked | not stated |
| `5.1.1` | not stated | 0 | walked | not stated |
| `5.1.2` | not stated | 0 | walked | not stated |
| `5.1.2.1` | not stated | 1 | walked | not stated |
| `5.1.2.2` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 4 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.1.1` | not stated | 2 | walked | not stated |
| `6.1.2` | not stated | 1 | walked | not stated |
| `6.2` | not stated | 5 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 3 | walked | not stated |
| `7.2` | not stated | 2 | walked | not stated |
| `7.3` | not stated | 5 | walked | not stated |
| `7.4` | not stated | 2 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 2 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.2.1` | not stated | 1 | walked | not stated |
| `8.2.2` | not stated | 1 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 2 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | skipped (iana) | IANA Considerations: the section assigns no obligation to an implementation |
| `13` | Normative References | 0 | skipped (references) | Normative References |
| `14` | Informative References | 0 | skipped (references) | Informative References |
| `B.1` | not stated | 0 | skipped (appendix-non-normative) | Appendix B is the decorrelation algorithm, presented as an example implementation aid |
| `D.1` | not stated | 0 | skipped (appendix-non-normative) | Appendix D is 'Fragment Handling Rationale'; D.1 states the problem and imposes nothing |
| `D.2` | not stated | 2 | walked | not stated |
| `D.3` | fragment-handling rationale | 0 | skipped (appendix-non-normative) | fragment-handling rationale |
| `D.4` | not stated | 2 | walked | not stated |
| `D.5` | fragment-handling rationale | 0 | skipped (appendix-non-normative) | fragment-handling rationale |
| `D.6` | fragment-handling rationale | 0 | skipped (appendix-non-normative) | fragment-handling rationale |
| `D.7` | fragment-handling rationale | 0 | skipped (appendix-non-normative) | fragment-handling rationale |
| `D.8` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the multicast inbound SA de-multiplexing obligation that site 4.1:1 maps: the SPI-collision sentence is the same three-step lookup stated from the receiver's side | A multicast-capable IPsec implementation MUST correctly de-multiplex inbound traffic even in the context of SPI collisions. |
| `4.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the same three-step SAD search as an externally-observable-behavior clause; the obligation is the ordering site 4.1:1 maps | In practice, an implementation may choose any method (or none at all) to accelerate this search, although its externally visible behavior MUST be functionally equivalent to having searched the SAD in the above order. |
| `4.4.1:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | a pointer sentence: it announces that the SPD elements enumerated in this section are mandatory, which is the obligation site 4.4.1:5 maps | However, this document does specify a standard set of SPD elements that all IPsec implementations MUST support. |
| `4.4.1.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the sentence defers to Section 4.4.1.1 'Name' for the forms that must be supported, which is the name-selector obligation site 4.4.1.1:5 maps | The forms that MUST be supported are described above in Section 4.4.1.1 under "Name". o PFP flags -- one per traffic selector. |
| `4.4.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | a parenthetical cross-reference to the Section 4.1 inbound SA mapping algorithm that site 4.1:1 maps | (See Section 4.1 for details on the algorithm that MUST be used for mapping inbound IPsec datagrams to SAs.) The following parameters are associated with each entry in the SAD. |
| `10:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a blanket conformance statement: 'All IPv4 IPsec implementations MUST comply with all requirements of this document' adds no obligation beyond the requirements the rest of the document states | All IPv4 IPsec implementations MUST comply with all requirements of this document. |
| `10:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the same blanket conformance statement for IPv6; boilerplate that names no distinct behavior | All IPv6 implementations MUST comply with all requirements of this document. |
| `D.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix D.2 is fragment-handling rationale restating the Section 7.1 rule that undefined port selectors are set to ANY | Accordingly, if a specific protocol value is used as a selector, and if that protocol has no port fields, then the port field selectors are to be ignored and ANY MUST be specified as the value for the port fields. |
| `D.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same rationale sentence for the ANY protocol case, restating the Section 7.1 rule site 7.1:3 maps | (In this context, ICMP TYPE and CODE values are lumped together as a single port field (for IKEv2 negotiation), as is the IPv6 Mobility Header TYPE value.) If the protocol selector is ANY, then this should be treated as equivalent to specifying a protocol for which no port fields are defined, and thus the port selectors should be ignored, and MUST be set to ANY. |
| `D.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix D.4 says 'this document says that implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified', an explicit restatement of the Section 7.4 stateful fragment checking obligation | To that end, this document says that implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified. |
| `D.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the Section 7.4 obligation to accept or reject such traffic through the ordinary SPD BYPASS/DISCARD conventions | An implementation also MUST permit a user or administrator to accept such traffic or reject such traffic using the SPD conventions described in Section 4.4.1. |
| `D.8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a description of the document's own structure ('this document offers 3 choices -- one MUST and two MAYs'), not an obligation on an implementation | Thus, this document offers 3 choices -- one MUST and two MAYs. |

## Superseded

No document obsoletes RFC 4301, so its obligations are stated where they were written.
