# RFC 4301 - Security Architecture for the Internet Protocol

Partial. Every requirement this repository extracted from RFC 4301, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 43.6% | 34 of 78 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 11.5% | 9 of 78 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 78 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 77.1% | 74 of 96 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 78 | of 81 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 6 | of 78 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 7.7% | 6 of 78 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 5.1% | 4 of 78 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 78 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 32.1% | 25 of 78 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 32 | of 78 gated MUSTs judged | 17 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 81 |
| Gated MUST-level | 78 |
| Not applicable, so out of scope | 6 |
| Declared gaps | 25 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 96 |
| Tagged units | 96 |
| Recorded audit verdicts | 32 |
| Discrimination records | 74 |
| Summary | `rfc/short/rfc4301.md` |
| Requirement shard | `rfc/requirements/rfc4301.md` |
| RFC text | `rfc/full/rfc4301.txt` |

## Enrolment

Enrolled: Security Architecture for IP (RFC 4301): native control-plane SPD/SAD projected to kernel XFRM. The extraction walk of 2026-09-21 read all 89 normative sites in `rfc/full/rfc4301.txt` and grew the checklist from 24 rows to 81 (72 MUST, 6 MUST NOT, 2 SHOULD, 1 MAY). 14 rows carry an annotation from the 2026-08-30 pass: 1 gap (port/ICMP selectors), 7 single-polarity positive (tunnel/transport modes, SPD present, negotiated selectors, manual+automated keying), 6 not-applicable (per-packet datapath kernel-delegated). The 57 rows added on 2026-09-21 carried no annotation and no test when added; the ruling pass of the same day classified every one (Support remaining).

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Native control-plane SPD/SAD model projected to kernel XFRM. The SPD carries all three dispositions of Section 4.4.1: a negotiated Child SA produces the PROTECT entries, and the operator `vpn ipsec policy` list produces the BYPASS and DISCARD entries, each with a selector, a direction and a total order. A selector holds the local and remote prefixes, the next-layer protocol, and a local and a remote port as an any-or-one-exact value. IKEv2 Child SAs install tunnel mode, or transport mode when USE_TRANSPORT_MODE is negotiated and echoed, and the RFC 4552 OSPFv3 path installs manual transport-mode ESP or AH. SAD entries carry the SPI, the address pair, the mode, the protocol, the algorithms, the replay window and a time-based lifetime. Per-packet SPD and SAD processing is kernel-delegated to XFRM. The 2026-09-21 walk widened the checklist to the whole document, so this paragraph now describes the covered part of a much larger row set rather than most of it.

**What the ledger says remains**

Lowered from `Supported` on 2026-08-30, after an extraction walk of [`rfc/full/rfc4301.txt`](https://github.com/ze-software/ze/blob/main/rfc/full/rfc4301.txt) found architecture obligations Ze does not meet. The implementation work is [`plan/immediate/spec-rfc4301-architecture-gaps.md`](https://github.com/ze-software/ze/blob/main/plan/immediate/spec-rfc4301-architecture-gaps.md), one phase per block.

- **Section 6, ICMP processing:** no control lets an administrator accept or reject unauthenticated ICMP error messages per ICMP type, and no check compares a protected transit ICMP error message payload header against the traffic selectors of the SA that carried it.
- **Section 8, DF bit and PMTU:** the DF treatment of a tunnel-mode SA is not configurable, and no per-SA PMTU value is held or aged. Section 4.4.2.1, SAD lifetimes: `newLifetimeState` ([`internal/component/ike/engine/rekey.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey.go)) assigns a time lifetime only, `softBytes` is never assigned, and the byte-count arm of `softExpired` is unreachable in a running daemon; [`plan/spec-ipsec-lifetime-volume.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-lifetime-volume.md) designs the byte-count lifetime. Section 5.1.2.1, outer header: the DSCP value of the outer tunnel header is not mapped for the domain the packet enters. Section 4.4.1.1, selectors: a port selector holds any port or one exact port rather than the range the section defines, and `SPParams` carries no ICMP type or code field. Each of those blocks is now gated by a checklist row of its own, added by the 2026-09-21 extraction walk: [`RFC4301-6.1.1-1`](#rfc4301-6.1.1-1) and [`RFC4301-6.1.1-2`](#rfc4301-6.1.1-2) for the unauthenticated-ICMP control, [`RFC4301-6.2-1`](#rfc4301-6.2-1) for the ICMP error payload check, [`RFC4301-8.1-1`](#rfc4301-8.1-1) and [`RFC4301-8.1-2`](#rfc4301-8.1-2) for the DF bit, [`RFC4301-8.2.2-1`](#rfc4301-8.2.2-1) for PMTU ageing, [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2) for the byte-count lifetime, and [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1) for the outer-header DSCP mapping. [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1) remains the gated selector row and its reason text is stale about ports. The same walk added 49 further rows, for obligations no earlier pass had recorded at all: the Section 3 connectivity and ESP-support rows, the Section 4.4.1 SPD management interface and decorrelation rows, the Section 4.4.1.1 OPAQUE and ICMP-range selector rows, the Section 4.4.2.1 SAD data items, the Section 4.4.3 PAD rows, the Section 4.5.2 key-splitting rule, the Section 5 processing-step rows, and the Section 7 fragment-handling rows. None of them carried a test on 2026-09-21. The ruling pass of that day implemented the Section 5 catch-all (`vpn ipsec unmatched`, [`RFC4301-5-1`](#rfc4301-5-1)), proved the Section 3.1, 4, 4.1, 4.4, 4.4.1, 4.4.1.1, 4.4.2.1, 5.1, 5.2 and 7.3 obligations the kernel performs on state Ze installs at the XFRM boundary, and scheduled every remaining MUST as a `{gap}` naming its spec: the Section 6 ICMP block, the Section 8 DF bit and PMTU block, the outer-header DSCP mapping and the ICMP type/code selector in [`plan/immediate/spec-rfc4301-architecture-gaps.md`](https://github.com/ze-software/ze/blob/main/plan/immediate/spec-rfc4301-architecture-gaps.md); the byte-count lifetime in [`plan/spec-ipsec-lifetime-volume.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-lifetime-volume.md); OPAQUE ports in the operator list in [`plan/spec-ipsec-opaque-selector-port-mask.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-opaque-selector-port-mask.md); multiple SPDs, the user override control and the name selector in [`plan/spec-ipsec-spd-selection.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-spd-selection.md); the dual-family PAD range in [`plan/spec-ipsec-pad-dual-family-range.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-pad-dual-family-range.md); and NON_FIRST_FRAGMENTS_ALSO in [`plan/spec-ipsec-non-first-fragments-also.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-non-first-fragments-also.md).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 34 | one part of the gated population |
| Annotated instead of tested | 44 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **78** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (34):** [`RFC4301-4.2-1`](#rfc4301-4.2-1), [`RFC4301-4.4.3.1-1`](#rfc4301-4.4.3.1-1), [`RFC4301-4.4.3.1-2`](#rfc4301-4.4.3.1-2), [`RFC4301-4.4.1-4`](#rfc4301-4.4.1-4), [`RFC4301-7.4-1`](#rfc4301-7.4-1), [`RFC4301-7.4-2`](#rfc4301-7.4-2), [`RFC4301-3.1-1`](#rfc4301-3.1-1), [`RFC4301-4-1`](#rfc4301-4-1), [`RFC4301-4.1-8`](#rfc4301-4.1-8), [`RFC4301-4.1-9`](#rfc4301-4.1-9), [`RFC4301-4.4-1`](#rfc4301-4.4-1), [`RFC4301-4.4.1-6`](#rfc4301-4.4.1-6), [`RFC4301-4.4.1-7`](#rfc4301-4.4.1-7), [`RFC4301-4.4.1-9`](#rfc4301-4.4.1-9), [`RFC4301-4.4.1-10`](#rfc4301-4.4.1-10), [`RFC4301-4.4.1.1-3`](#rfc4301-4.4.1.1-3), [`RFC4301-4.4.2.1-1`](#rfc4301-4.4.2.1-1), [`RFC4301-4.4.2.1-3`](#rfc4301-4.4.2.1-3), [`RFC4301-4.4.3.1-3`](#rfc4301-4.4.3.1-3), [`RFC4301-4.4.3.2-1`](#rfc4301-4.4.3.2-1), [`RFC4301-4.5.2-1`](#rfc4301-4.5.2-1), [`RFC4301-4.5.3-1`](#rfc4301-4.5.3-1), [`RFC4301-5-1`](#rfc4301-5-1), [`RFC4301-5.1-1`](#rfc4301-5.1-1), [`RFC4301-5.1-2`](#rfc4301-5.1-2), [`RFC4301-5.2-3`](#rfc4301-5.2-3), [`RFC4301-5.2-4`](#rfc4301-5.2-4), [`RFC4301-5.2-5`](#rfc4301-5.2-5), [`RFC4301-7.1-1`](#rfc4301-7.1-1), [`RFC4301-7.1-2`](#rfc4301-7.1-2), [`RFC4301-7.1-3`](#rfc4301-7.1-3), [`RFC4301-7.3-3`](#rfc4301-7.3-3), [`RFC4301-7.3-4`](#rfc4301-7.3-4), [`RFC4301-7.3-5`](#rfc4301-7.3-5)

**Annotated instead of tested (44):** [`RFC4301-4.1-1`](#rfc4301-4.1-1), [`RFC4301-4.1-2`](#rfc4301-4.1-2), [`RFC4301-4.1-3`](#rfc4301-4.1-3), [`RFC4301-4.1-4`](#rfc4301-4.1-4), [`RFC4301-4.1-5`](#rfc4301-4.1-5), [`RFC4301-4.4.1-1`](#rfc4301-4.4.1-1), [`RFC4301-4.4.1-2`](#rfc4301-4.4.1-2), [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1), [`RFC4301-4.4.2-1`](#rfc4301-4.4.2-1), [`RFC4301-4.5-1`](#rfc4301-4.5-1), [`RFC4301-5.2-1`](#rfc4301-5.2-1), [`RFC4301-5.2-2`](#rfc4301-5.2-2), [`RFC4301-4.1-6`](#rfc4301-4.1-6), [`RFC4301-7-1`](#rfc4301-7-1), [`RFC4301-3.2-1`](#rfc4301-3.2-1), [`RFC4301-4.1-10`](#rfc4301-4.1-10), [`RFC4301-4.1-11`](#rfc4301-4.1-11), [`RFC4301-4.4.1-5`](#rfc4301-4.4.1-5), [`RFC4301-4.4.1-8`](#rfc4301-4.4.1-8), [`RFC4301-4.4.1.1-2`](#rfc4301-4.4.1.1-2), [`RFC4301-4.4.1.1-4`](#rfc4301-4.4.1.1-4), [`RFC4301-4.4.1.1-5`](#rfc4301-4.4.1.1-5), [`RFC4301-4.4.1.2-1`](#rfc4301-4.4.1.2-1), [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2), [`RFC4301-4.4.3.3-1`](#rfc4301-4.4.3.3-1), [`RFC4301-5.1-3`](#rfc4301-5.1-3), [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1), [`RFC4301-6-1`](#rfc4301-6-1), [`RFC4301-6.1.1-1`](#rfc4301-6.1.1-1), [`RFC4301-6.1.1-2`](#rfc4301-6.1.1-2), [`RFC4301-6.1.2-1`](#rfc4301-6.1.2-1), [`RFC4301-6.2-1`](#rfc4301-6.2-1), [`RFC4301-6.2-2`](#rfc4301-6.2-2), [`RFC4301-6.2-3`](#rfc4301-6.2-3), [`RFC4301-6.2-4`](#rfc4301-6.2-4), [`RFC4301-6.2-5`](#rfc4301-6.2-5), [`RFC4301-7.2-1`](#rfc4301-7.2-1), [`RFC4301-7.2-2`](#rfc4301-7.2-2), [`RFC4301-7.3-1`](#rfc4301-7.3-1), [`RFC4301-7.3-2`](#rfc4301-7.3-2), [`RFC4301-8.1-1`](#rfc4301-8.1-1), [`RFC4301-8.1-2`](#rfc4301-8.1-2), [`RFC4301-8.2.1-1`](#rfc4301-8.2.1-1), [`RFC4301-8.2.2-1`](#rfc4301-8.2.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4301-4.1-1` | a) A host implementation of IPsec MUST support both transport and tunnel mode. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L169). **positive:** `unit/verify` [`TestIPsecSAPerDestinationWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L226). **negative:** no negative test. **{single-polarity}:** ze projects both modes -- IKE child SAs install tunnel-mode ESP and OSPFv3 RFC 4552 installs transport-mode ESP/AH; a capability-presence MUST has no meaningful negative (internal/component/ike/engine/child.go:224, :253, internal/plugins/ospf/ipsec_install.go:413, :441) |
| `RFC4301-4.1-2` | A security gateway MUST support tunnel mode (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L184). **negative:** no negative test. **{single-polarity}:** ze (a security gateway) installs tunnel-mode ESP for every IKE-negotiated peer SA and policy (internal/component/ike/engine/child.go:224, :253, :281, :295) |
| `RFC4301-4.1-3` | Aside from the two exceptions below, whenever either end of a security association is a security gateway, the SA MUST be tunnel mode. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L171). **negative:** no negative test. **{single-polarity}:** the IKE child-SA path hardcodes tunnel mode for every peer SA, so a peer SA can never be transport (internal/component/ike/engine/child.go:40, :224, :253) |
| `RFC4301-4.1-4` | IKE creates pairs of SAs, so for simplicity, we choose to require that both SAs in a pair be of the same mode, transport or tunnel. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L173). **negative:** no negative test. **{single-polarity}:** the inbound and outbound child SAs of a pair are both built with modeTunnel, so the pair is always same-mode (internal/component/ike/engine/child.go:224, :253) |
| `RFC4301-4.1-5` | To permit this, the IPsec implementation MUST permit establishment and maintenance of multiple SAs between a given sender and receiver, with the same selectors. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** multiple SAs with identical selectors coexist by SPI in the kernel XFRM SAD and ze's SPI-keyed installs do not forbid this; the DSCP classification that selects among them for QoS is a datapath function ze does not model (internal/component/ike/dataplane/dataplane.go:81-108) |
| `RFC4301-4.2-1` | Note: A compliant implementation MUST NOT allow instantiation of an ESP SA that employs both NULL encryption and no integrity algorithm. (§4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L124). **negative:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L102) |
| `RFC4301-4.4.1-1` | An IPsec implementation MUST have at least one SPD, and it MAY support multiple SPDs, if appropriate for the context in which the IPsec implementation operates. (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L150). **negative:** no negative test. **{single-polarity}:** ze maintains an SPD policy model (SPParams) and installs at least the PROTECT entries into the kernel SPD via both the IKE and OSPFv3 paths (internal/component/ike/dataplane/dataplane.go:145, engine/child.go:276, :290, internal/plugins/ospf/ipsec_install.go:432-452) |
| `RFC4301-4.4.1-2` | The SPD, or relevant caches, must be consulted during the processing of all traffic (inbound and outbound), including traffic not protected by IPsec, that traverses the IPsec boundary. This includes IPsec management traffic such as IKE. (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** per-packet SPD consultation for every crossing packet is a kernel XFRM datapath function; ze populates the kernel SPD but does not process packets |
| `RFC4301-4.4.1.1-1` | The following selector parameters MUST be supported by all IPsec implementations to facilitate control of SA granularity. (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's policy model carries IP-prefix and next-layer-protocol selectors only; the IKE traffic-selector negotiation discards ports/protocol into an address-only net.IPNet, and SPParams has no port or ICMP-type/code field (internal/component/ike/dataplane/dataplane.go:110-130 exists; ports/ICMP missing at internal/component/ike/engine/sa.go:128-129, engine/initiator.go:325) |
| `RFC4301-4.4.2-1` | For each of the selectors defined in Section 4.4.1.1, the entry for an inbound SA in the SAD MUST be initially populated with the value or values negotiated at the time the SA was created. (§4.4.2) | MUST | 4.4.2 | **positive:** `unit/verify` [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L412). **positive:** `unit/verify` [`TestNarrowedSelectorsReachTheInstalledPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L423). **negative:** no negative test. **{single-polarity}:** ze captures the RFC 7296-narrowed negotiated traffic selectors and projects them into the inbound require-policy; the per-packet check against them is kernel-enforced (internal/component/ike/engine/child.go:155-164, :276-288) |
| `RFC4301-4.5-1` | All IPsec implementations MUST support both manual and automated SA and cryptographic key management. (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L144). **positive:** `unit/verify` [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L110). **negative:** no negative test. **{single-polarity}:** ze implements automated keying via its native IKEv2 engine and manual keying via the OSPFv3 RFC 4552 config, both installing SAs through the same dataplane seam (internal/component/ike/engine/child.go:199-307, internal/plugins/ospf/ipsec_install.go:295-342) |
| `RFC4301-5.2-1` | Since the SPD-I is just a part of the SPD, if a packet that is looked up in the SPD-I cannot be matched to an entry there, then the packet MUST be discarded. (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** dropping inbound packets that fail the SPD-I match is a kernel XFRM datapath function; ze installs the inbound require-policies but does not process packets (internal/component/ike/engine/child.go:276) |
| `RFC4301-5.2-2` | Then match the packet against the inbound selectors identified by the SAD entry to verify that the received packet is appropriate for the SA via which it was received. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** post-decapsulation inner-selector verification against the SAD is a kernel XFRM datapath function; the kernel verifies the inner packet against the inbound policy ze installs |
| `RFC4301-4.1-6` | If an IPsec implementation supports multicast, then it MUST support multicast SAs using the algorithm below for mapping inbound IPsec datagrams to SAs. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the three-step inbound SAD lookup (SPI+dst+src / SPI+dst / SPI) is a per-packet kernel XFRM function; ze performs no inbound SAD lookups |
| `RFC4301-7-1` | Note: AH and ESP cannot be applied using transport mode to IPv4 packets that are fragments. Only tunnel mode can be employed in such cases. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies transport mode only to IPv6 OSPFv3 traffic (IPv4-family IPsec is rejected at config), so transport-mode IPsec on an IPv4 fragment structurally cannot arise, and per-packet fragment handling is kernel-delegated (internal/plugins/ospf/config_ipsec.go:21-22) |
| `RFC4301-4.4.1-3` | Every SPD SHOULD have a nominal, final entry that matches anything that is otherwise unmatched, and discards it. (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-7-2` | Implementations SHOULD use the approach described in the Path MTU Discovery document (RFC 1191 [MD90], Section 6.3), which suggests periodically resetting the PMTU to the first-hop data-link MTU and then letting the normal PMTU Discovery processes update the PMTU as necessary. (§8.2.2) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.1-7` | Where traffic is destined for a security gateway, e.g., Simple Network Management Protocol (SNMP) commands, the security gateway is acting as a host and transport mode is allowed. (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.3.1-1` | The specific syntax used by an implementation to accommodate sub-tree matching for distinguished names, domain names or RFC 822 e-mail addresses is a local matter. But, at a minimum, sub-tree matching of the sort described above MUST be supported. (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** `unit/verify` [`TestPadSubtreeAdmitsAPeerBeneathIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L35). **positive:** `unit/verify` [`TestPadSubtreeStillBindsTheCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L185). **positive:** `unit/verify` [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/validate_identity_test.go#L166). **negative:** `unit/verify` [`TestPadKeyIDStaysExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L111). **negative:** `unit/verify` [`TestPadSubtreeRefusesAPeerOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L69) |
| `RFC4301-4.4.3.1-2` | For IPv4 and IPv6 addresses, the same address range syntax used for SPD entries MUST be supported. (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** `unit/verify` [`TestPadAddressRangeAdmitsAnAddressInsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L129). **positive:** `unit/verify` [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/validate_identity_test.go#L167). **negative:** `unit/verify` [`TestPadAddressRangeRefusesWhatIsOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L155) |
| `RFC4301-4.4.1-4` | Thus, a user or administrator MUST be able to order the entries to express a desired access control policy. (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestSPDPolicyMirrorsTheInboundSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L94). **positive:** `unit/verify` [`TestSpdOperatorOrdersOverlappingPeers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L20). **negative:** `unit/verify` [`TestSPDPolicyOrderInsideBypassBandRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L123). **negative:** `unit/verify` [`TestSpdOrderCannotCaptureTheIKEControlPlane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L107). **negative:** `unit/verify` [`TestSpdUnstatedOrderTakesTheDefaultRank`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L67) |
| `RFC4301-7.4-1` | All implementations MUST support DISCARDing of fragments using the normal SPD packet classification mechanisms (§7.4) | MUST | 7.4 | **positive:** `unit/verify` [`TestDiscardPolicyReachesTheBackend`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L45). **positive:** `unit/verify` [`TestDiscardPolicyReachesTheKernelAsBlock`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L41). **positive:** `unit/verify` [`TestPolicyReadbackReportsDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L98). **positive:** `unit/verify` [`TestSPDPolicyCarriesTheDiscardDisposition`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L52). **positive:** `unit/verify` [`TestVPPSPDActionCarriesEveryDisposition`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_vpp_test.go#L23). **negative:** `unit/verify` [`TestBypassPolicyIsNotBlocked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L64). **negative:** `unit/verify` [`TestBypassPolicyReachesTheBackendAsBypass`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L75). **negative:** `unit/verify` [`TestSPDPolicyBypassIsNotADiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L83) |
| `RFC4301-7.4-2` | All implementations MUST support stateful fragment checking to accommodate BYPASS traffic for which a non-trivial port range is specified (§7.4; Appendix D.4 restates it as "implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified") | MUST | 7.4 | **positive:** `unit/verify` [`TestPortScopedBypassNeverReachesTheForwardPath`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_test.go#L74). **positive:** `unit/verify` [`TestXFRMPortScopedBypassIsNeverStoredOnTheForwardPath`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_fragment_integration_linux_test.go#L69). **negative:** `unit/verify` [`TestIKEBypassIsPortScopedSoSection74Binds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_test.go#L133) |
| `RFC4301-3.1-1` | A compliant host implementation MUST support (a) and (c) and a compliant security gateway must support all three of these forms of connectivity, since under certain circumstances a security gateway acts as a host (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L131). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L194) |
| `RFC4301-3.2-1` | IPsec implementations MUST support ESP and MAY support AH (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC4301ChildSAIsInstalledAsESP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_esp_test.go#L18). **negative:** no negative test. **{single-polarity}:** "support ESP" has no violating input, every IKE Child SA installs ESP (internal/component/ike/engine/child.go::installChildSA) and AH is the MAY Ze offers on the OSPFv3 manual path only |
| `RFC4301-4-1` | All implementations of AH or ESP MUST support the concept of an SA as described below (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L69). **negative:** `unit/verify` [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L113) |
| `RFC4301-4.1-8` | The indication of whether source and destination address matching is required to map inbound IPsec traffic to SAs MUST be set either as a side effect of manual SA configuration or via negotiation using an SA management protocol, e.g., IKE or Group Domain of Interpretation (GDOI) [RFC3547]. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L70). **negative:** `unit/verify` [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L114) |
| `RFC4301-4.1-9` | The receiver MUST process the packets from the different SAs without prejudice. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L71). **negative:** `unit/verify` [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L115) |
| `RFC4301-4.1-10` | In transport mode, the DSCP value might change en route, but this should not cause problems with respect to IPsec processing since the value is not employed for SA selection and MUST NOT be checked as part of SA/packet validation. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes no DSCP into the state and xfrmPolicyFromParams none into the selector, so no value Ze installs is read against DSCP |
| `RFC4301-4.1-11` | In IPv6, the security protocol header appears after the base IP header and selected extension headers, but may appear before or after destination options; it MUST appear before next layer protocols (e.g., TCP, UDP, Stream Control Transmission Protocol (SCTP)). (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the ESP state and the kernel output path builds every header, so no value Ze writes decides where the ESP header sits |
| `RFC4301-4.4-1` | The model described below is nominal; implementations need not match details of this model as presented, but the external behavior of implementations MUST correspond to the externally observable characteristics of this model in order to be compliant. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L132). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L195) |
| `RFC4301-4.4.1-5` | However, if an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing. (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze holds one SPD and no selection function; plan/spec-ipsec-spd-selection.md |
| `RFC4301-4.4.1-6` | The SPD MUST permit a user or administrator to specify policy entries as follows: (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC4301SPDPermitsEachDispositionInEachDatabase`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L35). **negative:** `unit/verify` [`TestRFC4301SPDRefusesADispositionItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L70) |
| `RFC4301-4.4.1-7` | For every IPsec implementation, there MUST be a management interface that allows a user or system administrator to manage the SPD (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC4301SPDIsManagedThroughTheConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L100). **negative:** `unit/verify` [`TestRFC4301SPDInterfaceRefusesWhatItCannotProgram`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L143) |
| `RFC4301-4.4.1-8` | (The means of signaling such requests to the IPsec implementation are outside the scope of this standard.) However, the system administrator MUST be able to specify whether or not a user or application can override (default) system policies. (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no user or application override control exists in the SPD model; plan/spec-ipsec-spd-selection.md |
| `RFC4301-4.4.1-9` | Note that when an SPD entry is decorrelated all the resulting entries MUST be linked together, so that all members of the group derived from an individual, SPD entry (prior to decorrelation) can all be placed into caches and into the SAD at the same time. (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L133). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L196) |
| `RFC4301-4.4.1-10` | If the SPD is not decorrelated, caching is not allowed and an ordered search of SPD MUST be performed to verify that inbound traffic arriving on an SA is consistent with the access control policy expressed in the SPD. (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L134). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L197) |
| `RFC4301-4.4.1.1-2` | Note that the Local and Remote ports may not be available in the case of receipt of a fragmented packet or if the port fields have been protected by IPsec (encrypted); thus, a value of OPAQUE also MUST be supported. (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the operator SPD list refuses OPAQUE and the XFRM selector cannot express port 0 mask 0xffff; plan/spec-ipsec-opaque-selector-port-mask.md |
| `RFC4301-4.4.1.1-3` | If the SA requires a port value other than ANY or OPAQUE, an arriving fragment without ports MUST be discarded (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L135). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L198) |
| `RFC4301-4.4.1.1-4` | Given a policy entry with a range of Types (T-start to T-end) and a range of Codes (C-start to C-end), and an ICMP packet with Type t and Code c, an implementation MUST test for a match using (T-start*256) + C-start <= (t*256) + c <= (T-end*256) + C-end (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** SPParams carries no ICMP type or code selector; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-4.4.1.1-5` | The name used to match this field is communicated during the IKE negotiation in the ID payload. In this context, the initiator's Source IP address (inner IP header in tunnel mode) is bound to the Remote IP address in the SAD entry created by the IKE negotiation. This address overrides the Remote IP address value in the SPD, when the SPD entry is selected in this fashion. All IPsec implementations MUST support this use of names. (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no name-valued selector selects an operator SPD entry during IKE; plan/spec-ipsec-spd-selection.md |
| `RFC4301-4.4.1.2-1` | Until IKE provides a facility that conveys the semantics that are expressed in the SPD via selector sets (as described below), users MUST NOT include multiple selector sets in a single SPD entry unless the access control intent aligns with the IKE "mix and match" semantics. (§4.4.1.2) | MUST NOT | 4.4.1.2 | **positive:** `unit/verify` [`TestRFC4301SPDEntryHoldsOneSelectorSet`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_selector_set_test.go#L10). **negative:** no negative test. **{single-polarity}:** internal/component/ike/ipsec/spd_policy.go::SPDPolicy holds one local and one remote prefix leaf, so no input can carry a second selector set |
| `RFC4301-4.4.2.1-1` | The following data items MUST be in the SAD: (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L72). **negative:** `unit/verify` [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L116) |
| `RFC4301-4.4.2.1-2` | A compliant implementation MUST support both types of lifetimes, and MUST support a simultaneous use of both. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** newLifetimeState sets a time lifetime only and never a byte count; plan/spec-ipsec-lifetime-volume.md |
| `RFC4301-4.4.2.1-3` | Note that implementations MUST be able to handle having the counters at the ends of an SA get out of synch, e.g., because of packet loss or because the implementations at each end of the SA aren't doing things the same way. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L73). **negative:** `unit/verify` [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L117) |
| `RFC4301-4.4.3.1-3` | For this name type, only exact-match syntax MUST be supported (since there is no explicit structure for this ID type). (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** `unit/verify` [`TestRFC4301PadOtherIDTypeAdmitsAnExactMatch`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L18). **negative:** `unit/verify` [`TestRFC4301PadOtherIDTypeRefusesEverythingButExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L35) |
| `RFC4301-4.4.3.2-1` | Thus, implementations MUST provide a means for an administrator to require a match between an asserted IKE ID and the subject name or subject alt name in a certificate. (§4.4.3.2) | MUST | 4.4.3.2 | **positive:** `unit/verify` [`TestRFC4301PadRequiresTheCertificateToCarryTheIKEID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L66). **negative:** `unit/verify` [`TestRFC4301PadRefusesACertificateWithoutTheIKEID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L96) |
| `RFC4301-4.4.3.3-1` | (A peer may be authorized for both address types, so there MUST be provision for both a v4 and a v6 address range.) (§4.4.3.3) | MUST | 4.4.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** one remote-id string holds one address range, not a v4 and a v6 range; plan/spec-ipsec-pad-dual-family-range.md |
| `RFC4301-4.5.2-1` | To ensure that the IPsec implementations at each end of the SA use the same bits for the same keys, and irrespective of which part of the system divides the string of bits into individual keys, the encryption keys MUST be taken from the first (left-most, high-order) bits and the integrity keys MUST be taken from the remaining bits. (§4.5.2) | MUST | 4.5.2 | **positive:** `unit/verify` [`TestRFC4301KeymatEncryptionKeysLeadEachSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4301_keymat_test.go#L45). **negative:** `unit/verify` [`TestRFC4301KeymatIntegrityKeysNeverLead`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4301_keymat_test.go#L72) |
| `RFC4301-4.5.3-1` | To address these problems, an IPsec-supporting host or security gateway MUST have an administrative interface that allows the user/administrator to configure the address of one or more security gateways for ranges of destination addresses that require its use. (§4.5.3) | MUST | 4.5.3 | **positive:** `unit/verify` [`TestRFC4301GatewayIsConfiguredForDestinationRanges`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_gateway_test.go#L39). **negative:** `unit/verify` [`TestRFC4301GatewayRefusesAMalformedDestinationRange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_gateway_test.go#L65) |
| `RFC4301-5-1` | If no policy is found in the SPD that matches a packet (for either inbound or outbound traffic), the packet MUST be discarded (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4301UnmatchedDiscardIsTheLastEntryOfEveryDatabase`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_unmatched_test.go#L23). **positive:** `unit/verify` [`TestRFC4301UnmatchedLeafIsReadAsWritten`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_unmatched_test.go#L23). **negative:** `unit/verify` [`TestRFC4301UnmatchedLeafRefusesAnUnknownWord`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_unmatched_test.go#L42). **negative:** `unit/verify` [`TestRFC4301UnmatchedNeverDiscardsWhatTheOperatorDidNotAskToDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_unmatched_test.go#L80) |
| `RFC4301-5.1-1` | IPsec MUST perform the following steps when processing outbound packets: (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L136). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L199) |
| `RFC4301-5.1-2` | If so, there MUST be an entry in SPD-I database that permits inbound bypassing of the packet, otherwise the packet will be discarded. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC4301BypassReturnTrafficHasAnSPDIEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L48). **negative:** `unit/verify` [`TestRFC4301BypassSPDIEntryIsNeverAnUnmirroredCopy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L69) |
| `RFC4301-5.1-3` | With regard to determining and enforcing the PMTU of an SA, the IPsec system MUST follow the steps described in Section 8.2 (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no per-SA PMTU is held or aged; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-5.1.2.1-1` | If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain (§5.1.2.1) | MUST | 5.1.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the outer-header DSCP is not mapped for the domain entered; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-5.2-3` | IPsec MUST perform the following steps: (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L137). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L200) |
| `RFC4301-5.2-4` | IKE traffic MUST have an explicit BYPASS entry in the SPD (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC4301IKETrafficHasAnExplicitBypassEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ike_bypass_test.go#L21). **negative:** `unit/verify` [`TestRFC4301IKEBypassIsNeverAProtectOrAWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ike_bypass_test.go#L54) |
| `RFC4301-5.2-5` | If so, then as with ALL outbound traffic that is to be bypassed, the packet MUST be matched against an SPD-O entry. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC4301InboundBypassReturnTrafficHasAnSPDOEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L88). **negative:** `unit/verify` [`TestRFC4301BypassSPDOEntryIsNeverAnUnmirroredCopy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L109) |
| `RFC4301-6-1` | Disposition of non-error, ICMP messages (that are not addressed to the IPsec implementation itself) MUST be explicitly accounted for using SPD entries. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no ICMP type or code selector and no default entry accounts for non-error ICMP; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-6.1.1-1` | To accommodate both ends of this spectrum, a compliant IPsec implementation MUST permit a local administrator to configure an IPsec implementation to accept or reject unauthenticated ICMP traffic. (§6.1.1) | MUST | 6.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no control accepts or rejects unauthenticated ICMP; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-6.1.1-2` | This control MUST be at the granularity of ICMP type and MAY be at the granularity of ICMP type and code. (§6.1.1) | MUST | 6.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no ICMP-type granularity exists for the control; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-6.1.2-1` | Thus, implementers MUST provide controls to allow local administrators to constrain the processing of ICMP error messages received on the protected side of the boundary, and directed to the IPsec implementation. (§6.1.2) | MUST | 6.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no control constrains ICMP error processing at the IPsec boundary; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-6.2-1` | Thus, an IPsec implementation MUST be configurable to check that this payload header information is consistent with the SA via which it arrives. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no check compares an ICMP error payload header against the SA selectors; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-6.2-2` | IPsec senders and receivers MUST support the following processing for ICMP error messages that are sent and received via SAs. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Section 6.2 ICMP error processing is absent; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-6.2-3` | If no SA exists that would carry the outbound ICMP message in question, and if no SPD entry would allow carriage of this outbound ICMP error message, then an IPsec implementation MUST map the message to the SA that would carry the return traffic associated with the packet that triggered the ICMP error message. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** whether the kernel maps an outbound ICMP error to the return SA is unverified and Ze holds no producer for it; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-6.2-4` | If an IPsec implementation receives an inbound ICMP error message on an SA, and the IP and ICMP headers of the message do not match the traffic selectors for the SA, the receiver MUST process the received message in a special fashion. Specifically, the receiver must extract the header of the triggering packet from the ICMP payload, and reverse fields as described above to determine if the packet is consistent with the selectors for the SA via which the ICMP error message was received. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no special processing of a mismatching inbound ICMP error exists; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-6.2-5` | If the packet fails this check, the IPsec implementation MUST NOT forwarded the ICMP message to the destination. (§6.2) | MUST NOT | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no payload check exists, so nothing refuses to forward on its failure; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-7.1-1` | All implementations MUST support tunnel mode SAs that are configured to pass traffic without regard to port field (or ICMP type/code or Mobility Header type) values (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC4301TunnelSAPassesTrafficWithoutRegardToPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_portless_test.go#L47). **negative:** `unit/verify` [`TestRFC4301PortScopedSAIsNeverInstalledAsAnyPort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_portless_test.go#L71) |
| `RFC4301-7.1-2` | If the SA will carry traffic for specified protocols, the selector set for the SA MUST specify the port fields (or ICMP type/code or Mobility Header type) as ANY. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC4301ProtocolScopedSASpecifiesPortsAsAny`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_portless_test.go#L99). **negative:** `unit/verify` [`TestRFC4301AnyPortFormNeverLeaksAPortNumber`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_portless_test.go#L123) |
| `RFC4301-7.1-3` | If the SA will carry traffic without regard to a specific protocol value (i.e., ANY is specified as the (Next Layer) protocol selector value), then the port field values are undefined and MUST be set to ANY as well. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC4301ProtocolAnyEntryCarriesAnyPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L178). **negative:** `unit/verify` [`TestRFC4301ProtocolAnyEntryRefusesAPort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L199) |
| `RFC4301-7.2-1` | Receivers MUST perform a minimum offset check on IPv4 (non-initial) fragments to protect against overlapping fragment attacks when SAs of this type are employed. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/engine/child.go::childPolicyParams builds one policy pair per Child SA and no separate non-initial-fragment SA, so approach #2 of Section 7.2 is never taken and no value Ze installs decides the offset check the kernel reassembly performs |
| `RFC4301-7.2-2` | Specific port (or ICMP type/code or Mobility Header type) selector values will be used to define SAs to carry initial fragments and non-fragmented packets. This approach can be used if a user or administrator wants to create one or more tunnel mode SAs between the same Local/Remote addresses that discriminate based on port (or ICMP type/code or Mobility Header type) fields. These SAs MUST have non-trivial protocol selector values, otherwise approach #1 above MUST be used. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/engine/child.go::childPolicyParams builds no fragment-carrying SA, so the protocol selector this sentence constrains is never written by Ze |
| `RFC4301-7.3-1` | Implementations that will transmit non-initial fragments on a tunnel mode SA that makes use of non-trivial port (or ICMP type/code or MH type) selectors MUST notify a peer via the IKE NOTIFY NON_FIRST_FRAGMENTS_ALSO payload (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never sends NON_FIRST_FRAGMENTS_ALSO; plan/spec-ipsec-non-first-fragments-also.md |
| `RFC4301-7.3-2` | The peer MUST reject this proposal if it will not accept non-initial fragments in this context. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never parses or answers NON_FIRST_FRAGMENTS_ALSO; plan/spec-ipsec-non-first-fragments-also.md |
| `RFC4301-7.3-3` | If an implementation does not successfully negotiate transmission of non-initial fragments for such an SA, it MUST NOT send such fragments over the SA (§7.3) | MUST NOT | 7.3 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L138). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L201) |
| `RFC4301-7.3-4` | However, a receiver MUST discard non-initial fragments that arrive on an SA with non-trivial port (or ICMP type/code or MH type) selector values unless this feature has been negotiated. (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L139). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L202) |
| `RFC4301-7.3-5` | Also, the receiver MUST discard non-initial fragments that do not comply with the security policy applied to the overall packet. (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L140). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L203) |
| `RFC4301-8.1-1` | All IPsec implementations MUST support the option of copying the DF bit from an outbound packet to the tunnel mode header that it emits, when traffic is carried via a tunnel mode SA (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no option copies the DF bit into the tunnel header per SA; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-8.1-2` | This means that it MUST be possible to configure the implementation's treatment of the DF bit (set, clear, copy from inner header) for each SA. (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the DF treatment of an SA is not configurable; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-8.2.1-1` | When such traffic arrives, if the traffic would exceed the updated PMTU value the traffic MUST be handled as follows: Case 1: Original (cleartext) packet is IPv4 and has the DF bit set. The implementation SHOULD discard the packet and send a PMTU ICMP message. Case 2: Original (cleartext) packet is IPv4 and has the DF bit clear. The implementation SHOULD fragment (before or after encryption per its configuration) and then forward the fragments. It SHOULD NOT send a PMTU ICMP message. Case 3: Original (cleartext) packet is IPv6. The implementation SHOULD discard the packet and send a PMTU ICMP message. (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no per-SA PMTU exists to exceed; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-8.2.2-1` | In all IPsec implementations, the PMTU associated with an SA MUST be "aged" and some mechanism is required to update the PMTU in a timely manner (§8.2.2) | MUST | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no per-SA PMTU is held or aged; plan/immediate/spec-rfc4301-architecture-gaps.md |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4301-4.1-5`](#rfc4301-4.1-5) To permit this, the IPsec implementation MUST permit establishment and maintenance of multiple SAs between a given sender and receiver, with the same selectors. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: multiple SAs with identical selectors coexist by SPI in the kernel XFRM SAD and ze's SPI-keyed installs do not forbid this; the DSCP classification that selects among them for QoS is a datapath function ze does not model (internal/component/ike/dataplane/dataplane.go:81-108) |
| [`RFC4301-4.4.1-2`](#rfc4301-4.4.1-2) The SPD, or relevant caches, must be consulted during the processing of all traffic (inbound and outbound), including traffic not protected by IPsec, that traverses the IPsec boundary. This includes IPsec management traffic such as IKE. (§4.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: per-packet SPD consultation for every crossing packet is a kernel XFRM datapath function; ze populates the kernel SPD but does not process packets |
| [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1) The following selector parameters MUST be supported by all IPsec implementations to facilitate control of SA granularity. (§4.4.1.1) | {gap}, no test | ze's policy model carries IP-prefix and next-layer-protocol selectors only; the IKE traffic-selector negotiation discards ports/protocol into an address-only net.IPNet, and SPParams has no port or ICMP-type/code field (internal/component/ike/dataplane/dataplane.go:110-130 exists; ports/ICMP missing at internal/component/ike/engine/sa.go:128-129, engine/initiator.go:325) |
| [`RFC4301-5.2-1`](#rfc4301-5.2-1) Since the SPD-I is just a part of the SPD, if a packet that is looked up in the SPD-I cannot be matched to an entry there, then the packet MUST be discarded. (§4.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: dropping inbound packets that fail the SPD-I match is a kernel XFRM datapath function; ze installs the inbound require-policies but does not process packets (internal/component/ike/engine/child.go:276) |
| [`RFC4301-5.2-2`](#rfc4301-5.2-2) Then match the packet against the inbound selectors identified by the SAD entry to verify that the received packet is appropriate for the SA via which it was received. (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: post-decapsulation inner-selector verification against the SAD is a kernel XFRM datapath function; the kernel verifies the inner packet against the inbound policy ze installs |
| [`RFC4301-4.1-6`](#rfc4301-4.1-6) If an IPsec implementation supports multicast, then it MUST support multicast SAs using the algorithm below for mapping inbound IPsec datagrams to SAs. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: the three-step inbound SAD lookup (SPI+dst+src / SPI+dst / SPI) is a per-packet kernel XFRM function; ze performs no inbound SAD lookups |
| [`RFC4301-7-1`](#rfc4301-7-1) Note: AH and ESP cannot be applied using transport mode to IPv4 packets that are fragments. Only tunnel mode can be employed in such cases. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze applies transport mode only to IPv6 OSPFv3 traffic (IPv4-family IPsec is rejected at config), so transport-mode IPsec on an IPv4 fragment structurally cannot arise, and per-packet fragment handling is kernel-delegated (internal/plugins/ospf/config_ipsec.go:21-22) |
| [`RFC4301-4.1-10`](#rfc4301-4.1-10) In transport mode, the DSCP value might change en route, but this should not cause problems with respect to IPsec processing since the value is not employed for SA selection and MUST NOT be checked as part of SA/packet validation. (§4.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes no DSCP into the state and xfrmPolicyFromParams none into the selector, so no value Ze installs is read against DSCP |
| [`RFC4301-4.1-11`](#rfc4301-4.1-11) In IPv6, the security protocol header appears after the base IP header and selected extension headers, but may appear before or after destination options; it MUST appear before next layer protocols (e.g., TCP, UDP, Stream Control Transmission Protocol (SCTP)). (§4.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the ESP state and the kernel output path builds every header, so no value Ze writes decides where the ESP header sits |
| [`RFC4301-4.4.1-5`](#rfc4301-4.4.1-5) However, if an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing. (§4.4.1) | {gap}, no test | Ze holds one SPD and no selection function; plan/spec-ipsec-spd-selection.md |
| [`RFC4301-4.4.1-8`](#rfc4301-4.4.1-8) (The means of signaling such requests to the IPsec implementation are outside the scope of this standard.) However, the system administrator MUST be able to specify whether or not a user or application can override (default) system policies. (§4.4.1) | {gap}, no test | no user or application override control exists in the SPD model; plan/spec-ipsec-spd-selection.md |
| [`RFC4301-4.4.1.1-2`](#rfc4301-4.4.1.1-2) Note that the Local and Remote ports may not be available in the case of receipt of a fragmented packet or if the port fields have been protected by IPsec (encrypted); thus, a value of OPAQUE also MUST be supported. (§4.4.1.1) | {gap}, no test | the operator SPD list refuses OPAQUE and the XFRM selector cannot express port 0 mask 0xffff; plan/spec-ipsec-opaque-selector-port-mask.md |
| [`RFC4301-4.4.1.1-4`](#rfc4301-4.4.1.1-4) Given a policy entry with a range of Types (T-start to T-end) and a range of Codes (C-start to C-end), and an ICMP packet with Type t and Code c, an implementation MUST test for a match using (T-start*256) + C-start <= (t*256) + c <= (T-end*256) + C-end (§4.4.1.1) | {gap}, no test | SPParams carries no ICMP type or code selector; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-4.4.1.1-5`](#rfc4301-4.4.1.1-5) The name used to match this field is communicated during the IKE negotiation in the ID payload. In this context, the initiator's Source IP address (inner IP header in tunnel mode) is bound to the Remote IP address in the SAD entry created by the IKE negotiation. This address overrides the Remote IP address value in the SPD, when the SPD entry is selected in this fashion. All IPsec implementations MUST support this use of names. (§4.4.1.1) | {gap}, no test | no name-valued selector selects an operator SPD entry during IKE; plan/spec-ipsec-spd-selection.md |
| [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2) A compliant implementation MUST support both types of lifetimes, and MUST support a simultaneous use of both. (§4.4.2.1) | {gap}, no test | newLifetimeState sets a time lifetime only and never a byte count; plan/spec-ipsec-lifetime-volume.md |
| [`RFC4301-4.4.3.3-1`](#rfc4301-4.4.3.3-1) (A peer may be authorized for both address types, so there MUST be provision for both a v4 and a v6 address range.) (§4.4.3.3) | {gap}, no test | one remote-id string holds one address range, not a v4 and a v6 range; plan/spec-ipsec-pad-dual-family-range.md |
| [`RFC4301-5.1-3`](#rfc4301-5.1-3) With regard to determining and enforcing the PMTU of an SA, the IPsec system MUST follow the steps described in Section 8.2 (§5.1) | {gap}, no test | no per-SA PMTU is held or aged; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1) If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain (§5.1.2.1) | {gap}, no test | the outer-header DSCP is not mapped for the domain entered; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-6-1`](#rfc4301-6-1) Disposition of non-error, ICMP messages (that are not addressed to the IPsec implementation itself) MUST be explicitly accounted for using SPD entries. (§6) | {gap}, no test | no ICMP type or code selector and no default entry accounts for non-error ICMP; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-6.1.1-1`](#rfc4301-6.1.1-1) To accommodate both ends of this spectrum, a compliant IPsec implementation MUST permit a local administrator to configure an IPsec implementation to accept or reject unauthenticated ICMP traffic. (§6.1.1) | {gap}, no test | no control accepts or rejects unauthenticated ICMP; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-6.1.1-2`](#rfc4301-6.1.1-2) This control MUST be at the granularity of ICMP type and MAY be at the granularity of ICMP type and code. (§6.1.1) | {gap}, no test | no ICMP-type granularity exists for the control; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-6.1.2-1`](#rfc4301-6.1.2-1) Thus, implementers MUST provide controls to allow local administrators to constrain the processing of ICMP error messages received on the protected side of the boundary, and directed to the IPsec implementation. (§6.1.2) | {gap}, no test | no control constrains ICMP error processing at the IPsec boundary; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-6.2-1`](#rfc4301-6.2-1) Thus, an IPsec implementation MUST be configurable to check that this payload header information is consistent with the SA via which it arrives. (§6.2) | {gap}, no test | no check compares an ICMP error payload header against the SA selectors; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-6.2-2`](#rfc4301-6.2-2) IPsec senders and receivers MUST support the following processing for ICMP error messages that are sent and received via SAs. (§6.2) | {gap}, no test | the Section 6.2 ICMP error processing is absent; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-6.2-3`](#rfc4301-6.2-3) If no SA exists that would carry the outbound ICMP message in question, and if no SPD entry would allow carriage of this outbound ICMP error message, then an IPsec implementation MUST map the message to the SA that would carry the return traffic associated with the packet that triggered the ICMP error message. (§6.2) | {gap}, no test | whether the kernel maps an outbound ICMP error to the return SA is unverified and Ze holds no producer for it; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-6.2-4`](#rfc4301-6.2-4) If an IPsec implementation receives an inbound ICMP error message on an SA, and the IP and ICMP headers of the message do not match the traffic selectors for the SA, the receiver MUST process the received message in a special fashion. Specifically, the receiver must extract the header of the triggering packet from the ICMP payload, and reverse fields as described above to determine if the packet is consistent with the selectors for the SA via which the ICMP error message was received. (§6.2) | {gap}, no test | no special processing of a mismatching inbound ICMP error exists; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-6.2-5`](#rfc4301-6.2-5) If the packet fails this check, the IPsec implementation MUST NOT forwarded the ICMP message to the destination. (§6.2) | {gap}, no test | no payload check exists, so nothing refuses to forward on its failure; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-7.2-1`](#rfc4301-7.2-1) Receivers MUST perform a minimum offset check on IPv4 (non-initial) fragments to protect against overlapping fragment attacks when SAs of this type are employed. (§7.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/engine/child.go::childPolicyParams builds one policy pair per Child SA and no separate non-initial-fragment SA, so approach #2 of Section 7.2 is never taken and no value Ze installs decides the offset check the kernel reassembly performs |
| [`RFC4301-7.2-2`](#rfc4301-7.2-2) Specific port (or ICMP type/code or Mobility Header type) selector values will be used to define SAs to carry initial fragments and non-fragmented packets. This approach can be used if a user or administrator wants to create one or more tunnel mode SAs between the same Local/Remote addresses that discriminate based on port (or ICMP type/code or Mobility Header type) fields. These SAs MUST have non-trivial protocol selector values, otherwise approach #1 above MUST be used. (§7.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/engine/child.go::childPolicyParams builds no fragment-carrying SA, so the protocol selector this sentence constrains is never written by Ze |
| [`RFC4301-7.3-1`](#rfc4301-7.3-1) Implementations that will transmit non-initial fragments on a tunnel mode SA that makes use of non-trivial port (or ICMP type/code or MH type) selectors MUST notify a peer via the IKE NOTIFY NON_FIRST_FRAGMENTS_ALSO payload (§7.3) | {gap}, no test | Ze never sends NON_FIRST_FRAGMENTS_ALSO; plan/spec-ipsec-non-first-fragments-also.md |
| [`RFC4301-7.3-2`](#rfc4301-7.3-2) The peer MUST reject this proposal if it will not accept non-initial fragments in this context. (§7.3) | {gap}, no test | Ze never parses or answers NON_FIRST_FRAGMENTS_ALSO; plan/spec-ipsec-non-first-fragments-also.md |
| [`RFC4301-8.1-1`](#rfc4301-8.1-1) All IPsec implementations MUST support the option of copying the DF bit from an outbound packet to the tunnel mode header that it emits, when traffic is carried via a tunnel mode SA (§8.1) | {gap}, no test | no option copies the DF bit into the tunnel header per SA; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-8.1-2`](#rfc4301-8.1-2) This means that it MUST be possible to configure the implementation's treatment of the DF bit (set, clear, copy from inner header) for each SA. (§8.1) | {gap}, no test | the DF treatment of an SA is not configurable; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-8.2.1-1`](#rfc4301-8.2.1-1) When such traffic arrives, if the traffic would exceed the updated PMTU value the traffic MUST be handled as follows: Case 1: Original (cleartext) packet is IPv4 and has the DF bit set. The implementation SHOULD discard the packet and send a PMTU ICMP message. Case 2: Original (cleartext) packet is IPv4 and has the DF bit clear. The implementation SHOULD fragment (before or after encryption per its configuration) and then forward the fragments. It SHOULD NOT send a PMTU ICMP message. Case 3: Original (cleartext) packet is IPv6. The implementation SHOULD discard the packet and send a PMTU ICMP message. (§8.2.1) | {gap}, no test | no per-SA PMTU exists to exceed; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-8.2.2-1`](#rfc4301-8.2.2-1) In all IPsec implementations, the PMTU associated with an SA MUST be "aged" and some mechanism is required to update the PMTU in a timely manner (§8.2.2) | {gap}, no test | no per-SA PMTU is held or aged; plan/immediate/spec-rfc4301-architecture-gaps.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4301-4.1-1`](#rfc4301-4.1-1)

a) A host implementation of IPsec MUST support both transport and tunnel mode. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IKE Child SA install asserts Mode==tunnel on both SAs and the OSPFv3 installer asserts Mode==transport on every SA, so both modes are proven; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L169) | unit/verify | unproven |
| positive | [`TestIPsecSAPerDestinationWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L226) | unit/verify | revert, verified |

### [`RFC4301-4.1-2`](#rfc4301-4.1-2)

A security gateway MUST support tunnel mode (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestChildSAInstallsInDataplane asserts every installed policy is tunnel mode; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L184) | unit/verify | unproven |

### [`RFC4301-4.1-3`](#rfc4301-4.1-3)

Aside from the two exceptions below, whenever either end of a security association is a security gateway, the SA MUST be tunnel mode. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. every IKE peer SA is asserted modeTunnel; OSPFv3 transport SAs fall under the security-gateway-acting-as-host exception; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L171) | unit/verify | unproven |

### [`RFC4301-4.1-4`](#rfc4301-4.1-4)

IKE creates pairs of SAs, so for simplicity, we choose to require that both SAs in a pair be of the same mode, transport or tunnel. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. inbound and outbound SA of one IKE pair both asserted modeTunnel, so the pair is same-mode; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L173) | unit/verify | unproven |

### [`RFC4301-4.1-5`](#rfc4301-4.1-5)

To permit this, the IPsec implementation MUST permit establishment and maintenance of multiple SAs between a given sender and receiver, with the same selectors. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-5, so no unit is bound to it.

### [`RFC4301-4.2-1`](#rfc4301-4.2-1)

Note: A compliant implementation MUST NOT allow instantiation of an ESP SA that employs both NULL encryption and no integrity algorithm. (§4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: instantiating any ESP SA with NULL encryption and no integrity. OSPFv3 manual path: NULL encryption without integrity is refused with ErrIPsecAuthAlgo and NULL with integrity accepted (both polarities). The IKE-negotiated ESP path, which instantiates most of Ze's ESP SAs, has no assertion in the tagged unit: no case offers or accepts an ENCR_NULL proposal without an integrity transform, and the row carries no marker bounding the clause to the manual path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L102) | unit/verify | unproven |
| positive | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L124) | unit/verify | unproven |

### [`RFC4301-4.4.1-1`](#rfc4301-4.4.1-1)

An IPsec implementation MUST have at least one SPD, and it MAY support multiple SPDs, if appropriate for the context in which the IPsec implementation operates. (§4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Child SA install asserts two PROTECT policies reach the SPD; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L150) | unit/verify | unproven |

### [`RFC4301-4.4.1-2`](#rfc4301-4.4.1-2)

The SPD, or relevant caches, must be consulted during the processing of all traffic (inbound and outbound), including traffic not protected by IPsec, that traverses the IPsec boundary. This includes IPsec management traffic such as IKE. (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-2, so no unit is bound to it.

### [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1)

The following selector parameters MUST be supported by all IPsec implementations to facilitate control of SA granularity. (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-1, so no unit is bound to it.

### [`RFC4301-4.4.2-1`](#rfc4301-4.4.2-1)

For each of the selectors defined in Section 4.4.1.1, the entry for an inbound SA in the SAD MUST be initially populated with the value or values negotiated at the time the SA was created. (§4.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clause: for EACH Section 4.4.1.1 selector, the inbound SAD entry is populated with the negotiated values. TestChildSAInboundPolicyUsesNegotiatedTS and TestNarrowedSelectorsReachTheInstalledPolicy assert the inbound SPD policy's Src/Dst equal the narrowed TSr/TSi: that is the SPD entry, not the SAD entry (the XFRM state's selector is never read), and the Next Layer protocol and port selectors are not asserted at all, so a SAD entry carrying unnegotiated protocol or ports stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L412) | unit/verify | unproven |
| positive | [`TestNarrowedSelectorsReachTheInstalledPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L423) | unit/verify | unproven |

### [`RFC4301-4.5-1`](#rfc4301-4.5-1)

All IPsec implementations MUST support both manual and automated SA and cryptographic key management. (§4.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. automated half: createFirstChildSA installs IKE-keyed SAs; manual half: OSPFv3 installs static SPI+key SAs; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L144) | unit/verify | unproven |
| positive | [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L110) | unit/verify | mutant, verified |

### [`RFC4301-5.2-1`](#rfc4301-5.2-1)

Since the SPD-I is just a part of the SPD, if a packet that is looked up in the SPD-I cannot be matched to an entry there, then the packet MUST be discarded. (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.2-1, so no unit is bound to it.

### [`RFC4301-5.2-2`](#rfc4301-5.2-2)

Then match the packet against the inbound selectors identified by the SAD entry to verify that the received packet is appropriate for the SA via which it was received. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.2-2, so no unit is bound to it.

### [`RFC4301-4.1-6`](#rfc4301-4.1-6)

If an IPsec implementation supports multicast, then it MUST support multicast SAs using the algorithm below for mapping inbound IPsec datagrams to SAs. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-6, so no unit is bound to it.

### [`RFC4301-7-1`](#rfc4301-7-1)

Note: AH and ESP cannot be applied using transport mode to IPv4 packets that are fragments. Only tunnel mode can be employed in such cases. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7-1, so no unit is bound to it.

### [`RFC4301-4.4.3.1-1`](#rfc4301-4.4.3.1-1)

The specific syntax used by an implementation to accommodate sub-tree matching for distinguished names, domain names or RFC 822 e-mail addresses is a local matter. But, at a minimum, sub-tree matching of the sort described above MUST be supported. (§4.4.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. sub-tree entries admit DNS, RFC 822 and DN peers beneath them and refuse near misses, apex and outside peers; certificate binding kept

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPadKeyIDStaysExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L111) | unit/verify | unproven |
| negative | [`TestPadSubtreeRefusesAPeerOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L69) | unit/verify | unproven |
| positive | [`TestPadSubtreeAdmitsAPeerBeneathIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L35) | unit/verify | unproven |
| positive | [`TestPadSubtreeStillBindsTheCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L185) | unit/verify | unproven |
| positive | [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/validate_identity_test.go#L166) | unit/verify | unproven |

### [`RFC4301-4.4.3.1-2`](#rfc4301-4.4.3.1-2)

For IPv4 and IPv6 addresses, the same address range syntax used for SPD entries MUST be supported. (§4.4.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. prefix PAD entries admit addresses inside for v4 and v6 and refuse outside, other family and text spellings; the same prefix syntax the SPD uses

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPadAddressRangeRefusesWhatIsOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L155) | unit/verify | unproven |
| positive | [`TestPadAddressRangeAdmitsAnAddressInsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L129) | unit/verify | unproven |
| positive | [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/validate_identity_test.go#L167) | unit/verify | unproven |

### [`RFC4301-4.4.1-4`](#rfc4301-4.4.1-4)

Thus, a user or administrator MUST be able to order the entries to express a desired access control policy. (§4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestSpdOperatorOrdersOverlappingPeers asserts the operator's priority reaches every entry in both directions and orders overlapping peers; negatives refuse rank 0 and ranks capturing IKE. TestSPDPolicyMirrorsTheInboundSelector is mistagged: it proves direction mirroring, not ordering

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

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L194) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L131) | unit/verify | revert, verified |

### [`RFC4301-3.2-1`](#rfc4301-3.2-1)

IPsec implementations MUST support ESP and MAY support AH (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4301ChildSAIsInstalledAsESP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_esp_test.go#L18) | unit/verify | revert, verified |

### [`RFC4301-4-1`](#rfc4301-4-1)

All implementations of AH or ESP MUST support the concept of an SA as described below (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L113) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L69) | unit/verify | revert, verified |

### [`RFC4301-4.1-8`](#rfc4301-4.1-8)

The indication of whether source and destination address matching is required to map inbound IPsec traffic to SAs MUST be set either as a side effect of manual SA configuration or via negotiation using an SA management protocol, e.g., IKE or Group Domain of Interpretation (GDOI) [RFC3547]. (§4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive only shows the SAD entry copies src and dst; the negative (unknown mode refused) violates a different rule; neither asserts how the address-matching indication is set by manual configuration or negotiation

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L70) | unit/verify | revert, verified |

### [`RFC4301-4.1-9`](#rfc4301-4.1-9)

The receiver MUST process the packets from the different SAs without prejudice. (§4.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The sentence: the receiver processes packets from parallel SAs (same selectors, different QoS/DSCP) without prejudice. The positive asserts two SAs with the same selector and different SPIs build two distinct SAD entries, which is exactly the neighbouring 'MUST permit establishment and maintenance of multiple SAs ... with the same selectors' (RFC4301-4.1-5); the negative asserts an unbuildable SA is refused. No unit receives packets on either SA, so the tags attribute the neighbouring obligation's evidence to this row. Re-judged after the blind reader's weak: nothing in either unit is a partial proof of unprejudiced processing.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L115) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L71) | unit/verify | revert, verified |

### [`RFC4301-4.1-10`](#rfc4301-4.1-10)

In transport mode, the DSCP value might change en route, but this should not cause problems with respect to IPsec processing since the value is not employed for SA selection and MUST NOT be checked as part of SA/packet validation. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-10, so no unit is bound to it.

### [`RFC4301-4.1-11`](#rfc4301-4.1-11)

In IPv6, the security protocol header appears after the base IP header and selected extension headers, but may appear before or after destination options; it MUST appear before next layer protocols (e.g., TCP, UDP, Stream Control Transmission Protocol (SCTP)). (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-11, so no unit is bound to it.

### [`RFC4301-4.4-1`](#rfc4301-4.4-1)

The model described below is nominal; implementations need not match details of this model as presented, but the external behavior of implementations MUST correspond to the externally observable characteristics of this model in order to be compliant. (§4.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. one policy-builder unit cannot prove whole-model conformance; positive asserts the fields of one SPD entry, negative a selector-expressibility refusal

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L195) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L132) | unit/verify | revert, verified |

### [`RFC4301-4.4.1-5`](#rfc4301-4.4.1-5)

However, if an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing. (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-5, so no unit is bound to it.

### [`RFC4301-4.4.1-6`](#rfc4301-4.4.1-6)

The SPD MUST permit a user or administrator to specify policy entries as follows: (§4.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SPD-I and SPD-O BYPASS/DISCARD entries asserted configurable, unknown dispositions refused; the SPD-S (PROTECT) clause of the sentence is proved by no tagged unit

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SPDRefusesADispositionItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestRFC4301SPDPermitsEachDispositionInEachDatabase`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L35) | unit/verify | revert, verified |

### [`RFC4301-4.4.1-7`](#rfc4301-4.4.1-7)

For every IPsec implementation, there MUST be a management interface that allows a user or system administrator to manage the SPD (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SPDInterfaceRefusesWhatItCannotProgram`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L143) | unit/verify | revert, verified |
| positive | [`TestRFC4301SPDIsManagedThroughTheConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L100) | unit/verify | revert, verified |

### [`RFC4301-4.4.1-8`](#rfc4301-4.4.1-8)

(The means of signaling such requests to the IPsec implementation are outside the scope of this standard.) However, the system administrator MUST be able to specify whether or not a user or application can override (default) system policies. (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-8, so no unit is bound to it.

### [`RFC4301-4.4.1-9`](#rfc4301-4.4.1-9)

Note that when an SPD entry is decorrelated all the resulting entries MUST be linked together, so that all members of the group derived from an individual, SPD entry (prior to decorrelation) can all be placed into caches and into the SAD at the same time. (§4.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. obligation is conditional on decorrelation, which Ze does not perform; the units assert one entry builds one policy and a refused entry builds none, which proves the condition absent rather than the linking

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L196) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L133) | unit/verify | revert, verified |

### [`RFC4301-4.4.1-10`](#rfc4301-4.4.1-10)

If the SPD is not decorrelated, caching is not allowed and an ordered search of SPD MUST be performed to verify that inbound traffic arriving on an SA is consistent with the access control policy expressed in the SPD. (§4.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive asserts the priority only on the OUTBOUND entry, while the sentence is about the ordered search verifying INBOUND traffic; the inbound entry's priority is not asserted

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L197) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L134) | unit/verify | revert, verified |

### [`RFC4301-4.4.1.1-2`](#rfc4301-4.4.1.1-2)

Note that the Local and Remote ports may not be available in the case of receipt of a fragmented packet or if the port fields have been protected by IPsec (encrypted); thus, a value of OPAQUE also MUST be supported. (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-2, so no unit is bound to it.

### [`RFC4301-4.4.1.1-3`](#rfc4301-4.4.1.1-3)

If the SA requires a port value other than ANY or OPAQUE, an arriving fragment without ports MUST be discarded (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L198) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L135) | unit/verify | revert, verified |

### [`RFC4301-4.4.1.1-4`](#rfc4301-4.4.1.1-4)

Given a policy entry with a range of Types (T-start to T-end) and a range of Codes (C-start to C-end), and an ICMP packet with Type t and Code c, an implementation MUST test for a match using (T-start*256) + C-start <= (t*256) + c <= (T-end*256) + C-end (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-4, so no unit is bound to it.

### [`RFC4301-4.4.1.1-5`](#rfc4301-4.4.1.1-5)

The name used to match this field is communicated during the IKE negotiation in the ID payload. In this context, the initiator's Source IP address (inner IP header in tunnel mode) is bound to the Remote IP address in the SAD entry created by the IKE negotiation. This address overrides the Remote IP address value in the SPD, when the SPD entry is selected in this fashion. All IPsec implementations MUST support this use of names. (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-5, so no unit is bound to it.

### [`RFC4301-4.4.1.2-1`](#rfc4301-4.4.1.2-1)

Until IKE provides a facility that conveys the semantics that are expressed in the SPD via selector sets (as described below), users MUST NOT include multiple selector sets in a single SPD entry unless the access control intent aligns with the IKE "mix and match" semantics. (§4.4.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts one selector set parsed from a one-set input; would still pass if the model grew a list of selector sets, so it cannot fail on non-compliance

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4301SPDEntryHoldsOneSelectorSet`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_selector_set_test.go#L10) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-1`](#rfc4301-4.4.2.1-1)

The following data items MUST be in the SAD: (§4.4.2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts SPI, mode, protocol, transforms, keys and replay window; the sequence counter, overflow flag, lifetime, PMTU and the DF/DSCP bypass items of the list are not asserted

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L72) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2)

A compliant implementation MUST support both types of lifetimes, and MUST support a simultaneous use of both. (§4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.2.1-2, so no unit is bound to it.

### [`RFC4301-4.4.2.1-3`](#rfc4301-4.4.2.1-3)

Note that implementations MUST be able to handle having the counters at the ends of an SA get out of synch, e.g., because of packet loss or because the implementations at each end of the SA aren't doing things the same way. (§4.4.2.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. the sentence concerns byte-count LIFETIME counters at the two ends getting out of synch (4.4.2.1 lifetime item (a)); the units assert the anti-replay window, a different mechanism

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L73) | unit/verify | revert, verified |

### [`RFC4301-4.4.3.1-3`](#rfc4301-4.4.3.1-3)

For this name type, only exact-match syntax MUST be supported (since there is no explicit structure for this ID type). (§4.4.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. ID_KEY_ID admitted only on octet-exact equality; prefix, suffix, case, trailing dot and sub-tree variants refused

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301PadOtherIDTypeRefusesEverythingButExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC4301PadOtherIDTypeAdmitsAnExactMatch`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L18) | unit/verify | revert, verified |

### [`RFC4301-4.4.3.2-1`](#rfc4301-4.4.3.2-1)

Thus, implementations MUST provide a means for an administrator to require a match between an asserted IKE ID and the subject name or subject alt name in a certificate. (§4.4.3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. with remote-id set, a certificate carrying the asserted ID (SAN, or CN without SAN) is admitted and one issued to another name is refused naming the asserted ID

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301PadRefusesACertificateWithoutTheIKEID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestRFC4301PadRequiresTheCertificateToCarryTheIKEID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L66) | unit/verify | revert, verified |

### [`RFC4301-4.4.3.3-1`](#rfc4301-4.4.3.3-1)

(A peer may be authorized for both address types, so there MUST be provision for both a v4 and a v6 address range.) (§4.4.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.3.3-1, so no unit is bound to it.

### [`RFC4301-4.5.2-1`](#rfc4301-4.5.2-1)

To ensure that the IPsec implementations at each end of the SA use the same bits for the same keys, and irrespective of which part of the system divides the string of bits into individual keys, the encryption keys MUST be taken from the first (left-most, high-order) bits and the integrity keys MUST be taken from the remaining bits. (§4.5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. encryption key asserted as the leading octets and integrity key the following octets per SA, and the swapped split asserted absent

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301KeymatIntegrityKeysNeverLead`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4301_keymat_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestRFC4301KeymatEncryptionKeysLeadEachSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4301_keymat_test.go#L45) | unit/verify | revert, verified |

### [`RFC4301-4.5.3-1`](#rfc4301-4.5.3-1)

To address these problems, an IPsec-supporting host or security gateway MUST have an administrative interface that allows the user/administrator to configure the address of one or more security gateways for ranges of destination addresses that require its use. (§4.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. gateway address and destination ranges parsed as written; malformed range refused naming peer and range

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301GatewayRefusesAMalformedDestinationRange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_gateway_test.go#L65) | unit/verify | revert, verified |
| positive | [`TestRFC4301GatewayIsConfiguredForDestinationRanges`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_gateway_test.go#L39) | unit/verify | revert, verified |

### [`RFC4301-5-1`](#rfc4301-5-1)

If no policy is found in the SPD that matches a packet (for either inbound or outbound traffic), the packet MUST be discarded (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301UnmatchedNeverDiscardsWhatTheOperatorDidNotAskToDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_unmatched_test.go#L80) | unit/verify | revert, verified |
| negative | [`TestRFC4301UnmatchedLeafRefusesAnUnknownWord`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_unmatched_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestRFC4301UnmatchedDiscardIsTheLastEntryOfEveryDatabase`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_unmatched_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestRFC4301UnmatchedLeafIsReadAsWritten`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_unmatched_test.go#L23) | unit/verify | revert, verified |

### [`RFC4301-5.1-1`](#rfc4301-5.1-1)

IPsec MUST perform the following steps when processing outbound packets: (§5.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quote obliges IPsec to perform the Section 5.1 outbound steps (a list the quote points to). The kernel performs them; the tagged units assert one outbound SPD entry Ze installs (out dir, selector, ESP tunnel template, priority) and the refusal of a partial port mask. No assertion ties to the individual steps (cache lookup, SPD-S/O/I selection, DISCARD, the SA-less PROTECT case), so a Ze install that broke any step other than this entry's shape stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L136) | unit/verify | revert, verified |

### [`RFC4301-5.1-2`](#rfc4301-5.1-2)

If so, there MUST be an entry in SPD-I database that permits inbound bypassing of the packet, otherwise the packet will be discarded. (§5.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. the sentence is §5.1 step 4: a packet the forwarding function passes back across the boundary (nested SAs) needs an SPD-I bypass entry; the units assert the return flow of a both-direction bypass gets a mirrored SPD-I entry, a different obligation

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BypassSPDIEntryIsNeverAnUnmirroredCopy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC4301BypassReturnTrafficHasAnSPDIEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L48) | unit/verify | revert, verified |

### [`RFC4301-5.1-3`](#rfc4301-5.1-3)

With regard to determining and enforcing the PMTU of an SA, the IPsec system MUST follow the steps described in Section 8.2 (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.1-3, so no unit is bound to it.

### [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1)

If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain (§5.1.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.1.2.1-1, so no unit is bound to it.

### [`RFC4301-5.2-3`](#rfc4301-5.2-3)

IPsec MUST perform the following steps: (§5.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quote obliges IPsec to perform the Section 5.2 inbound steps (a list the quote points to). The tagged units assert one inbound SPD entry Ze installs (in dir, same selector, template) and the refusal of a partial port mask; no assertion ties to the individual steps (SPI demux to the SAD, the SPD-I bypass/discard lookup, the post-decapsulation selector check), so the steps are not proven by what Ze installs here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L200) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L137) | unit/verify | revert, verified |

### [`RFC4301-5.2-4`](#rfc4301-5.2-4)

IKE traffic MUST have an explicit BYPASS entry in the SPD (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301IKEBypassIsNeverAProtectOrAWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ike_bypass_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC4301IKETrafficHasAnExplicitBypassEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ike_bypass_test.go#L21) | unit/verify | revert, verified |

### [`RFC4301-5.2-5`](#rfc4301-5.2-5)

If so, then as with ALL outbound traffic that is to be bypassed, the packet MUST be matched against an SPD-O entry. (§5.2)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The sentence (§5.2): an inbound packet that the forwarding function sends back out across the boundary (nested SAs) MUST be matched against an SPD-O entry, the entry matching THAT packet's orientation. The units assert that a both-direction bypass builds a mirrored SPD-O entry for the reply flow (src 10.1/24 -> 10.2/24, src port 22) and explicitly assert it does NOT carry the inbound orientation, and that an in-only bypass gets no SPD-O entry. That is the bidirectional-bypass mirroring rule, not the re-crossing match the sentence states. Re-judged after the blind reader's enforced: the SPD-O entry proven is for a different packet.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BypassSPDOEntryIsNeverAnUnmirroredCopy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestRFC4301InboundBypassReturnTrafficHasAnSPDOEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L88) | unit/verify | revert, verified |

### [`RFC4301-6-1`](#rfc4301-6-1)

Disposition of non-error, ICMP messages (that are not addressed to the IPsec implementation itself) MUST be explicitly accounted for using SPD entries. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6-1, so no unit is bound to it.

### [`RFC4301-6.1.1-1`](#rfc4301-6.1.1-1)

To accommodate both ends of this spectrum, a compliant IPsec implementation MUST permit a local administrator to configure an IPsec implementation to accept or reject unauthenticated ICMP traffic. (§6.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.1.1-1, so no unit is bound to it.

### [`RFC4301-6.1.1-2`](#rfc4301-6.1.1-2)

This control MUST be at the granularity of ICMP type and MAY be at the granularity of ICMP type and code. (§6.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.1.1-2, so no unit is bound to it.

### [`RFC4301-6.1.2-1`](#rfc4301-6.1.2-1)

Thus, implementers MUST provide controls to allow local administrators to constrain the processing of ICMP error messages received on the protected side of the boundary, and directed to the IPsec implementation. (§6.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.1.2-1, so no unit is bound to it.

### [`RFC4301-6.2-1`](#rfc4301-6.2-1)

Thus, an IPsec implementation MUST be configurable to check that this payload header information is consistent with the SA via which it arrives. (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.2-1, so no unit is bound to it.

### [`RFC4301-6.2-2`](#rfc4301-6.2-2)

IPsec senders and receivers MUST support the following processing for ICMP error messages that are sent and received via SAs. (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.2-2, so no unit is bound to it.

### [`RFC4301-6.2-3`](#rfc4301-6.2-3)

If no SA exists that would carry the outbound ICMP message in question, and if no SPD entry would allow carriage of this outbound ICMP error message, then an IPsec implementation MUST map the message to the SA that would carry the return traffic associated with the packet that triggered the ICMP error message. (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.2-3, so no unit is bound to it.

### [`RFC4301-6.2-4`](#rfc4301-6.2-4)

If an IPsec implementation receives an inbound ICMP error message on an SA, and the IP and ICMP headers of the message do not match the traffic selectors for the SA, the receiver MUST process the received message in a special fashion. Specifically, the receiver must extract the header of the triggering packet from the ICMP payload, and reverse fields as described above to determine if the packet is consistent with the selectors for the SA via which the ICMP error message was received. (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.2-4, so no unit is bound to it.

### [`RFC4301-6.2-5`](#rfc4301-6.2-5)

If the packet fails this check, the IPsec implementation MUST NOT forwarded the ICMP message to the destination. (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-6.2-5, so no unit is bound to it.

### [`RFC4301-7.1-1`](#rfc4301-7.1-1)

All implementations MUST support tunnel mode SAs that are configured to pass traffic without regard to port field (or ICMP type/code or Mobility Header type) values (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301PortScopedSAIsNeverInstalledAsAnyPort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_portless_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestRFC4301TunnelSAPassesTrafficWithoutRegardToPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_portless_test.go#L47) | unit/verify | revert, verified |

### [`RFC4301-7.1-2`](#rfc4301-7.1-2)

If the SA will carry traffic for specified protocols, the selector set for the SA MUST specify the port fields (or ICMP type/code or Mobility Header type) as ANY. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a protocol-scoped SA carrying traffic without regard to ports whose installed selector constrains a port. TestRFC4301ProtocolScopedSASpecifiesPortsAsAny asserts, in both directions, UpperProto == 17 and SrcPort.IsAny() && DstPort.IsAny() on childPolicyParams output, red if either port is narrowed. TestRFC4301AnyPortFormNeverLeaksAPortNumber asserts no ExactPortMatch(8080/9090/0) under the ANY form, red if a stray Port value or a zero reaches the policy.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301AnyPortFormNeverLeaksAPortNumber`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_portless_test.go#L123) | unit/verify | revert, verified |
| positive | [`TestRFC4301ProtocolScopedSASpecifiesPortsAsAny`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_portless_test.go#L99) | unit/verify | revert, verified |

### [`RFC4301-7.1-3`](#rfc4301-7.1-3)

If the SA will carry traffic without regard to a specific protocol value (i.e., ANY is specified as the (Next Layer) protocol selector value), then the port field values are undefined and MUST be set to ANY as well. (§7.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. Forbidden: an SA whose Next Layer protocol selector is ANY installed with a port field other than ANY. Both tagged units (TestRFC4301ProtocolAnyEntryCarriesAnyPorts, TestRFC4301ProtocolAnyEntryRefusesAPort) drive ParseIPsecConfig and ValidateSPDPolicies over bypass SPD entries, which create no SA; they prove the neighbouring SPD-entry validation. No tagged unit builds a Child SA with protocol ANY through childPolicyParams and asserts both ports ANY, nor refuses a port selector under protocol ANY on the SA path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301ProtocolAnyEntryRefusesAPort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestRFC4301ProtocolAnyEntryCarriesAnyPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L178) | unit/verify | revert, verified |

### [`RFC4301-7.2-1`](#rfc4301-7.2-1)

Receivers MUST perform a minimum offset check on IPv4 (non-initial) fragments to protect against overlapping fragment attacks when SAs of this type are employed. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.2-1, so no unit is bound to it.

### [`RFC4301-7.2-2`](#rfc4301-7.2-2)

Specific port (or ICMP type/code or Mobility Header type) selector values will be used to define SAs to carry initial fragments and non-fragmented packets. This approach can be used if a user or administrator wants to create one or more tunnel mode SAs between the same Local/Remote addresses that discriminate based on port (or ICMP type/code or Mobility Header type) fields. These SAs MUST have non-trivial protocol selector values, otherwise approach #1 above MUST be used. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.2-2, so no unit is bound to it.

### [`RFC4301-7.3-1`](#rfc4301-7.3-1)

Implementations that will transmit non-initial fragments on a tunnel mode SA that makes use of non-trivial port (or ICMP type/code or MH type) selectors MUST notify a peer via the IKE NOTIFY NON_FIRST_FRAGMENTS_ALSO payload (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.3-1, so no unit is bound to it.

### [`RFC4301-7.3-2`](#rfc4301-7.3-2)

The peer MUST reject this proposal if it will not accept non-initial fragments in this context. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7.3-2, so no unit is bound to it.

### [`RFC4301-7.3-3`](#rfc4301-7.3-3)

If an implementation does not successfully negotiate transmission of non-initial fragments for such an SA, it MUST NOT send such fragments over the SA (§7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L138) | unit/verify | revert, verified |

### [`RFC4301-7.3-4`](#rfc4301-7.3-4)

However, a receiver MUST discard non-initial fragments that arrive on an SA with non-trivial port (or ICMP type/code or MH type) selector values unless this feature has been negotiated. (§7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a non-initial fragment on an SA with non-trivial port selectors (Ze has no NON_FIRST_FRAGMENTS_ALSO negotiation). Ze's boundary is the inbound XFRM policy the kernel checks; Linux decodes no ports for a non-initial fragment, so an exact port selector refuses it. TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate asserts in.SrcPort == 443 on the inbound entry, red if the selector is widened to ANY. TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened asserts a partial mask (0x00ff) is refused for SADirIn and SADirOut, red if it is installed widened.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L202) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L139) | unit/verify | revert, verified |

### [`RFC4301-7.3-5`](#rfc4301-7.3-5)

Also, the receiver MUST discard non-initial fragments that do not comply with the security policy applied to the overall packet. (§7.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: accepting a non-initial fragment that does not comply with the policy applied to the overall packet. The positive asserts only that the inbound entry carries the same selector as the outbound and exactly one template (len(in.Tmpls) == 1); no assertion goes red on a fragment of a non-compliant packet being accepted. The negative asserts that a bypass entry carries no template, a neighbouring property unrelated to fragment compliance.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L203) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L140) | unit/verify | revert, verified |

### [`RFC4301-8.1-1`](#rfc4301-8.1-1)

All IPsec implementations MUST support the option of copying the DF bit from an outbound packet to the tunnel mode header that it emits, when traffic is carried via a tunnel mode SA (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-8.1-1, so no unit is bound to it.

### [`RFC4301-8.1-2`](#rfc4301-8.1-2)

This means that it MUST be possible to configure the implementation's treatment of the DF bit (set, clear, copy from inner header) for each SA. (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-8.1-2, so no unit is bound to it.

### [`RFC4301-8.2.1-1`](#rfc4301-8.2.1-1)

When such traffic arrives, if the traffic would exceed the updated PMTU value the traffic MUST be handled as follows: Case 1: Original (cleartext) packet is IPv4 and has the DF bit set. The implementation SHOULD discard the packet and send a PMTU ICMP message. Case 2: Original (cleartext) packet is IPv4 and has the DF bit clear. The implementation SHOULD fragment (before or after encryption per its configuration) and then forward the fragments. It SHOULD NOT send a PMTU ICMP message. Case 3: Original (cleartext) packet is IPv6. The implementation SHOULD discard the packet and send a PMTU ICMP message. (§8.2.1)

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
| Signed off | 2026-09-27 |
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
| `A` | Appendix A | 0 | walked | Appendix A. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B` | Appendix B | 0 | walked | Appendix B. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B.1` | not stated | 0 | skipped (appendix-non-normative) | Appendix B is the decorrelation algorithm, presented as an example implementation aid |
| `C` | Appendix C | 0 | walked | Appendix C. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `D` | Appendix D | 0 | walked | Appendix D. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `D.1` | not stated | 0 | skipped (appendix-non-normative) | Appendix D is 'Fragment Handling Rationale'; D.1 states the problem and imposes nothing |
| `D.2` | not stated | 2 | walked | not stated |
| `D.3` | fragment-handling rationale | 0 | skipped (appendix-non-normative) | fragment-handling rationale |
| `D.4` | not stated | 2 | walked | not stated |
| `D.5` | fragment-handling rationale | 0 | skipped (appendix-non-normative) | fragment-handling rationale |
| `D.6` | fragment-handling rationale | 0 | skipped (appendix-non-normative) | fragment-handling rationale |
| `D.7` | fragment-handling rationale | 0 | skipped (appendix-non-normative) | fragment-handling rationale |
| `D.8` | not stated | 1 | walked | not stated |
| `E` | Appendix E | 0 | walked | Appendix E. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |

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
