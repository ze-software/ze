# RFC 4301 - Security Architecture for the Internet Protocol

Partial. Every requirement this repository extracted from RFC 4301, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 46.0% | 46 of 100 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.0% | 12 of 100 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 100 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 100 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 88.9% | 136 of 153 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 100 | of 103 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 100 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 3.0% | 3 of 100 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 4.0% | 4 of 100 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 1.0% | 1 of 100 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 34.0% | 34 of 100 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 52 | of 100 gated MUSTs judged | 6 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 100 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
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
| Requirements | 103 |
| Gated MUST-level | 100 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 35 |
| Declared gaps a test demonstrates | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 153 |
| Tagged units | 153 |
| Recorded audit verdicts | 52 |
| Discrimination records | 136 |
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
- **Section 8, DF bit and PMTU:** the DF treatment of a tunnel-mode SA is not configurable, and no per-SA PMTU value is held or aged. Section 4.4.2.1, SAD lifetimes: `newLifetimeState` ([`internal/component/ike/engine/rekey.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey.go)) assigns a time lifetime only, `softBytes` is never assigned, and the byte-count arm of `softExpired` is unreachable in a running daemon; [`plan/spec-ipsec-lifetime-volume.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-lifetime-volume.md) designs the byte-count lifetime. Section 5.1.2.1, outer header: the DSCP value of the outer tunnel header is not mapped for the domain the packet enters. Section 4.4.1.1, selectors: a port selector holds any port or one exact port rather than the range the section defines, and `SPParams` carries no ICMP type or code field. Each of those blocks is now gated by a checklist row of its own, added by the 2026-09-21 extraction walk: [`RFC4301-6.1.1-1`](#rfc4301-6.1.1-1) and [`RFC4301-6.1.1-2`](#rfc4301-6.1.1-2) for the unauthenticated-ICMP control, [`RFC4301-6.2-1`](#rfc4301-6.2-1) for the ICMP error payload check, [`RFC4301-8.1-1`](#rfc4301-8.1-1) and [`RFC4301-8.1-2`](#rfc4301-8.1-2) for the DF bit, [`RFC4301-8.2.2-1`](#rfc4301-8.2.2-1) for PMTU ageing, [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2) for the byte-count lifetime, and [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1) for the outer-header DSCP mapping. [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1) remains the gated selector row and its reason text is stale about ports. The same walk added 49 further rows, for obligations no earlier pass had recorded at all: the Section 3 connectivity and ESP-support rows, the Section 4.4.1 SPD management interface and decorrelation rows, the Section 4.4.1.1 OPAQUE and ICMP-range selector rows, the Section 4.4.2.1 SAD data items, the Section 4.4.3 PAD rows, the Section 4.5.2 key-splitting rule, the Section 5 processing-step rows, and the Section 7 fragment-handling rows. None of them carried a test on 2026-09-21. The ruling pass of that day implemented the Section 5 catch-all (`vpn ipsec unmatched`, [`RFC4301-5-1`](#rfc4301-5-1)), proved the Section 3.1, 4, 4.1, 4.4, 4.4.1, 4.4.1.1, 4.4.2.1, 5.1, 5.2 and 7.3 obligations the kernel performs on state Ze installs at the XFRM boundary, and scheduled every remaining MUST as a `{gap}` naming its spec: the Section 6 ICMP block, the Section 8 DF bit and PMTU block, the outer-header DSCP mapping and the ICMP type/code selector in [`plan/immediate/spec-rfc4301-architecture-gaps.md`](https://github.com/ze-software/ze/blob/main/plan/immediate/spec-rfc4301-architecture-gaps.md); the byte-count lifetime in [`plan/spec-ipsec-lifetime-volume.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-lifetime-volume.md); OPAQUE ports in the operator list in [`plan/spec-ipsec-opaque-selector-port-mask.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-opaque-selector-port-mask.md); multiple SPDs, the user override control and the name selector in [`plan/spec-ipsec-spd-selection.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-spd-selection.md); the dual-family PAD range in [`plan/spec-ipsec-pad-dual-family-range.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-pad-dual-family-range.md); and NON_FIRST_FRAGMENTS_ALSO in [`plan/spec-ipsec-non-first-fragments-also.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ipsec-non-first-fragments-also.md). Section 4.4.1, final SPD entry: a disclosed deviation, not counted conformant. RFC 4301 Section 4.4.1 says "Every SPD SHOULD have a nominal, final entry that matches anything that is otherwise unmatched, and discards it", and the `vpn ipsec unmatched` leaf defaults to bypass, so with the default the final entry Ze installs passes unmatched traffic in the clear. The default is kept by owner ruling because a discard default would stop the router's own control plane (BGP, SSH, DNS) the moment `vpn ipsec` was configured; `unmatched discard` gives the RFC behavior. [`RFC4301-4.4.1-3`](#rfc4301-4.4.1-3) carries it as a `{gap}` demonstrated by TestRFC4301DefaultFinalSPDEntryDiscards, and [`RFC4301-5.2-9`](#rfc4301-5.2-9), the Section 5.2 step 3b discard of an inbound packet no SPD-I entry matches, and [`RFC4301-5.2-1`](#rfc4301-5.2-1), the same discard as Section 4.4.1 states it, each carry a `{gap}` for the paths where no catch-all is installed at all, so an inbound packet meets no SPD-I entry: the VPP backend, which holds no entry bound to no interface, the time before the first apply and after the engine exits, and a bypass catch-all whose install failed. A discard catch-all that cannot be installed is refused at commit or fails the apply. Section 4.4.1.2, multiple selector sets: an SPD entry may hold "One to N selector sets", and Ze offers one per entry (parseSPDPolicy); the multiple-set entry is an optional feature Ze declined, so [`RFC4301-4.4.1.2-2`](#rfc4301-4.4.1.2-2), the MUST NOT on users that constrains such entries, carries `{feature-declined}`. Section 5.1, traffic-triggered SA creation: an outbound packet whose SPD entry calls for PROTECT with no SA is meant to invoke key management, and Ze installs a PROTECT entry only together with the Child SA it negotiated and handles no XFRM acquire, so no packet starts IKEv2; [`RFC4301-5.1-7`](#rfc4301-5.1-7) carries it as a `{gap}`.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 46 | one part of the gated population |
| Annotated (including scoped evidence) | 54 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **100** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (46):** [`RFC4301-4.2-1`](#rfc4301-4.2-1), [`RFC4301-4.4.2-1`](#rfc4301-4.4.2-1), [`RFC4301-5.2-2`](#rfc4301-5.2-2), [`RFC4301-4.1-6`](#rfc4301-4.1-6), [`RFC4301-4.4.3.1-1`](#rfc4301-4.4.3.1-1), [`RFC4301-4.4.3.1-2`](#rfc4301-4.4.3.1-2), [`RFC4301-4.4.1-4`](#rfc4301-4.4.1-4), [`RFC4301-7.4-1`](#rfc4301-7.4-1), [`RFC4301-7.4-2`](#rfc4301-7.4-2), [`RFC4301-3.1-1`](#rfc4301-3.1-1), [`RFC4301-4-1`](#rfc4301-4-1), [`RFC4301-4.1-9`](#rfc4301-4.1-9), [`RFC4301-4.4.1-6`](#rfc4301-4.4.1-6), [`RFC4301-4.4.1-7`](#rfc4301-4.4.1-7), [`RFC4301-4.4.1-10`](#rfc4301-4.4.1-10), [`RFC4301-4.4.1-11`](#rfc4301-4.4.1-11), [`RFC4301-4.4.1-12`](#rfc4301-4.4.1-12), [`RFC4301-4.4.1.1-3`](#rfc4301-4.4.1.1-3), [`RFC4301-4.4.2.1-1`](#rfc4301-4.4.2.1-1), [`RFC4301-4.4.2.1-7`](#rfc4301-4.4.2.1-7), [`RFC4301-4.4.2.1-8`](#rfc4301-4.4.2.1-8), [`RFC4301-4.4.2.1-9`](#rfc4301-4.4.2.1-9), [`RFC4301-4.4.2.1-10`](#rfc4301-4.4.2.1-10), [`RFC4301-4.4.2.1-11`](#rfc4301-4.4.2.1-11), [`RFC4301-4.4.2.1-12`](#rfc4301-4.4.2.1-12), [`RFC4301-4.4.2.1-18`](#rfc4301-4.4.2.1-18), [`RFC4301-4.4.2.1-3`](#rfc4301-4.4.2.1-3), [`RFC4301-4.4.3.1-3`](#rfc4301-4.4.3.1-3), [`RFC4301-4.4.3.2-1`](#rfc4301-4.4.3.2-1), [`RFC4301-4.5.2-1`](#rfc4301-4.5.2-1), [`RFC4301-4.5.3-1`](#rfc4301-4.5.3-1), [`RFC4301-5-1`](#rfc4301-5-1), [`RFC4301-5.1-4`](#rfc4301-5.1-4), [`RFC4301-5.1-5`](#rfc4301-5.1-5), [`RFC4301-5.1-6`](#rfc4301-5.1-6), [`RFC4301-5.2-4`](#rfc4301-5.2-4), [`RFC4301-5.2-6`](#rfc4301-5.2-6), [`RFC4301-5.2-7`](#rfc4301-5.2-7), [`RFC4301-5.2-8`](#rfc4301-5.2-8), [`RFC4301-5.2-10`](#rfc4301-5.2-10), [`RFC4301-7.1-1`](#rfc4301-7.1-1), [`RFC4301-7.1-2`](#rfc4301-7.1-2), [`RFC4301-7.1-3`](#rfc4301-7.1-3), [`RFC4301-7.3-3`](#rfc4301-7.3-3), [`RFC4301-7.3-4`](#rfc4301-7.3-4), [`RFC4301-7.3-5`](#rfc4301-7.3-5)

**Annotated (including scoped evidence) (54):** [`RFC4301-4.1-1`](#rfc4301-4.1-1), [`RFC4301-4.1-2`](#rfc4301-4.1-2), [`RFC4301-4.1-3`](#rfc4301-4.1-3), [`RFC4301-4.1-4`](#rfc4301-4.1-4), [`RFC4301-4.1-5`](#rfc4301-4.1-5), [`RFC4301-4.4.1-1`](#rfc4301-4.4.1-1), [`RFC4301-4.4.1-2`](#rfc4301-4.4.1-2), [`RFC4301-4.4.1.1-1`](#rfc4301-4.4.1.1-1), [`RFC4301-4.4.2-2`](#rfc4301-4.4.2-2), [`RFC4301-4.5-1`](#rfc4301-4.5-1), [`RFC4301-5.2-1`](#rfc4301-5.2-1), [`RFC4301-7-1`](#rfc4301-7-1), [`RFC4301-3.2-1`](#rfc4301-3.2-1), [`RFC4301-4.1-12`](#rfc4301-4.1-12), [`RFC4301-4.1-10`](#rfc4301-4.1-10), [`RFC4301-4.1-11`](#rfc4301-4.1-11), [`RFC4301-4.4.1-5`](#rfc4301-4.4.1-5), [`RFC4301-4.4.1-8`](#rfc4301-4.4.1-8), [`RFC4301-4.4.1.1-2`](#rfc4301-4.4.1.1-2), [`RFC4301-4.4.1.1-4`](#rfc4301-4.4.1.1-4), [`RFC4301-4.4.1.1-5`](#rfc4301-4.4.1.1-5), [`RFC4301-4.4.1.2-1`](#rfc4301-4.4.1.2-1), [`RFC4301-4.4.1.2-2`](#rfc4301-4.4.1.2-2), [`RFC4301-4.4.2.1-4`](#rfc4301-4.4.2.1-4), [`RFC4301-4.4.2.1-5`](#rfc4301-4.4.2.1-5), [`RFC4301-4.4.2.1-6`](#rfc4301-4.4.2.1-6), [`RFC4301-4.4.2.1-13`](#rfc4301-4.4.2.1-13), [`RFC4301-4.4.2.1-14`](#rfc4301-4.4.2.1-14), [`RFC4301-4.4.2.1-15`](#rfc4301-4.4.2.1-15), [`RFC4301-4.4.2.1-16`](#rfc4301-4.4.2.1-16), [`RFC4301-4.4.2.1-17`](#rfc4301-4.4.2.1-17), [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2), [`RFC4301-4.4.3.3-1`](#rfc4301-4.4.3.3-1), [`RFC4301-5.1-3`](#rfc4301-5.1-3), [`RFC4301-5.1-7`](#rfc4301-5.1-7), [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1), [`RFC4301-5.2-9`](#rfc4301-5.2-9), [`RFC4301-6-1`](#rfc4301-6-1), [`RFC4301-6.1.1-1`](#rfc4301-6.1.1-1), [`RFC4301-6.1.1-2`](#rfc4301-6.1.1-2), [`RFC4301-6.1.2-1`](#rfc4301-6.1.2-1), [`RFC4301-6.2-1`](#rfc4301-6.2-1), [`RFC4301-6.2-2`](#rfc4301-6.2-2), [`RFC4301-6.2-3`](#rfc4301-6.2-3), [`RFC4301-6.2-4`](#rfc4301-6.2-4), [`RFC4301-6.2-5`](#rfc4301-6.2-5), [`RFC4301-7.2-1`](#rfc4301-7.2-1), [`RFC4301-7.2-2`](#rfc4301-7.2-2), [`RFC4301-7.3-1`](#rfc4301-7.3-1), [`RFC4301-7.3-2`](#rfc4301-7.3-2), [`RFC4301-8.1-1`](#rfc4301-8.1-1), [`RFC4301-8.1-2`](#rfc4301-8.1-2), [`RFC4301-8.2.1-1`](#rfc4301-8.2.1-1), [`RFC4301-8.2.2-1`](#rfc4301-8.2.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4301-4.1-1` | a) A host implementation of IPsec MUST support both transport and tunnel mode. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L175). **positive:** `unit/verify` [`TestIPsecSAPerDestinationWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L226). **negative:** no negative test. **{single-polarity}:** ze projects both modes -- IKE child SAs install tunnel-mode ESP and OSPFv3 RFC 4552 installs transport-mode ESP/AH; a capability-presence MUST has no meaningful negative (internal/component/ike/engine/child.go:224, :253, internal/plugins/ospf/ipsec_install.go:413, :441) |
| `RFC4301-4.1-2` | A security gateway MUST support tunnel mode (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L190). **negative:** no negative test. **{single-polarity}:** ze (a security gateway) installs tunnel-mode ESP for every IKE-negotiated peer SA and policy (internal/component/ike/engine/child.go:224, :253, :281, :295) |
| `RFC4301-4.1-3` | Aside from the two exceptions below, whenever either end of a security association is a security gateway, the SA MUST be tunnel mode. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L177). **negative:** no negative test. **{single-polarity}:** the IKE child-SA path hardcodes tunnel mode for every peer SA, so a peer SA can never be transport (internal/component/ike/engine/child.go:40, :224, :253) |
| `RFC4301-4.1-4` | IKE creates pairs of SAs, so for simplicity, we choose to require that both SAs in a pair be of the same mode, transport or tunnel. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L179). **negative:** no negative test. **{single-polarity}:** the inbound and outbound child SAs of a pair are both built with modeTunnel, so the pair is always same-mode (internal/component/ike/engine/child.go:224, :253) |
| `RFC4301-4.1-5` | To permit this, the IPsec implementation MUST permit establishment and maintenance of multiple SAs between a given sender and receiver, with the same selectors. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** multiple SAs with identical selectors coexist by SPI in the kernel XFRM SAD and ze's SPI-keyed installs do not forbid this; the DSCP classification that selects among them for QoS is a datapath function ze does not model (internal/component/ike/dataplane/dataplane.go:81-108) |
| `RFC4301-4.2-1` | Note: A compliant implementation MUST NOT allow instantiation of an ESP SA that employs both NULL encryption and no integrity algorithm. (§4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L121). **positive:** `unit/verify` [`TestRFC4301ESPNullEncryptionWithIntegrityIsInstantiated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_esp_null_linux_test.go#L17). **negative:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L102). **negative:** `unit/verify` [`TestRFC4301ESPNullEncryptionWithoutIntegrityIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_esp_null_linux_test.go#L36). **negative:** `unit/verify` [`TestRFC4301VPPESPNullEncryptionWithoutIntegrityIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_esp_null_vpp_test.go#L26) |
| `RFC4301-4.4.1-1` | An IPsec implementation MUST have at least one SPD, and it MAY support multiple SPDs, if appropriate for the context in which the IPsec implementation operates. (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L156). **negative:** no negative test. **{single-polarity}:** ze maintains an SPD policy model (SPParams) and installs at least the PROTECT entries into the kernel SPD via both the IKE and OSPFv3 paths (internal/component/ike/dataplane/dataplane.go:145, engine/child.go:276, :290, internal/plugins/ospf/ipsec_install.go:432-452) |
| `RFC4301-4.4.1-2` | The SPD, or relevant caches, must be consulted during the processing of all traffic (inbound and outbound), including traffic not protected by IPsec, that traverses the IPsec boundary. This includes IPsec management traffic such as IKE. (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** per-packet SPD consultation for every crossing packet is a kernel XFRM datapath function; ze populates the kernel SPD but does not process packets |
| `RFC4301-4.4.1.1-1` | The following selector parameters MUST be supported by all IPsec implementations to facilitate control of SA granularity. (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's policy model carries IP-prefix and next-layer-protocol selectors only; the IKE traffic-selector negotiation discards ports/protocol into an address-only net.IPNet, and SPParams has no port or ICMP-type/code field (internal/component/ike/dataplane/dataplane.go:110-130 exists; ports/ICMP missing at internal/component/ike/engine/sa.go:128-129, engine/initiator.go:325) |
| `RFC4301-4.4.2-1` | For each of the selectors defined in Section 4.4.1.1, the entry for an inbound SA in the SAD MUST be initially populated with the value or values negotiated at the time the SA was created. (§4.4.2) | MUST | 4.4.2 | **positive:** `unit/verify` [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L418). **positive:** `unit/verify` [`TestNarrowedSelectorsReachTheInstalledPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L423). **positive:** `unit/verify` [`TestRFC4301InboundSADEntryCarriesTheNegotiatedSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_test.go#L60). **positive:** `unit/verify` [`TestRFC4301TransportModeInboundSADEntryDropsPacketsOutsideItsSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L401). **positive:** `unit/verify` [`TestRFC4301TunnelModeInboundSelectorsAreEnforcedByTheRequirePolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L345). **negative:** `unit/verify` [`TestRFC4301TransportModeInboundSADEntryDropsPacketsOutsideItsSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L403). **negative:** `unit/verify` [`TestRFC4301TunnelModeInboundSelectorsAreEnforcedByTheRequirePolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L348) |
| `RFC4301-4.4.2-2` | the entry for an inbound SA in the SAD MUST be initially populated with the value or values negotiated at the time the SA was created. (§4.4.2) | MUST | 4.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** when the peer's answer holds more than one traffic selector pair, only the first reaches the dataplane. installChildSA (internal/component/ike/engine/child.go) installs one inbound policy and, in transport mode, one inbound state selector, both from Selectors[0] (selectorProto, selectorPort, TSLocal/TSRemote); an XFRM state carries one selector and the install shape one policy per direction, so the other negotiated values are not populated |
| `RFC4301-4.5-1` | All IPsec implementations MUST support both manual and automated SA and cryptographic key management. (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L150). **positive:** `unit/verify` [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L110). **negative:** no negative test. **{single-polarity}:** ze implements automated keying via its native IKEv2 engine and manual keying via the OSPFv3 RFC 4552 config, both installing SAs through the same dataplane seam (internal/component/ike/engine/child.go:199-307, internal/plugins/ospf/ipsec_install.go:295-342) |
| `RFC4301-5.2-1` | Since the SPD-I is just a part of the SPD, if a packet that is looked up in the SPD-I cannot be matched to an entry there, then the packet MUST be discarded. (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** re-examined under RULINGS R28 on the R29 fail-closed code. Where the catch-all is installed (installUnmatched, internal/component/ike/engine/unmatched.go, ranked last at dataplane.PriorityUnmatched) every inbound packet meets an SPD-I entry and the action is the RFC4301-4.4.1-3 choice, but paths remain where NO entry is installed and the kernel delivers an unmatched packet: (a) the VPP backend holds no entry bound to no interface (vppPolicyInterface, vppBackend.CatchAllSupported, internal/component/ike/dataplane/vpp_policy.go), so under the default `unmatched bypass` it installs no catch-all and the config is not refused; (b) before the first configuration is applied and after the engine exits (applyConfig, internal/component/ike/engine/apply.go; removeUnmatched); (c) a bypass catch-all whose install fails, or a platform without XFRM, is logged and tolerated. `unmatched discard` is refused at verify on (a) and fails the apply on (c) (verifyUnmatchedEnforceable; TestUnmatchedDiscardRefusedAtVerifyWhereItCannotBeEnforced, TestUnmatchedDiscardInstallFailureFailsTheApply), and TestRFC4301InboundClearPacketMatchingNoEntryFollowsTheUnmatchedLeaf shows the installed catch-all deciding an inbound clear packet on XFRM |
| `RFC4301-5.2-2` | Then match the packet against the inbound selectors identified by the SAD entry to verify that the received packet is appropriate for the SA via which it was received. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L132). **positive:** `unit/verify` [`TestRFC4301TransportModeInboundSADEntryDropsPacketsOutsideItsSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L406). **positive:** `unit/verify` [`TestRFC4301TunnelModeInboundSelectorsAreEnforcedByTheRequirePolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L351). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L193). **negative:** `unit/verify` [`TestRFC4301TransportModeInboundSADEntryDropsPacketsOutsideItsSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L408). **negative:** `unit/verify` [`TestRFC4301TunnelModeInboundSelectorsAreEnforcedByTheRequirePolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L353) |
| `RFC4301-4.1-6` | If an IPsec implementation supports multicast, then it MUST support multicast SAs using the algorithm below for mapping inbound IPsec datagrams to SAs. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4301MulticastSAsAreMappedBySPIAndGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4301_multicast_sa_linux_test.go#L126). **negative:** `unit/verify` [`TestRFC4301MulticastSALookupRefusesAnUnconfiguredSPIOrGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4301_multicast_sa_linux_test.go#L152) |
| `RFC4301-7-1` | Note: AH and ESP cannot be applied using transport mode to IPv4 packets that are fragments. Only tunnel mode can be employed in such cases. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies transport mode only to IPv6 OSPFv3 traffic (IPv4-family IPsec is rejected at config), so transport-mode IPsec on an IPv4 fragment structurally cannot arise, and per-packet fragment handling is kernel-delegated (internal/plugins/ospf/config_ipsec.go:21-22) |
| `RFC4301-4.4.1-3` | Every SPD SHOULD have a nominal, final entry that matches anything that is otherwise unmatched, and discards it. (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a disclosed deviation, kept by owner ruling: the `vpn ipsec unmatched` leaf defaults to bypass (ze-ipsec-conf.yang `default bypass`; internal/component/ike/ipsec/config.go::parseUnmatched returns SPActionBypass for an absent leaf), so with the default the final entry installUnmatched (internal/component/ike/engine/unmatched.go) ranks last in every SPD passes unmatched traffic in the clear rather than discarding it. The reason is the router's own control plane (BGP, SSH, DNS), which crosses the IPsec boundary in the clear and would stop the moment `vpn ipsec` was configured on an upgraded box. `unmatched discard` installs the final discard entry the section asks for; docs/guide/ipsec.md "The catch-all entry" discloses the deviation |
| `RFC4301-7-2` | Implementations SHOULD use the approach described in the Path MTU Discovery document (RFC 1191 [MD90], Section 6.3), which suggests periodically resetting the PMTU to the first-hop data-link MTU and then letting the normal PMTU Discovery processes update the PMTU as necessary. (§8.2.2) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.1-7` | Where traffic is destined for a security gateway, e.g., Simple Network Management Protocol (SNMP) commands, the security gateway is acting as a host and transport mode is allowed. (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4301-4.4.3.1-1` | The specific syntax used by an implementation to accommodate sub-tree matching for distinguished names, domain names or RFC 822 e-mail addresses is a local matter. But, at a minimum, sub-tree matching of the sort described above MUST be supported. (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** `unit/verify` [`TestPadSubtreeAdmitsAPeerBeneathIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L35). **positive:** `unit/verify` [`TestPadSubtreeStillBindsTheCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L185). **positive:** `unit/verify` [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_validate_identity_test.go#L166). **negative:** `unit/verify` [`TestPadKeyIDStaysExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L111). **negative:** `unit/verify` [`TestPadSubtreeRefusesAPeerOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L69) |
| `RFC4301-4.4.3.1-2` | For IPv4 and IPv6 addresses, the same address range syntax used for SPD entries MUST be supported. (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** `unit/verify` [`TestPadAddressRangeAdmitsAnAddressInsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L129). **positive:** `unit/verify` [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_validate_identity_test.go#L167). **negative:** `unit/verify` [`TestPadAddressRangeRefusesWhatIsOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L155) |
| `RFC4301-4.4.1-4` | Thus, a user or administrator MUST be able to order the entries to express a desired access control policy. (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestSpdOperatorOrdersOverlappingPeers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L20). **negative:** `unit/verify` [`TestSPDPolicyOrderInsideBypassBandRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L123). **negative:** `unit/verify` [`TestSpdOrderCannotCaptureTheIKEControlPlane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L107). **negative:** `unit/verify` [`TestSpdUnstatedOrderTakesTheDefaultRank`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L67) |
| `RFC4301-7.4-1` | All implementations MUST support DISCARDing of fragments using the normal SPD packet classification mechanisms (§7.4) | MUST | 7.4 | **positive:** `unit/verify` [`TestDiscardPolicyReachesTheBackend`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L45). **positive:** `unit/verify` [`TestDiscardPolicyReachesTheKernelAsBlock`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L41). **positive:** `unit/verify` [`TestPolicyReadbackReportsDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L101). **positive:** `unit/verify` [`TestSPDPolicyCarriesTheDiscardDisposition`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L52). **positive:** `unit/verify` [`TestVPPSPDActionCarriesEveryDisposition`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_vpp_test.go#L23). **negative:** `unit/verify` [`TestBypassPolicyIsNotBlocked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L64). **negative:** `unit/verify` [`TestBypassPolicyReachesTheBackendAsBypass`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L75). **negative:** `unit/verify` [`TestSPDPolicyBypassIsNotADiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L83) |
| `RFC4301-7.4-2` | All implementations MUST support stateful fragment checking to accommodate BYPASS traffic for which a non-trivial port range is specified (§7.4; Appendix D.4 restates it as "implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified") | MUST | 7.4 | **positive:** `unit/verify` [`TestPortScopedBypassNeverReachesTheForwardPath`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_test.go#L74). **positive:** `unit/verify` [`TestXFRMPortScopedBypassIsNeverStoredOnTheForwardPath`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_fragment_integration_linux_test.go#L69). **negative:** `unit/verify` [`TestIKEBypassIsPortScopedSoSection74Binds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_test.go#L133) |
| `RFC4301-3.1-1` | A compliant host implementation MUST support (a) and (c) and a compliant security gateway must support all three of these forms of connectivity, since under certain circumstances a security gateway acts as a host (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L128). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L189) |
| `RFC4301-3.2-1` | IPsec implementations MUST support ESP and MAY support AH (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC4301ChildSAIsInstalledAsESP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_esp_test.go#L19). **negative:** no negative test. **{single-polarity}:** "support ESP" has no violating input, every IKE Child SA installs ESP (internal/component/ike/engine/child.go::installChildSA) and AH is the MAY Ze offers on the OSPFv3 manual path only |
| `RFC4301-4-1` | All implementations of AH or ESP MUST support the concept of an SA as described below (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L69). **negative:** `unit/verify` [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L112) |
| `RFC4301-4.1-12` | The indication of whether source and destination address matching is required to map inbound IPsec traffic to SAs MUST be set either as a side effect of manual SA configuration or via negotiation using an SA management protocol, e.g., IKE or Group Domain of Interpretation (GDOI) [RFC3547]. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4301MulticastSASourceMatchingIsSetByManualConfiguration`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4301_multicast_sa_linux_test.go#L180). **negative:** no negative test. **{single-polarity}:** Ze's multicast SAs exist only as the RFC 4552 manual configuration of an OSPFv3 interface, and that configuration always sets the indication (a state selector whose source is the wildcard ::/0, so the SA is found by SPI and group and no source address match is demanded of inbound traffic), so no input builds a multicast SA whose indication came from anywhere else and no refusal path exists (internal/plugins/ospf/ipsec_install.go::buildIPsecSA) |
| `RFC4301-4.1-9` | The receiver MUST process the packets from the different SAs without prejudice. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L70). **positive:** `unit/verify` [`TestRFC4301ParallelSAsWithOneSelectorAreEachReceived`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_parallel_sa_linux_test.go#L81). **negative:** `unit/verify` [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L113). **negative:** `unit/verify` [`TestRFC4301ParallelSAsAreHeldToTheOneSharedPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_parallel_sa_linux_test.go#L108) |
| `RFC4301-4.1-10` | In transport mode, the DSCP value might change en route, but this should not cause problems with respect to IPsec processing since the value is not employed for SA selection and MUST NOT be checked as part of SA/packet validation. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes no DSCP into the state and xfrmPolicyFromParams none into the selector, so no value Ze installs is read against DSCP |
| `RFC4301-4.1-11` | In IPv6, the security protocol header appears after the base IP header and selected extension headers, but may appear before or after destination options; it MUST appear before next layer protocols (e.g., TCP, UDP, Stream Control Transmission Protocol (SCTP)). (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the ESP state and the kernel output path builds every header, so no value Ze writes decides where the ESP header sits |
| `RFC4301-4.4.1-5` | However, if an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing. (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze holds one SPD and no selection function; plan/spec-ipsec-spd-selection.md |
| `RFC4301-4.4.1-6` | The SPD MUST permit a user or administrator to specify policy entries as follows: (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC4301SPDPermitsEachDispositionInEachDatabase`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L35). **positive:** `unit/verify` [`TestRFC4301SPDSEntryCarriesSelectorsSAControlsAndProtection`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_s_test.go#L58). **negative:** `unit/verify` [`TestRFC4301SPDRefusesADispositionItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L70). **negative:** `unit/verify` [`TestRFC4301SPDSEntryRefusesAMissingOrUnusablePart`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_s_test.go#L110) |
| `RFC4301-4.4.1-7` | For every IPsec implementation, there MUST be a management interface that allows a user or system administrator to manage the SPD (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC4301SPDIsManagedThroughTheConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L100). **negative:** `unit/verify` [`TestRFC4301SPDInterfaceRefusesWhatItCannotProgram`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L143) |
| `RFC4301-4.4.1-8` | (The means of signaling such requests to the IPsec implementation are outside the scope of this standard.) However, the system administrator MUST be able to specify whether or not a user or application can override (default) system policies. (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no user or application override control exists in the SPD model; plan/spec-ipsec-spd-selection.md |
| `RFC4301-4.4.1-10` | If the SPD is not decorrelated, caching is not allowed and an ordered search of SPD MUST be performed to verify that inbound traffic arriving on an SA is consistent with the access control policy expressed in the SPD. (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L129). **positive:** `unit/verify` [`TestRFC4301OrderedInboundSearchAdmitsOnTheFirstRankedEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ordered_spd_linux_test.go#L87). **positive:** `unit/verify` [`TestRFC4301ParallelSAsWithOneSelectorAreEachReceived`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_parallel_sa_linux_test.go#L84). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L190). **negative:** `unit/verify` [`TestRFC4301OrderedInboundSearchRefusesOnTheFirstRankedDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ordered_spd_linux_test.go#L140). **negative:** `unit/verify` [`TestRFC4301ParallelSAsAreHeldToTheOneSharedPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_parallel_sa_linux_test.go#L111) |
| `RFC4301-4.4.1-11` | SPD-I: For inbound traffic that is to be bypassed or discarded, the entry consists of the values of the selectors that apply to the traffic to be bypassed or discarded. (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC4301BypassReturnTrafficHasAnSPDIEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L48). **positive:** `unit/verify` [`TestSPDPolicyMirrorsTheInboundSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L94). **negative:** `unit/verify` [`TestRFC4301BypassSPDIEntryIsNeverAnUnmirroredCopy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L69) |
| `RFC4301-4.4.1-12` | SPD-O: For outbound traffic that is to be bypassed or discarded, the entry consists of the values of the selectors that apply to the traffic to be bypassed or discarded. (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC4301InboundBypassReturnTrafficHasAnSPDOEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L88). **positive:** `unit/verify` [`TestSPDPolicyMirrorsTheInboundSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L95). **negative:** `unit/verify` [`TestRFC4301BypassSPDOEntryIsNeverAnUnmirroredCopy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L109) |
| `RFC4301-4.4.1.1-2` | Note that the Local and Remote ports may not be available in the case of receipt of a fragmented packet or if the port fields have been protected by IPsec (encrypted); thus, a value of OPAQUE also MUST be supported. (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the operator SPD list refuses OPAQUE and the XFRM selector cannot express port 0 mask 0xffff; plan/spec-ipsec-opaque-selector-port-mask.md |
| `RFC4301-4.4.1.1-3` | If the SA requires a port value other than ANY or OPAQUE, an arriving fragment without ports MUST be discarded (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L130). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L191) |
| `RFC4301-4.4.1.1-4` | Given a policy entry with a range of Types (T-start to T-end) and a range of Codes (C-start to C-end), and an ICMP packet with Type t and Code c, an implementation MUST test for a match using (T-start*256) + C-start <= (t*256) + c <= (T-end*256) + C-end (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** SPParams carries no ICMP type or code selector; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-4.4.1.1-5` | The name used to match this field is communicated during the IKE negotiation in the ID payload. In this context, the initiator's Source IP address (inner IP header in tunnel mode) is bound to the Remote IP address in the SAD entry created by the IKE negotiation. This address overrides the Remote IP address value in the SPD, when the SPD entry is selected in this fashion. All IPsec implementations MUST support this use of names. (§4.4.1.1) | MUST | 4.4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no name-valued selector selects an operator SPD entry during IKE; plan/spec-ipsec-spd-selection.md |
| `RFC4301-4.4.1.2-1` | One to N selector sets that correspond to the "condition" for applying a particular IPsec action. (§4.4.1.2) | MUST | 4.4.1.2 | **positive:** `unit/verify` [`TestRFC4301SPDEntryCannotCarryASecondSelectorSet`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_selector_set_test.go#L56). **positive:** `unit/verify` [`TestRFC4301SPDEntryHoldsOneSelectorSet`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_selector_set_test.go#L88). **negative:** no negative test. **{single-polarity}:** internal/component/ike/ipsec/spd_policy.go::parseSPDPolicy reads every operator SPD entry as exactly one selector set (one local and one remote prefix, one protocol, one port pair), N = 1, so no input builds an entry holding zero or several sets and no refusal path exists |
| `RFC4301-4.4.1.2-2` | Until IKE provides a facility that conveys the semantics that are expressed in the SPD via selector sets (as described below), users MUST NOT include multiple selector sets in a single SPD entry unless the access control intent aligns with the IKE "mix and match" semantics. (§4.4.1.2) | MUST NOT | 4.4.1.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "It is possible to associate multiple protocols (and ports) with a single SA by specifying multiple selector sets for that SA"; internal/component/ike/ipsec/spd_policy.go::parseSPDPolicy does the narrower thing: it reads one selector set per SPD entry, so the multiple-set entry this MUST NOT constrains cannot be configured |
| `RFC4301-4.4.2.1-1` | Security Parameter Index (SPI): a 32-bit value selected by the receiving end of an SA to uniquely identify the SA. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301SADItemSPIIsTheNegotiatedValue`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L23). **negative:** `unit/verify` [`TestRFC4301SADItemZeroSPIIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L40) |
| `RFC4301-4.4.2.1-4` | Sequence Number Counter: a 64-bit counter used to generate the Sequence Number field in AH or ESP headers. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301SADItemSequenceCounterIsThe32BitCounterNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L51). **negative:** no negative test. **{single-polarity}:** Ze negotiates 32-bit sequence numbers only (ESN transform "not extended", which the item permits "if negotiated") and builds every SAD entry without the ESN flag, so no input selects another counter width to refuse |
| `RFC4301-4.4.2.1-5` | Sequence Counter Overflow: a flag indicating whether overflow of the sequence number counter should generate an auditable event and prevent transmission of additional packets on the SA, or whether rollover is permitted. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301SADItemSequenceCounterOverflowIsNeverRollover`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L279). **negative:** no negative test. **{single-polarity}:** Ze never offers rollover: xfrmStateFromParams builds every SAD entry with the rollover-permitted flag unset and no ESN, for every input, so no input asks for rollover and there is nothing to refuse (internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams) |
| `RFC4301-4.4.2.1-6` | Anti-Replay Window: a 64-bit counter and a bit-map (or equivalent) used to determine whether an inbound AH or ESP packet is a replay. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L72). **positive:** `unit/verify` [`TestRFC4301SADItemAntiReplayWindowIsInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L67). **negative:** no negative test. **{single-polarity}:** every window size Ze is given is installed as given, including zero for a receiver that disabled anti-replay, so no input is refused |
| `RFC4301-4.4.2.1-7` | AH Authentication algorithm, key, etc. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301SADItemAHCarriesItsIntegrityAlgorithmAndKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L85). **negative:** `unit/verify` [`TestRFC4301SADItemAHUnknownIntegrityIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L114) |
| `RFC4301-4.4.2.1-8` | ESP Encryption algorithm, key, mode, IV, etc. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301SADItemESPTransformsCarryAlgorithmAndKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L127). **negative:** `unit/verify` [`TestRFC4301SADItemUnknownESPTransformIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L154) |
| `RFC4301-4.4.2.1-9` | ESP integrity algorithm, keys, etc. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301SADItemESPTransformsCarryAlgorithmAndKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L128). **negative:** `unit/verify` [`TestRFC4301SADItemUnknownESPTransformIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L155) |
| `RFC4301-4.4.2.1-10` | ESP combined mode algorithms, key(s), etc. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301SADItemCombinedModeCarriesOneAEADTransform`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L172). **negative:** `unit/verify` [`TestRFC4301SADItemUnknownCombinedModeIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L198) |
| `RFC4301-4.4.2.1-11` | Lifetime of this SA: a time interval after which an SA must be replaced with a new SA (and new SPI) or terminated, plus an indication of which of these actions should occur. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301SADLifetimeIndicatesReplaceBeforeTerminate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_lifetime_test.go#L23). **positive:** `unit/verify` [`TestRFC4301SADLifetimeSoftTimeReplacesTheSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_lifetime_action_test.go#L23). **negative:** `unit/verify` [`TestRFC4301SADLifetimeHardTimeTerminatesTheSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_lifetime_action_test.go#L81). **negative:** `unit/verify` [`TestRFC4301SADLifetimeTerminatesTheSAAtTheEndOfItsInterval`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_lifetime_test.go#L52) |
| `RFC4301-4.4.2.1-12` | IPsec protocol mode: tunnel or transport. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301SADItemModeIsInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L210). **negative:** `unit/verify` [`TestRFC4301SADItemUnknownModeIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L234) |
| `RFC4301-4.4.2.1-13` | Stateful fragment checking flag. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SAD entry Ze installs holds a stateful fragment checking flag, and no config sets one; no spec owns it yet |
| `RFC4301-4.4.2.1-14` | Bypass DF bit (T/F) -- applicable to tunnel mode SAs where both inner and outer headers are IPv4. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the DF treatment of a tunnel-mode SA is not configurable and no SAD entry holds it; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-4.4.2.1-15` | DSCP values -- the set of DSCP values allowed for packets carried over this SA. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SAD entry Ze installs holds a set of DSCP values, so no outbound SA selection by DSCP exists; no spec owns it yet |
| `RFC4301-4.4.2.1-16` | Bypass DSCP (T/F) or map to unprotected DSCP values (array) if needed to restrict bypass of DSCP values -- applicable to tunnel mode SAs. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SAD entry holds a bypass DSCP flag or map, and the outer-header DSCP is not mapped; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-4.4.2.1-17` | Path MTU: any observed path MTU and aging variables. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no per-SA PMTU is held or aged; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-4.4.2.1-18` | Tunnel header IP source and destination address -- both addresses must be either IPv4 or IPv6 addresses. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301SADItemTunnelHeaderAddressesAreInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L246). **negative:** `unit/verify` [`TestRFC4301SADItemMixedFamilyTunnelHeaderIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L265) |
| `RFC4301-4.4.2.1-2` | A compliant implementation MUST support both types of lifetimes, and MUST support a simultaneous use of both. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** newLifetimeState sets a time lifetime only and never a byte count; plan/spec-ipsec-lifetime-volume.md |
| `RFC4301-4.4.2.1-3` | Note that implementations MUST be able to handle having the counters at the ends of an SA get out of synch, e.g., because of packet loss or because the implementations at each end of the SA aren't doing things the same way. (§4.4.2.1) | MUST | 4.4.2.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L71). **negative:** `unit/verify` [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L114) |
| `RFC4301-4.4.3.1-3` | For this name type, only exact-match syntax MUST be supported (since there is no explicit structure for this ID type). (§4.4.3.1) | MUST | 4.4.3.1 | **positive:** `unit/verify` [`TestRFC4301PadOtherIDTypeAdmitsAnExactMatch`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L18). **negative:** `unit/verify` [`TestRFC4301PadOtherIDTypeRefusesEverythingButExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L35) |
| `RFC4301-4.4.3.2-1` | Thus, implementations MUST provide a means for an administrator to require a match between an asserted IKE ID and the subject name or subject alt name in a certificate. (§4.4.3.2) | MUST | 4.4.3.2 | **positive:** `unit/verify` [`TestRFC4301PadRequiresTheCertificateToCarryTheIKEID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L66). **negative:** `unit/verify` [`TestRFC4301PadRefusesACertificateWithoutTheIKEID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_exact_test.go#L96) |
| `RFC4301-4.4.3.3-1` | (A peer may be authorized for both address types, so there MUST be provision for both a v4 and a v6 address range.) (§4.4.3.3) | MUST | 4.4.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** one remote-id string holds one address range, not a v4 and a v6 range; plan/spec-ipsec-pad-dual-family-range.md |
| `RFC4301-4.5.2-1` | To ensure that the IPsec implementations at each end of the SA use the same bits for the same keys, and irrespective of which part of the system divides the string of bits into individual keys, the encryption keys MUST be taken from the first (left-most, high-order) bits and the integrity keys MUST be taken from the remaining bits. (§4.5.2) | MUST | 4.5.2 | **positive:** `unit/verify` [`TestRFC4301KeymatEncryptionKeysLeadEachSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4301_keymat_test.go#L45). **negative:** `unit/verify` [`TestRFC4301KeymatIntegrityKeysNeverLead`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4301_keymat_test.go#L72) |
| `RFC4301-4.5.3-1` | To address these problems, an IPsec-supporting host or security gateway MUST have an administrative interface that allows the user/administrator to configure the address of one or more security gateways for ranges of destination addresses that require its use. (§4.5.3) | MUST | 4.5.3 | **positive:** `unit/verify` [`TestRFC4301GatewayIsConfiguredForDestinationRanges`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_gateway_test.go#L39). **negative:** `unit/verify` [`TestRFC4301GatewayRefusesAMalformedDestinationRange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_gateway_test.go#L65) |
| `RFC4301-5-1` | If no policy is found in the SPD that matches a packet (for either inbound or outbound traffic), the packet MUST be discarded (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4301UnmatchedDiscardIsTheLastEntryOfEveryDatabase`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_unmatched_test.go#L23). **positive:** `unit/verify` [`TestRFC4301UnmatchedLeafIsReadAsWritten`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_unmatched_test.go#L23). **negative:** `unit/verify` [`TestRFC4301UnmatchedLeafRefusesAnUnknownWord`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_unmatched_test.go#L42). **negative:** `unit/verify` [`TestRFC4301UnmatchedNeverDiscardsWhatTheOperatorDidNotAskToDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_unmatched_test.go#L82) |
| `RFC4301-5.1-3` | With regard to determining and enforcing the PMTU of an SA, the IPsec system MUST follow the steps described in Section 8.2 (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no per-SA PMTU is held or aged; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-5.1-4` | Match the packet headers against the cache for the SPD specified by the SPD-ID from step 1. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L131). **positive:** `unit/verify` [`TestRFC4301OutboundPacketTakesTheDispositionOfTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L189). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L192). **negative:** `unit/verify` [`TestRFC4301OutboundPacketIsDiscardedByTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L238) |
| `RFC4301-5.1-5` | If there is a match, then process the packet as specified by the matching cache entry, i.e., BYPASS, DISCARD, or PROTECT using AH or ESP. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC4301OutboundPacketTakesTheDispositionOfTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L192). **negative:** `unit/verify` [`TestRFC4301OutboundPacketIsDiscardedByTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L241) |
| `RFC4301-5.1-6` | If no match is found in the cache, search the SPD (SPD-S and SPD-O parts) specified by SPD-ID. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC4301OutboundPacketTakesTheDispositionOfTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L195). **negative:** `unit/verify` [`TestRFC4301OutboundPacketIsDiscardedByTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L243) |
| `RFC4301-5.1-7` | If the SPD entry calls for PROTECT, i.e., creation of an SA, the key management mechanism (e.g., IKEv2) is invoked to create the SA. If SA creation succeeds, a new outbound (SPD-S) cache entry is created, along with outbound and inbound SAD entries, otherwise the packet is discarded. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze installs a Child SA's PROTECT entry only together with the SA it negotiated (createFirstChildSA, internal/component/ike/engine/child.go) and handles no XFRM acquire, so no outbound packet invokes IKEv2; a packet for a peer with no SA yet takes the catch-all entry, which bypasses by default (the RFC4301-4.4.1-3 deviation); no spec schedules traffic-triggered SA creation |
| `RFC4301-5.1.2.1-1` | If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain (§5.1.2.1) | MUST | 5.1.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the outer-header DSCP is not mapped for the domain entered; plan/immediate/spec-rfc4301-architecture-gaps.md |
| `RFC4301-5.2-4` | IKE traffic MUST have an explicit BYPASS entry in the SPD (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC4301IKETrafficHasAnExplicitBypassEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ike_bypass_test.go#L21). **negative:** `unit/verify` [`TestRFC4301IKEBypassIsNeverAProtectOrAWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ike_bypass_test.go#L54) |
| `RFC4301-5.2-6` | If the packet is addressed to the IPsec device and AH or ESP is specified as the protocol, the packet is looked up in the SAD. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC4301InboundESPIsProcessedWithTheSAItsSPISelects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L142). **negative:** `unit/verify` [`TestRFC4301InboundESPWithNoSADMatchIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L175) |
| `RFC4301-5.2-7` | In either case (unicast or multicast), if there is no match, discard the traffic. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC4301InboundESPIsProcessedWithTheSAItsSPISelects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L144). **negative:** `unit/verify` [`TestRFC4301InboundESPWithNoSADMatchIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L177) |
| `RFC4301-5.2-8` | If the packet is not addressed to the device or is addressed to this device and is not AH or ESP, look up the packet header in the (appropriate) SPD-I cache. If there is a match and the packet is to be discarded or bypassed, do so. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC4301InboundClearPacketIsBypassedByTheSPDIEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L241). **negative:** `unit/verify` [`TestRFC4301InboundClearPacketIsDiscardedByTheSPDIEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L266) |
| `RFC4301-5.2-9` | (No SAs are created in response to receipt of a packet that requires IPsec protection; only BYPASS or DISCARD cache entries can be created this way.) If there is no match, discard the traffic. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** re-examined under RULINGS R28 on the R29 fail-closed code. Where the catch-all is installed (installUnmatched, internal/component/ike/engine/unmatched.go, ranked last at dataplane.PriorityUnmatched) every inbound packet meets an SPD-I entry and the action is the RFC4301-4.4.1-3 choice, but paths remain where NO entry is installed and the kernel delivers an unmatched packet: (a) the VPP backend holds no entry bound to no interface (vppPolicyInterface, vppBackend.CatchAllSupported, internal/component/ike/dataplane/vpp_policy.go), so under the default `unmatched bypass` it installs no catch-all and the config is not refused; (b) before the first configuration is applied and after the engine exits (applyConfig, internal/component/ike/engine/apply.go; removeUnmatched); (c) a bypass catch-all whose install fails, or a platform without XFRM, is logged and tolerated. `unmatched discard` is refused at verify on (a) and fails the apply on (c) (verifyUnmatchedEnforceable; TestUnmatchedDiscardRefusedAtVerifyWhereItCannotBeEnforced, TestUnmatchedDiscardInstallFailureFailsTheApply), and TestRFC4301InboundClearPacketMatchingNoEntryFollowsTheUnmatchedLeaf shows the installed catch-all deciding an inbound clear packet on XFRM |
| `RFC4301-5.2-10` | Apply AH or ESP processing as specified, using the SAD entry selected in step 3a above. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC4301InboundESPIsProcessedWithTheSAItsSPISelects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L146). **negative:** `unit/verify` [`TestRFC4301InboundESPFailingIntegrityIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L204) |
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
| `RFC4301-7.1-3` | If the SA will carry traffic without regard to a specific protocol value (i.e., ANY is specified as the (Next Layer) protocol selector value), then the port field values are undefined and MUST be set to ANY as well. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC4301ProtocolAnyChildSACarriesAnyPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_protocol_any_sa_test.go#L19). **positive:** `unit/verify` [`TestRFC4301ProtocolAnyEntryCarriesAnyPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L178). **negative:** `unit/verify` [`TestRFC4301ProtocolAnyChildSARefusesAPort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_protocol_any_sa_test.go#L45). **negative:** `unit/verify` [`TestRFC4301ProtocolAnyEntryRefusesAPort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L199) |
| `RFC4301-7.2-1` | Receivers MUST perform a minimum offset check on IPv4 (non-initial) fragments to protect against overlapping fragment attacks when SAs of this type are employed. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/engine/child.go::childPolicyParams builds one policy pair per Child SA and no separate non-initial-fragment SA, so approach #2 of Section 7.2 is never taken and no value Ze installs decides the offset check the kernel reassembly performs |
| `RFC4301-7.2-2` | Specific port (or ICMP type/code or Mobility Header type) selector values will be used to define SAs to carry initial fragments and non-fragmented packets. This approach can be used if a user or administrator wants to create one or more tunnel mode SAs between the same Local/Remote addresses that discriminate based on port (or ICMP type/code or Mobility Header type) fields. These SAs MUST have non-trivial protocol selector values, otherwise approach #1 above MUST be used. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/engine/child.go::childPolicyParams builds no fragment-carrying SA, so the protocol selector this sentence constrains is never written by Ze |
| `RFC4301-7.3-1` | Implementations that will transmit non-initial fragments on a tunnel mode SA that makes use of non-trivial port (or ICMP type/code or MH type) selectors MUST notify a peer via the IKE NOTIFY NON_FIRST_FRAGMENTS_ALSO payload (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never sends NON_FIRST_FRAGMENTS_ALSO; plan/spec-ipsec-non-first-fragments-also.md |
| `RFC4301-7.3-2` | The peer MUST reject this proposal if it will not accept non-initial fragments in this context. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never parses or answers NON_FIRST_FRAGMENTS_ALSO; plan/spec-ipsec-non-first-fragments-also.md |
| `RFC4301-7.3-3` | If an implementation does not successfully negotiate transmission of non-initial fragments for such an SA, it MUST NOT send such fragments over the SA (§7.3) | MUST NOT | 7.3 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L133). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L194) |
| `RFC4301-7.3-4` | However, a receiver MUST discard non-initial fragments that arrive on an SA with non-trivial port (or ICMP type/code or MH type) selector values unless this feature has been negotiated. (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L134). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L195) |
| `RFC4301-7.3-5` | Also, the receiver MUST discard non-initial fragments that do not comply with the security policy applied to the overall packet. (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L135). **positive:** `unit/verify` [`TestRFC4301FragmentsOfACompliantPacketAreDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_linux_test.go#L57). **negative:** `unit/verify` [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L196). **negative:** `unit/verify` [`TestRFC4301FragmentsOfANonCompliantPacketAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_linux_test.go#L86) |
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
| [`RFC4301-4.4.2-2`](#rfc4301-4.4.2-2) the entry for an inbound SA in the SAD MUST be initially populated with the value or values negotiated at the time the SA was created. (§4.4.2) | {gap}, no test | when the peer's answer holds more than one traffic selector pair, only the first reaches the dataplane. installChildSA (internal/component/ike/engine/child.go) installs one inbound policy and, in transport mode, one inbound state selector, both from Selectors[0] (selectorProto, selectorPort, TSLocal/TSRemote); an XFRM state carries one selector and the install shape one policy per direction, so the other negotiated values are not populated |
| [`RFC4301-5.2-1`](#rfc4301-5.2-1) Since the SPD-I is just a part of the SPD, if a packet that is looked up in the SPD-I cannot be matched to an entry there, then the packet MUST be discarded. (§4.4.1) | {gap}, no test | re-examined under RULINGS R28 on the R29 fail-closed code. Where the catch-all is installed (installUnmatched, internal/component/ike/engine/unmatched.go, ranked last at dataplane.PriorityUnmatched) every inbound packet meets an SPD-I entry and the action is the RFC4301-4.4.1-3 choice, but paths remain where NO entry is installed and the kernel delivers an unmatched packet: (a) the VPP backend holds no entry bound to no interface (vppPolicyInterface, vppBackend.CatchAllSupported, internal/component/ike/dataplane/vpp_policy.go), so under the default `unmatched bypass` it installs no catch-all and the config is not refused; (b) before the first configuration is applied and after the engine exits (applyConfig, internal/component/ike/engine/apply.go; removeUnmatched); (c) a bypass catch-all whose install fails, or a platform without XFRM, is logged and tolerated. `unmatched discard` is refused at verify on (a) and fails the apply on (c) (verifyUnmatchedEnforceable; TestUnmatchedDiscardRefusedAtVerifyWhereItCannotBeEnforced, TestUnmatchedDiscardInstallFailureFailsTheApply), and TestRFC4301InboundClearPacketMatchingNoEntryFollowsTheUnmatchedLeaf shows the installed catch-all deciding an inbound clear packet on XFRM |
| [`RFC4301-7-1`](#rfc4301-7-1) Note: AH and ESP cannot be applied using transport mode to IPv4 packets that are fragments. Only tunnel mode can be employed in such cases. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze applies transport mode only to IPv6 OSPFv3 traffic (IPv4-family IPsec is rejected at config), so transport-mode IPsec on an IPv4 fragment structurally cannot arise, and per-packet fragment handling is kernel-delegated (internal/plugins/ospf/config_ipsec.go:21-22) |
| [`RFC4301-4.4.1-3`](#rfc4301-4.4.1-3) Every SPD SHOULD have a nominal, final entry that matches anything that is otherwise unmatched, and discards it. (§4.4.1) | {gap}, demonstrated by internal/component/ike/engine/rfc4301_final_entry_test.go::TestRFC4301DefaultFinalSPDEntryDiscards | a disclosed deviation, kept by owner ruling: the `vpn ipsec unmatched` leaf defaults to bypass (ze-ipsec-conf.yang `default bypass`; internal/component/ike/ipsec/config.go::parseUnmatched returns SPActionBypass for an absent leaf), so with the default the final entry installUnmatched (internal/component/ike/engine/unmatched.go) ranks last in every SPD passes unmatched traffic in the clear rather than discarding it. The reason is the router's own control plane (BGP, SSH, DNS), which crosses the IPsec boundary in the clear and would stop the moment `vpn ipsec` was configured on an upgraded box. `unmatched discard` installs the final discard entry the section asks for; docs/guide/ipsec.md "The catch-all entry" discloses the deviation |
| [`RFC4301-4.1-10`](#rfc4301-4.1-10) In transport mode, the DSCP value might change en route, but this should not cause problems with respect to IPsec processing since the value is not employed for SA selection and MUST NOT be checked as part of SA/packet validation. (§4.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes no DSCP into the state and xfrmPolicyFromParams none into the selector, so no value Ze installs is read against DSCP |
| [`RFC4301-4.1-11`](#rfc4301-4.1-11) In IPv6, the security protocol header appears after the base IP header and selected extension headers, but may appear before or after destination options; it MUST appear before next layer protocols (e.g., TCP, UDP, Stream Control Transmission Protocol (SCTP)). (§4.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the ESP state and the kernel output path builds every header, so no value Ze writes decides where the ESP header sits |
| [`RFC4301-4.4.1-5`](#rfc4301-4.4.1-5) However, if an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing. (§4.4.1) | {gap}, no test | Ze holds one SPD and no selection function; plan/spec-ipsec-spd-selection.md |
| [`RFC4301-4.4.1-8`](#rfc4301-4.4.1-8) (The means of signaling such requests to the IPsec implementation are outside the scope of this standard.) However, the system administrator MUST be able to specify whether or not a user or application can override (default) system policies. (§4.4.1) | {gap}, no test | no user or application override control exists in the SPD model; plan/spec-ipsec-spd-selection.md |
| [`RFC4301-4.4.1.1-2`](#rfc4301-4.4.1.1-2) Note that the Local and Remote ports may not be available in the case of receipt of a fragmented packet or if the port fields have been protected by IPsec (encrypted); thus, a value of OPAQUE also MUST be supported. (§4.4.1.1) | {gap}, no test | the operator SPD list refuses OPAQUE and the XFRM selector cannot express port 0 mask 0xffff; plan/spec-ipsec-opaque-selector-port-mask.md |
| [`RFC4301-4.4.1.1-4`](#rfc4301-4.4.1.1-4) Given a policy entry with a range of Types (T-start to T-end) and a range of Codes (C-start to C-end), and an ICMP packet with Type t and Code c, an implementation MUST test for a match using (T-start*256) + C-start <= (t*256) + c <= (T-end*256) + C-end (§4.4.1.1) | {gap}, no test | SPParams carries no ICMP type or code selector; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-4.4.1.1-5`](#rfc4301-4.4.1.1-5) The name used to match this field is communicated during the IKE negotiation in the ID payload. In this context, the initiator's Source IP address (inner IP header in tunnel mode) is bound to the Remote IP address in the SAD entry created by the IKE negotiation. This address overrides the Remote IP address value in the SPD, when the SPD entry is selected in this fashion. All IPsec implementations MUST support this use of names. (§4.4.1.1) | {gap}, no test | no name-valued selector selects an operator SPD entry during IKE; plan/spec-ipsec-spd-selection.md |
| [`RFC4301-4.4.1.2-2`](#rfc4301-4.4.1.2-2) Until IKE provides a facility that conveys the semantics that are expressed in the SPD via selector sets (as described below), users MUST NOT include multiple selector sets in a single SPD entry unless the access control intent aligns with the IKE "mix and match" semantics. (§4.4.1.2) | no test | no test carries this requirement id; annotated {feature-declined}: "It is possible to associate multiple protocols (and ports) with a single SA by specifying multiple selector sets for that SA"; internal/component/ike/ipsec/spd_policy.go::parseSPDPolicy does the narrower thing: it reads one selector set per SPD entry, so the multiple-set entry this MUST NOT constrains cannot be configured |
| [`RFC4301-4.4.2.1-13`](#rfc4301-4.4.2.1-13) Stateful fragment checking flag. (§4.4.2.1) | {gap}, no test | no SAD entry Ze installs holds a stateful fragment checking flag, and no config sets one; no spec owns it yet |
| [`RFC4301-4.4.2.1-14`](#rfc4301-4.4.2.1-14) Bypass DF bit (T/F) -- applicable to tunnel mode SAs where both inner and outer headers are IPv4. (§4.4.2.1) | {gap}, no test | the DF treatment of a tunnel-mode SA is not configurable and no SAD entry holds it; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-4.4.2.1-15`](#rfc4301-4.4.2.1-15) DSCP values -- the set of DSCP values allowed for packets carried over this SA. (§4.4.2.1) | {gap}, no test | no SAD entry Ze installs holds a set of DSCP values, so no outbound SA selection by DSCP exists; no spec owns it yet |
| [`RFC4301-4.4.2.1-16`](#rfc4301-4.4.2.1-16) Bypass DSCP (T/F) or map to unprotected DSCP values (array) if needed to restrict bypass of DSCP values -- applicable to tunnel mode SAs. (§4.4.2.1) | {gap}, no test | no SAD entry holds a bypass DSCP flag or map, and the outer-header DSCP is not mapped; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-4.4.2.1-17`](#rfc4301-4.4.2.1-17) Path MTU: any observed path MTU and aging variables. (§4.4.2.1) | {gap}, no test | no per-SA PMTU is held or aged; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2) A compliant implementation MUST support both types of lifetimes, and MUST support a simultaneous use of both. (§4.4.2.1) | {gap}, no test | newLifetimeState sets a time lifetime only and never a byte count; plan/spec-ipsec-lifetime-volume.md |
| [`RFC4301-4.4.3.3-1`](#rfc4301-4.4.3.3-1) (A peer may be authorized for both address types, so there MUST be provision for both a v4 and a v6 address range.) (§4.4.3.3) | {gap}, no test | one remote-id string holds one address range, not a v4 and a v6 range; plan/spec-ipsec-pad-dual-family-range.md |
| [`RFC4301-5.1-3`](#rfc4301-5.1-3) With regard to determining and enforcing the PMTU of an SA, the IPsec system MUST follow the steps described in Section 8.2 (§5.1) | {gap}, no test | no per-SA PMTU is held or aged; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-5.1-7`](#rfc4301-5.1-7) If the SPD entry calls for PROTECT, i.e., creation of an SA, the key management mechanism (e.g., IKEv2) is invoked to create the SA. If SA creation succeeds, a new outbound (SPD-S) cache entry is created, along with outbound and inbound SAD entries, otherwise the packet is discarded. (§5.1) | {gap}, no test | Ze installs a Child SA's PROTECT entry only together with the SA it negotiated (createFirstChildSA, internal/component/ike/engine/child.go) and handles no XFRM acquire, so no outbound packet invokes IKEv2; a packet for a peer with no SA yet takes the catch-all entry, which bypasses by default (the RFC4301-4.4.1-3 deviation); no spec schedules traffic-triggered SA creation |
| [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1) If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain (§5.1.2.1) | {gap}, no test | the outer-header DSCP is not mapped for the domain entered; plan/immediate/spec-rfc4301-architecture-gaps.md |
| [`RFC4301-5.2-9`](#rfc4301-5.2-9) (No SAs are created in response to receipt of a packet that requires IPsec protection; only BYPASS or DISCARD cache entries can be created this way.) If there is no match, discard the traffic. (§5.2) | {gap}, no test | re-examined under RULINGS R28 on the R29 fail-closed code. Where the catch-all is installed (installUnmatched, internal/component/ike/engine/unmatched.go, ranked last at dataplane.PriorityUnmatched) every inbound packet meets an SPD-I entry and the action is the RFC4301-4.4.1-3 choice, but paths remain where NO entry is installed and the kernel delivers an unmatched packet: (a) the VPP backend holds no entry bound to no interface (vppPolicyInterface, vppBackend.CatchAllSupported, internal/component/ike/dataplane/vpp_policy.go), so under the default `unmatched bypass` it installs no catch-all and the config is not refused; (b) before the first configuration is applied and after the engine exits (applyConfig, internal/component/ike/engine/apply.go; removeUnmatched); (c) a bypass catch-all whose install fails, or a platform without XFRM, is logged and tolerated. `unmatched discard` is refused at verify on (a) and fails the apply on (c) (verifyUnmatchedEnforceable; TestUnmatchedDiscardRefusedAtVerifyWhereItCannotBeEnforced, TestUnmatchedDiscardInstallFailureFailsTheApply), and TestRFC4301InboundClearPacketMatchingNoEntryFollowsTheUnmatchedLeaf shows the installed catch-all deciding an inbound clear packet on XFRM |
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
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L175) | unit/verify | unproven |
| positive | [`TestIPsecSAPerDestinationWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L226) | unit/verify | revert, verified |

### [`RFC4301-4.1-2`](#rfc4301-4.1-2)

A security gateway MUST support tunnel mode (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestChildSAInstallsInDataplane asserts every installed policy is tunnel mode; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L190) | unit/verify | unproven |

### [`RFC4301-4.1-3`](#rfc4301-4.1-3)

Aside from the two exceptions below, whenever either end of a security association is a security gateway, the SA MUST be tunnel mode. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. every IKE peer SA is asserted modeTunnel; OSPFv3 transport SAs fall under the security-gateway-acting-as-host exception; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L177) | unit/verify | unproven |

### [`RFC4301-4.1-4`](#rfc4301-4.1-4)

IKE creates pairs of SAs, so for simplicity, we choose to require that both SAs in a pair be of the same mode, transport or tunnel. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. inbound and outbound SA of one IKE pair both asserted modeTunnel, so the pair is same-mode; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L179) | unit/verify | unproven |

### [`RFC4301-4.1-5`](#rfc4301-4.1-5)

To permit this, the IPsec implementation MUST permit establishment and maintenance of multiple SAs between a given sender and receiver, with the same selectors. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-5, so no unit is bound to it.

### [`RFC4301-4.2-1`](#rfc4301-4.2-1)

Note: A compliant implementation MUST NOT allow instantiation of an ESP SA that employs both NULL encryption and no integrity algorithm. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged: the OSPF unit TestIPsecESPRequiresIntegrity changed only by removing an RFC4303-1-1 negative comment; its assertions and both RFC4301-4.2-1 tags are unchanged, and its validateIPsecInterface revert records still replay. Re-judged pass 6. Both ESP instantiation paths now carry the MUST NOT. XFRM (xfrmStateFromParams, shared by IKE Child SAs and RFC 4552 OSPFv3 manual SAs): NULL + sha256 instantiated, NULL + ''/'none' refused with no state. ze_vpp (vppBackend.InstallSA): new negative TestRFC4301VPPESPNullEncryptionWithoutIntegrityIsRefused, control install of the well-formed SA sends 1 message, then NULL with '', 'none' and sha256 each returns ErrNotSupported and sends VPP nothing, and vppCryptoAlg('null') errors; the sha256 variant shows the refusal comes from the cipher mapping, not only from vppIntegAlg. No VPP positive exists or is owed: the row is a MUST NOT, and VPP refusing NULL encryption outright is compliant (the XFRM positive shows the boundary where NULL is allowed). OSPF config units (validateIPsecInterface) keep both polarities on the manual path; their revert records were observed red and written by this judge (they were the unproven covers). Revert records: xfrmStateFromParams +/-, vppCryptoAlg -, validateIPsecInterface +/-. Ran with -tags ze_vpp -race: green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301ESPNullEncryptionWithoutIntegrityIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_esp_null_linux_test.go#L36) | unit/verify | revert, verified |
| negative | [`TestRFC4301VPPESPNullEncryptionWithoutIntegrityIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_esp_null_vpp_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L102) | unit/verify | revert, verified |
| positive | [`TestRFC4301ESPNullEncryptionWithIntegrityIsInstantiated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_esp_null_linux_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L121) | unit/verify | revert, verified |

### [`RFC4301-4.4.1-1`](#rfc4301-4.4.1-1)

An IPsec implementation MUST have at least one SPD, and it MAY support multiple SPDs, if appropriate for the context in which the IPsec implementation operates. (§4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Child SA install asserts two PROTECT policies reach the SPD; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L156) | unit/verify | unproven |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged at the whole stack (R21). Transport: installChildSA now sets the inbound SAParams.Sel from childPolicyParams(child, SADirIn) (src=TSr, dst=TSi, proto, sport/dport, direction checked with asymmetric ports 179/any in TestRFC4301InboundSADEntryCarriesTheNegotiatedSelectors), mapped to x->sel by xfrmStateFromParams. Tunnel: the inbound require-policy carries the same five. Kernel probes in rfc4301_sad_selector_linux_test.go (user+net namespace, real XFRM) assert delivery inside and exact +1 on XfrmInNoPols (tunnel: outside by source, port, protocol) or XfrmInStateMismatch (transport: port, protocol) with no delivery. Judge re-observed red under two overlays: transport Sel block disabled -> port 6000 delivered, counters 0; inbound tunnel policy widened -> all three outside cases delivered, NoPols 0. The multi-pair 'or values' half is the gap RFC4301-4.4.2-2. Re-judged 2026-09-30 (pass 3): the tagged units changed only by tag lines (RFC4301-4.4-1 and 4.4.2.1-1 lines deleted from the dataplane boundary units, RFC4301-5.2-2 lines added to the engine sad_selector units); bodies identical, verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301TransportModeInboundSADEntryDropsPacketsOutsideItsSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L403) | unit/verify | revert, verified |
| negative | [`TestRFC4301TunnelModeInboundSelectorsAreEnforcedByTheRequirePolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L348) | unit/verify | revert, verified |
| positive | [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L418) | unit/verify | revert, verified |
| positive | [`TestRFC4301TransportModeInboundSADEntryDropsPacketsOutsideItsSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L401) | unit/verify | revert, verified |
| positive | [`TestRFC4301TunnelModeInboundSelectorsAreEnforcedByTheRequirePolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L345) | unit/verify | revert, verified |
| positive | [`TestRFC4301InboundSADEntryCarriesTheNegotiatedSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestNarrowedSelectorsReachTheInstalledPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L423) | unit/verify | revert, verified |

### [`RFC4301-4.4.2-2`](#rfc4301-4.4.2-2)

the entry for an inbound SA in the SAD MUST be initially populated with the value or values negotiated at the time the SA was created. (§4.4.2)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent source rejudgment against full RFC 4301 Section 4.4.2: ‘For each of the selectors defined in Section 4.4.1.1, the entry for an inbound SA in the SAD MUST be initially populated with the value or values negotiated at the time the SA was created.’ The assigned row isolates the plural-values obligation and retains its applicable gap. internal/component/ike/engine/ts_narrow.go records all accepted pairs in NegotiatedPairs but only pair zero in NegotiatedTSi/NegotiatedTSr. createFirstChildSA retains the pair list while deriving TSLocal/TSRemote from those first prefixes. In internal/component/ike/engine/child.go, selectorProto and selectorPort consume Selectors[0] only; childPolicyParams builds one policy per direction from that protocol/port pair and TSLocal/TSRemote. installChildSA installs one inbound policy and copies its selector onto a transport-mode inbound SA; the tunnel-mode state has no selector and delegates matching to that one require-policy. Additional negotiated values therefore remain absent from the installed selector representation. No tests tag this gap row; neighboring single-selector proofs do not establish the missing plurality. The enum edit preserves ANY/zero, exact-port and OPAQUE outcomes and adds a panic for unknown forms; it neither creates nor remedies this existing gap. No implementation expansion or runtime verification performed.

No test carries RFC4301-4.4.2-2, so no unit is bound to it.

### [`RFC4301-4.5-1`](#rfc4301-4.5-1)

All IPsec implementations MUST support both manual and automated SA and cryptographic key management. (§4.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. automated half: createFirstChildSA installs IKE-keyed SAs; manual half: OSPFv3 installs static SPI+key SAs; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L150) | unit/verify | unproven |
| positive | [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L110) | unit/verify | mutant, verified |

### [`RFC4301-5.2-1`](#rfc4301-5.2-1)

Since the SPD-I is just a part of the SPD, if a packet that is looked up in the SPD-I cannot be matched to an entry there, then the packet MUST be discarded. (§4.4.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-09-30 on the R29-a fail-closed code (RULINGS R28). RFC 4301 Section 4.4.1 text read: 'if a packet that is looked up in the SPD-I cannot be matched to an entry there, then the packet MUST be discarded.' Where installUnmatched has installed the catch-all at PriorityUnmatched every inbound packet meets an SPD-I entry, but paths with NO entry remain and were each verified at the producer: (a) VPP: vppPolicyInterface refuses IfIndex 0 with ErrNotSupported, vppBackend.CatchAllSupported refuses, and installUnmatched under the default bypass logs and returns nil, so no catch-all and the config is accepted; (b) before the first apply (applyConfig is the only caller) and after exit (register.go removeUnmatched when unmatchedApplied); (c) a bypass install failure is logged (Warn) and an XFRM-unsupported error returns nil. `unmatched discard` is refused at verify on (a) and fails the apply on an install error (untagged TestUnmatchedDiscardRefusedAtVerifyWhereItCannotBeEnforced, TestUnmatchedDiscardInstallFailureFailsTheApply, judge ran both: PASS). Under the default bypass the catch-all delivers the packet (the R27 deviation). Row {gap} and Support remaining name all three paths. Not counted conformant.

No test carries RFC4301-5.2-1, so no unit is bound to it.

### [`RFC4301-5.2-2`](#rfc4301-5.2-2)

Then match the packet against the inbound selectors identified by the SAD entry to verify that the received packet is appropriate for the SA via which it was received. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Whole stack over real XFRM in a rootless netns (R23: the kernel performs the check on the state and policy Ze installs). Tunnel: an inner packet inside the negotiated selectors is delivered; outside by source, port or protocol each moves XfrmInNoPols by 1 and is not delivered. Transport: inside delivered; outside by port or protocol each moves XfrmInStateMismatch by 1. Records observed red on childPolicyParams and installChildSA. Re-judged 2026-09-30 (leftovers): the retired lead-in RFC4301-5.2-3's tags moved here onto the dataplane SPD boundary units, supplementary builder-level cover of the step 4 selector check's input: positive asserts the inbound entry's in direction, the outbound entry's source prefix, exact source port 443 and one template; negative asserts the inexpressible port mask is refused for SADirIn (and SADirOut). The RFC names the selectors 'identified by the SAD entry'; on Linux that check is the inbound require-policy plus template match against the SA, so the policy selector is the right boundary. Records: xfrmPolicyFromParams panic revert observed red both polarities. Judge ran both: PASS.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L193) | unit/verify | revert, verified |
| negative | [`TestRFC4301TransportModeInboundSADEntryDropsPacketsOutsideItsSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L408) | unit/verify | revert, verified |
| negative | [`TestRFC4301TunnelModeInboundSelectorsAreEnforcedByTheRequirePolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L353) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestRFC4301TransportModeInboundSADEntryDropsPacketsOutsideItsSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L406) | unit/verify | revert, verified |
| positive | [`TestRFC4301TunnelModeInboundSelectorsAreEnforcedByTheRequirePolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_selector_linux_test.go#L351) | unit/verify | revert, verified |

### [`RFC4301-4.1-6`](#rfc4301-4.1-6)

If an IPsec implementation supports multicast, then it MUST support multicast SAs using the algorithm below for mapping inbound IPsec datagrams to SAs. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged on the path where Ze has multicast SAs (R24): the RFC 4552 manual OSPFv3 installer (buildIPsecInterfaceSAs/buildIPsecSA) run through onInterfaceUp over the real XFRM backend in a rootless user+net namespace; the lookup is netlink XFRM_MSG_GETSA against the kernel SAD (xfrm_state_lookup, the function XFRM input uses), not a mock (run by the judge 2026-09-30: child ran, not skipped). Positive: ff02::5 and ff02::6 share one SPI and each (group, SPI) lookup returns that group's own SA, i.e. step 2 (SPI + destination) of the section 4.1 search resolves an Any-Source Multicast group SA. Negative: after a control lookup, ff02::5 under an unconfigured SPI and the configured SPI to ff02::1 map to no SA (step 3 fails, packet has no SA). Step 1 (SPI+dst+src) is vacuous: Ze builds no source-specific multicast SA. Caveat recorded, not a finding against the tests: the ff02::1 case rests on Linux identifying the fe80:: unicast SA by (dst, SPI, proto) rather than the section 4.1 SPI(+protocol) identifier, so a literal step 3 would match that unicast SA; the stray-SPI case alone carries the negative without that reading. Records: revert buildIPsecSA, both polarities observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301MulticastSALookupRefusesAnUnconfiguredSPIOrGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4301_multicast_sa_linux_test.go#L152) | unit/verify | revert, verified |
| positive | [`TestRFC4301MulticastSAsAreMappedBySPIAndGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4301_multicast_sa_linux_test.go#L126) | unit/verify | revert, verified |

### [`RFC4301-7-1`](#rfc4301-7-1)

Note: AH and ESP cannot be applied using transport mode to IPv4 packets that are fragments. Only tunnel mode can be employed in such cases. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-7-1, so no unit is bound to it.

### [`RFC4301-4.4.1-3`](#rfc4301-4.4.1-3)

Every SPD SHOULD have a nominal, final entry that matches anything that is otherwise unmatched, and discards it. (§4.4.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-09-30 after installUnmatched gained an error return (R29-a). Unchanged finding: disclosed deviation (R27, owner gate pending), with the default configuration the final SPD entry is a BYPASS, not the discard the SHOULD asks for. parseUnmatched returns SPActionBypass for an absent `unmatched` leaf and installUnmatched installs it at PriorityUnmatched in 3 directions x 2 families. The gap unit TestRFC4301DefaultFinalSPDEntryDiscards (rfcgap.Demonstrate) runs config -> ParseIPsecConfig -> installUnmatched (finalEntriesFor now fails on an install error) and asserts every final entry discards; it fails today, so the gap is demonstrated. `unmatched discard` installs the final discard, and since R29-a a discard that cannot be installed fails the apply or is refused at verify. Row, Support remaining and docs/guide/ipsec.md disclose it. Not counted conformant.

No test carries RFC4301-4.4.1-3, so no unit is bound to it.

### [`RFC4301-4.4.3.1-1`](#rfc4301-4.4.3.1-1)

The specific syntax used by an implementation to accommodate sub-tree matching for distinguished names, domain names or RFC 822 e-mail addresses is a local matter. But, at a minimum, sub-tree matching of the sort described above MUST be supported. (§4.4.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. sub-tree entries admit DNS, RFC 822 and DN peers beneath them and refuse near misses, apex and outside peers; certificate binding kept

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPadKeyIDStaysExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L111) | unit/verify | unproven |
| negative | [`TestPadSubtreeRefusesAPeerOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L69) | unit/verify | unproven |
| positive | [`TestPadSubtreeAdmitsAPeerBeneathIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L35) | unit/verify | unproven |
| positive | [`TestPadSubtreeStillBindsTheCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L185) | unit/verify | unproven |
| positive | [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_validate_identity_test.go#L166) | unit/verify | unproven |

### [`RFC4301-4.4.3.1-2`](#rfc4301-4.4.3.1-2)

For IPv4 and IPv6 addresses, the same address range syntax used for SPD entries MUST be supported. (§4.4.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. prefix PAD entries admit addresses inside for v4 and v6 and refuse outside, other family and text spellings; the same prefix syntax the SPD uses

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPadAddressRangeRefusesWhatIsOutsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L155) | unit/verify | unproven |
| positive | [`TestPadAddressRangeAdmitsAnAddressInsideIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_pad_subtree_test.go#L129) | unit/verify | unproven |
| positive | [`TestVidPeerAuthorizationSetsCommitOnlyAsRemoteID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_validate_identity_test.go#L167) | unit/verify | unproven |

### [`RFC4301-4.4.1-4`](#rfc4301-4.4.1-4)

Thus, a user or administrator MUST be able to order the entries to express a desired access control policy. (§4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestSpdOperatorOrdersOverlappingPeers asserts the operator's priority reaches every entry in both directions and orders overlapping peers; negatives refuse an unstated order reaching the SPD as 0 and ranks that capture IKE (engine and ipsec). The mistagged mirroring unit (TestSPDPolicyMirrorsTheInboundSelector) moved to RFC4301-4.4.1-11/12.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSpdOrderCannotCaptureTheIKEControlPlane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L107) | unit/verify | unproven |
| negative | [`TestSpdUnstatedOrderTakesTheDefaultRank`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L67) | unit/verify | unproven |
| negative | [`TestSPDPolicyOrderInsideBypassBandRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L123) | unit/verify | revert, verified |
| positive | [`TestSpdOperatorOrdersOverlappingPeers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_order_test.go#L20) | unit/verify | revert, verified |

### [`RFC4301-7.4-1`](#rfc4301-7.4-1)

All implementations MUST support DISCARDing of fragments using the normal SPD packet classification mechanisms (§7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBypassPolicyIsNotBlocked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L64) | unit/verify | revert, verified |
| negative | [`TestBypassPolicyReachesTheBackendAsBypass`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L75) | unit/verify | revert, verified |
| negative | [`TestSPDPolicyBypassIsNotADiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_discard_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestDiscardPolicyReachesTheKernelAsBlock`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestPolicyReadbackReportsDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_discard_linux_test.go#L101) | unit/verify | revert, verified |
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
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L189) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L128) | unit/verify | revert, verified |

### [`RFC4301-3.2-1`](#rfc4301-3.2-1)

IPsec implementations MUST support ESP and MAY support AH (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4301ChildSAIsInstalledAsESP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_esp_test.go#L19) | unit/verify | revert, verified |

### [`RFC4301-4-1`](#rfc4301-4-1)

All implementations of AH or ESP MUST support the concept of an SA as described below (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L69) | unit/verify | revert, verified |

### [`RFC4301-4.1-12`](#rfc4301-4.1-12)

The indication of whether source and destination address matching is required to map inbound IPsec traffic to SAs MUST be set either as a side effect of manual SA configuration or via negotiation using an SA management protocol, e.g., IKE or Group Domain of Interpretation (GDOI) [RFC3547]. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged pass 6. The unit now asserts what Linux actually reads for inbound source matching on a transport-mode SA: the state selector x->sel (xfrm_selector_match in __xfrm_policy_check), read back from the kernel for the ff02::5 and ff02::6 states found by (group, SPI), with source ::/0; a control AH state installed with selector source fe80::2/128 is read back first as a /128, so the read-back discriminates a pinned source from the wildcard. The props-source :: assertion is kept and correctly described as the outbound address check, not claimed. Doc comment, PREVENTS line and failure text now name the right mechanism. Producer read: buildIPsecSA sets Sel.Src to ospfWildcardNet(). Each probe runs in its own user+net namespace (no cross-test state). Revert record of buildIPsecSA observed red; author's overlay pinning Sel.Src also went red. single-polarity holds: multicast SAs exist only as RFC 4552 manual config, which always sets the indication, so no refusal path exists. Full ospf package -race green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4301MulticastSASourceMatchingIsSetByManualConfiguration`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4301_multicast_sa_linux_test.go#L180) | unit/verify | revert, verified |

### [`RFC4301-4.1-9`](#rfc4301-4.1-9)

The receiver MUST process the packets from the different SAs without prejudice. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 under owner ruling 5: judged on the positive. TestRFC4301ParallelSAsWithOneSelectorAreEachReceived installs two Child SAs on one selector pair through createFirstChildSA over real XFRM in a rootless netns (re-exec child) and sends an in-selector packet on SA1, SA2, then SA1 again, asserting each is delivered; a per-SA template/reqid mismatch or shared replay state that dropped the sibling's packets goes red. No receive-side injection of prejudiced processing exists, so the negatives (TestRFC4301ParallelSAsAreHeldToTheOneSharedPolicy, which proves the 4.4.1-10 post-decryption SPD check, and the dataplane SAD boundary units, which prove the 4.1-5 permission) stay supplementary per the BFD 6.1-3 precedent. All four units hold observed-red revert records.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L113) | unit/verify | revert, verified |
| negative | [`TestRFC4301ParallelSAsAreHeldToTheOneSharedPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_parallel_sa_linux_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestRFC4301ParallelSAsWithOneSelectorAreEachReceived`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_parallel_sa_linux_test.go#L81) | unit/verify | revert, verified |

### [`RFC4301-4.1-10`](#rfc4301-4.1-10)

In transport mode, the DSCP value might change en route, but this should not cause problems with respect to IPsec processing since the value is not employed for SA selection and MUST NOT be checked as part of SA/packet validation. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-10, so no unit is bound to it.

### [`RFC4301-4.1-11`](#rfc4301-4.1-11)

In IPv6, the security protocol header appears after the base IP header and selected extension headers, but may appear before or after destination options; it MUST appear before next layer protocols (e.g., TCP, UDP, Stream Control Transmission Protocol (SCTP)). (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.1-11, so no unit is bound to it.

### [`RFC4301-4.4.1-5`](#rfc4301-4.4.1-5)

However, if an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing. (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1-5, so no unit is bound to it.

### [`RFC4301-4.4.1-6`](#rfc4301-4.4.1-6)

The SPD MUST permit a user or administrator to specify policy entries as follows: (§4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged pass 6. The SPD-S clause (RFC 4301 4.4.1: selectors, controls on how to create SAs, and the parameters needed to effect the protection) is now proven: TestRFC4301SPDSEntryCarriesSelectorsSAControlsAndProtection writes a site-to-site peer through ParseIPsecConfig + ValidateGroupRefs and reads back the traffic-selector prefixes, connection-type/remote-address/esp-group, and mode transport with proposal aes256/sha256. The negative refuses an undefined esp-group (by ValidateGroupRefs), mode beet, an unknown connection-type and a /33 selector, each naming the part: an entry is never accepted with a part silently dropped or replaced, which is the form in which an SPD fails to let the administrator specify it. SPD-I/SPD-O units at HEAD unchanged. Revert records parseSiteToSitePeer +/- and parseSPDPolicy +/- observed red. The undefined esp-group case is refused outside the recorded producer; the other three cases redden through it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SPDRefusesADispositionItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L70) | unit/verify | revert, verified |
| negative | [`TestRFC4301SPDSEntryRefusesAMissingOrUnusablePart`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_s_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestRFC4301SPDPermitsEachDispositionInEachDatabase`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC4301SPDSEntryCarriesSelectorsSAControlsAndProtection`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_s_test.go#L58) | unit/verify | revert, verified |

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

### [`RFC4301-4.4.1-10`](#rfc4301-4.4.1-10)

If the SPD is not decorrelated, caching is not allowed and an ordered search of SPD MUST be performed to verify that inbound traffic arriving on an SA is consistent with the access control policy expressed in the SPD. (§4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Ordered search proven at the stack, both ways, over real XFRM in a rootless netns with two overlapping inbound entries whose kernel priorities the tests read back. Positive TestRFC4301OrderedInboundSearchAdmitsOnTheFirstRankedEntry: Child SA entry (priority 2000, allow) over the inbound catch-all DISCARD (2^32-1, block); an in-selector packet on the SA is delivered with XfrmInPolBlock +0, an out-of-selector packet meets only the catch-all and is dropped (+1). Negative TestRFC4301OrderedInboundSearchRefusesOnTheFirstRankedDiscard: an operator inbound DISCARD (spdPolicyParams, order 1000, read back as 1000 block) covering the Child SA selector drops an in-selector packet on the SA (XfrmInPolBlock +1, not delivered) although the later PROTECT template matches. Also: TestRFC4301ParallelSAs* (verification after decryption, XfrmInNoPols) and the SPD boundary units (priority carried). Minor: orderedInboundPriority returns the first kernel policy with the src/dst strings, which in the negative could be the Child SA entry; the Fatalf then fails loud, never passes wrongly. Re-judged 2026-09-30 (pass 3): the tagged units changed only by tag lines (RFC4301-4.4-1 and 4.4.2.1-1 lines deleted from the dataplane boundary units, RFC4301-5.2-2 lines added to the engine sad_selector units); bodies identical, verdict unchanged. Re-judged 2026-09-30 (leftovers): the two dataplane SPD boundary units changed only by tag lines (RFC4301-5.1-1 and RFC4301-5.2-3 lines, rows retired as lead-ins, moved to RFC4301-5.1-4 and RFC4301-5.2-2); bodies identical to HEAD, judge ran both: PASS; verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L190) | unit/verify | revert, verified |
| negative | [`TestRFC4301OrderedInboundSearchRefusesOnTheFirstRankedDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ordered_spd_linux_test.go#L140) | unit/verify | revert, verified |
| negative | [`TestRFC4301ParallelSAsAreHeldToTheOneSharedPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_parallel_sa_linux_test.go#L111) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L129) | unit/verify | revert, verified |
| positive | [`TestRFC4301OrderedInboundSearchAdmitsOnTheFirstRankedEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ordered_spd_linux_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestRFC4301ParallelSAsWithOneSelectorAreEachReceived`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_parallel_sa_linux_test.go#L84) | unit/verify | revert, verified |

### [`RFC4301-4.4.1-11`](#rfc4301-4.4.1-11)

SPD-I: For inbound traffic that is to be bypassed or discarded, the entry consists of the values of the selectors that apply to the traffic to be bypassed or discarded. (§4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment against full RFC 4301 Section 4.4.1. The controlling lead-in is: ‘The SPD MUST permit a user or administrator to specify policy entries as follows:’ The complete assigned item is: ‘SPD-I: For inbound traffic that is to be bypassed or discarded, the entry consists of the values of the selectors that apply to the traffic to be bypassed or discarded.’ Read all three tagged units: internal/component/ike/engine/rfc4301_spd_discard_test.go::TestSPDPolicyMirrorsTheInboundSelector and internal/component/ike/engine/rfc4301_bypass_return_test.go::TestRFC4301BypassReturnTrafficHasAnSPDIEntry / TestRFC4301BypassSPDIEntryIsNeverAnUnmirroredCopy. The DISCARD case requires an inbound policy and exact remote-source/local-destination prefixes. The BYPASS positive additionally pins BYPASS and asymmetric source-any/destination-22 ports. Its distinct negative rejects outbound orientation and an inbound policy for an out-only entry. internal/component/ike/engine/spd_policy.go::spdPolicyParams selects directions, spdPolicyDirection reverses addresses and ports, and installSPDPolicies consumes the results through dp.InstallPolicy. The owned edit adds an unknown-direction panic to the test without weakening assertions or changing either tag claim. This is unit-level projection evidence, not execution of config commit or kernel packet delivery. The DISCARD unit itself does not assert action, protocol, ports or exact multiplicity, so those checks are not attributed to it. The canonical scope report marks its positive discrimination record unit-changed; parent must obtain a separate native recorder observation for this cover. No new execution is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BypassSPDIEntryIsNeverAnUnmirroredCopy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC4301BypassReturnTrafficHasAnSPDIEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestSPDPolicyMirrorsTheInboundSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L94) | unit/verify | revert, verified |

### [`RFC4301-4.4.1-12`](#rfc4301-4.4.1-12)

SPD-O: For outbound traffic that is to be bypassed or discarded, the entry consists of the values of the selectors that apply to the traffic to be bypassed or discarded. (§4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment against full RFC 4301 Section 4.4.1. The controlling lead-in is: ‘The SPD MUST permit a user or administrator to specify policy entries as follows:’ The complete assigned item is: ‘SPD-O: For outbound traffic that is to be bypassed or discarded, the entry consists of the values of the selectors that apply to the traffic to be bypassed or discarded.’ Read all three tagged units: internal/component/ike/engine/rfc4301_spd_discard_test.go::TestSPDPolicyMirrorsTheInboundSelector and internal/component/ike/engine/rfc4301_bypass_return_test.go::TestRFC4301InboundBypassReturnTrafficHasAnSPDOEntry / TestRFC4301BypassSPDOEntryIsNeverAnUnmirroredCopy. The DISCARD case requires an outbound policy and exact local-source/remote-destination prefixes. The BYPASS positive additionally checks BYPASS and source-22/destination-any ports. Its distinct negative rejects inbound orientation and an outbound policy for an in-only entry. internal/component/ike/engine/spd_policy.go::spdPolicyParams selects directions, spdPolicyDirection retains outbound orientation, and installSPDPolicies forwards each result to dp.InstallPolicy. The owned unknown-direction panic strengthens failure handling without changing either tag claim. These tests inspect builder results, not kernel installation or packet delivery; the DISCARD unit alone does not assert action, protocol, ports or exact multiplicity. Parent must renew this behavior-changed unit’s positive record for this id separately from the -11 record. No test or recorder was executed by this judge.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BypassSPDOEntryIsNeverAnUnmirroredCopy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestRFC4301InboundBypassReturnTrafficHasAnSPDOEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_bypass_return_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestSPDPolicyMirrorsTheInboundSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_spd_discard_test.go#L95) | unit/verify | revert, verified |

### [`RFC4301-4.4.1.1-2`](#rfc4301-4.4.1.1-2)

Note that the Local and Remote ports may not be available in the case of receipt of a fragmented packet or if the port fields have been protected by IPsec (encrypted); thus, a value of OPAQUE also MUST be supported. (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-2, so no unit is bound to it.

### [`RFC4301-4.4.1.1-3`](#rfc4301-4.4.1.1-3)

If the SA requires a port value other than ANY or OPAQUE, an arriving fragment without ports MUST be discarded (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L191) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L130) | unit/verify | revert, verified |

### [`RFC4301-4.4.1.1-4`](#rfc4301-4.4.1.1-4)

Given a policy entry with a range of Types (T-start to T-end) and a range of Codes (C-start to C-end), and an ICMP packet with Type t and Code c, an implementation MUST test for a match using (T-start*256) + C-start <= (t*256) + c <= (T-end*256) + C-end (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-4, so no unit is bound to it.

### [`RFC4301-4.4.1.1-5`](#rfc4301-4.4.1.1-5)

The name used to match this field is communicated during the IKE negotiation in the ID payload. In this context, the initiator's Source IP address (inner IP header in tunnel mode) is bound to the Remote IP address in the SAD entry created by the IKE negotiation. This address overrides the Remote IP address value in the SPD, when the SPD entry is selected in this fashion. All IPsec implementations MUST support this use of names. (§4.4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.1-5, so no unit is bound to it.

### [`RFC4301-4.4.1.2-1`](#rfc4301-4.4.1.2-1)

One to N selector sets that correspond to the "condition" for applying a particular IPsec action. (§4.4.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment against complete RFC 4301 Section 4.4.1.2: ‘An SPD is an ordered list of entries each of which contains the following fields.’ The assigned complete item is: ‘One to N selector sets that correspond to the "condition" for applying a particular IPsec action.’ internal/component/ike/ipsec/config.go::ParseIPsecConfig calls parseSPDPolicies, and internal/component/ike/ipsec/spd_policy.go::parseSPDPolicy constructs one SPDPolicy with one local/remote prefix, one protocol and one port pair. N=1 satisfies this structural item; this does not prove the separately allocated users-MUST-NOT rule or multi-selector negotiation. Read both tagged units and their helpers in internal/component/ike/ipsec/rfc4301_selector_set_test.go. TestRFC4301SPDEntryHoldsOneSelectorSet parses an actual config tree and checks both exact prefixes, protocol 17, local port value 53 and remote ANY. TestRFC4301SPDEntryCannotCarryASecondSelectorSet checks the returned model for selector-bearing collections and exactly four scalar prefix/port fields. The latter is a structural guard, not an exhaustive validator of hypothetical future nested models. The single-positive annotation is justified for the one-set representation, which has no selectable zero-set/multiple-set cardinality; it does not mean malformed configurations have no refusal paths. The enum-owned edits are ordinary exhaustive comments in the test and holdsSelector, outside the tag claim. They stale the audit unit but do not themselves change discrimination behavior or claim hashes; the canonical scope report records the tagged comment-only cover as verified. No new execution or baseline-freshness claim.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4301SPDEntryCannotCarryASecondSelectorSet`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_selector_set_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestRFC4301SPDEntryHoldsOneSelectorSet`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_selector_set_test.go#L88) | unit/verify | revert, verified |

### [`RFC4301-4.4.1.2-2`](#rfc4301-4.4.1.2-2)

Until IKE provides a facility that conveys the semantics that are expressed in the SPD via selector sets (as described below), users MUST NOT include multiple selector sets in a single SPD entry unless the access control intent aligns with the IKE "mix and match" semantics. (§4.4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.1.2-2, so no unit is bound to it.

### [`RFC4301-4.4.2.1-1`](#rfc4301-4.4.2.1-1)

Security Parameter Index (SPI): a 32-bit value selected by the receiving end of an SA to uniquely identify the SA. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Row now quotes the SPI item alone (split, Correction 2026-09-30). Positive TestRFC4301SADItemSPIIsTheNegotiatedValue: inbound and outbound SAD entries carry the SPI given, unchanged. Negative TestRFC4301SADItemZeroSPIIsRefused: SPI 0 (RFC 4303 section 2.1: reserved, MUST NOT be sent on the wire, so it identifies no SA) builds no state; the refusal is the D-8 fix in xfrmStateFromParams. Unit-level over the builder; both records observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SADItemZeroSPIIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestRFC4301SADItemSPIIsTheNegotiatedValue`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L23) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-4`](#rfc4301-4.4.2.1-4)

Sequence Number Counter: a 64-bit counter used to generate the Sequence Number field in AH or ESP headers. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive (justified: Ze negotiates ESN 'not extended' only, which the item permits 'if negotiated', so no input selects another width). The SAD entry carries no ESN flag and no preset replay state. Builder-level.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4301SADItemSequenceCounterIsThe32BitCounterNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L51) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-5`](#rfc4301-4.4.2.1-5)

Sequence Counter Overflow: a flag indicating whether overflow of the sequence number counter should generate an auditable event and prevent transmission of additional packets on the SA, or whether rollover is permitted. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive accepted: no config leaf or negotiation offers rollover or ESN. The unit builds ESP and AH SAs in both directions through xfrmStateFromParams and asserts OSeqMayWrap and ESN unset, so the kernel (xfrm_replay_overflow) audits the overflow and stops transmission. Would go red if the producer set rollover. Record: revert xfrmStateFromParams, observed red. Judge finding (not fixed): the stop-not-rollover decision is the zero value of netlink.XfrmState.OSeqMayWrap and xfrmStateFromParams never names it; under the principles rule a zero another component relies on is a guard that must be named (explicit assignment with the RFC quote). The test pins it, so it cannot vanish silently, but the producer still owes the named guard.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4301SADItemSequenceCounterOverflowIsNeverRollover`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L279) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-6`](#rfc4301-4.4.2.1-6)

Anti-Replay Window: a 64-bit counter and a bit-map (or equivalent) used to determine whether an inbound AH or ESP packet is a replay. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive (justified: every window is installed as given, zero included, nothing to refuse). Windows 32 and 64 read back from the SAD entry. Builder-level; zero is not asserted. Re-judged 2026-09-30 (pass 4): TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp added as a second positive (window 64 read back from the built SAD entry); record: revert xfrmStateFromParams, observed red. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestRFC4301SADItemAntiReplayWindowIsInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L67) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-7`](#rfc4301-4.4.2.1-7)

AH Authentication algorithm, key, etc. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. AH in scope (OSPFv3, RFC 4552). Positive: AH entry carries integrity algorithm, 128-bit truncation, the given key, no crypt/aead. Negative: unknown integrity algorithm refused, no state. Builder-level.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SADItemAHUnknownIntegrityIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestRFC4301SADItemAHCarriesItsIntegrityAlgorithmAndKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L85) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-8`](#rfc4301-4.4.2.1-8)

ESP Encryption algorithm, key, mode, IV, etc. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: ESP entry carries the encryption algorithm and the given key (distinct from the integrity key). Negative: unknown encryption algorithm refused, no state. Builder-level.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SADItemUnknownESPTransformIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L154) | unit/verify | revert, verified |
| positive | [`TestRFC4301SADItemESPTransformsCarryAlgorithmAndKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L127) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-9`](#rfc4301-4.4.2.1-9)

ESP integrity algorithm, keys, etc. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: ESP entry carries the integrity algorithm, truncation and the given key. Negative: unknown integrity algorithm refused, no state. Builder-level.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SADItemUnknownESPTransformIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L155) | unit/verify | revert, verified |
| positive | [`TestRFC4301SADItemESPTransformsCarryAlgorithmAndKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L128) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-10`](#rfc4301-4.4.2.1-10)

ESP combined mode algorithms, key(s), etc. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: AES-GCM entry carries the AEAD algorithm, ICV 128 and key, and neither separate transform. Negative: unknown combined-mode algorithm refused, no state. Builder-level.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SADItemUnknownCombinedModeIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L198) | unit/verify | revert, verified |
| positive | [`TestRFC4301SADItemCombinedModeCarriesOneAEADTransform`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L172) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-11`](#rfc4301-4.4.2.1-11)

Lifetime of this SA: a time interval after which an SA must be replaced with a new SA (and new SPI) or terminated, plus an indication of which of these actions should occur. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both halves of the row now proven. Interval: the pass-4 units (newLifetimeState, the producer established.go uses) prove hard = build + configured lifetime with soft (replace) before hard (terminate). Indication of which action: the owner-loop units drive the real maintainSA over one tick. Positive: soft past, hard far -> a CREATE_CHILD_SA request on the wire, loop alive, pendingRekey kind rekeyChild naming the old Child SA with a new non-zero inbound SPI (replace with a new SA and new SPI). Negative: soft far, hard past -> loop returns errTimeout, no pendingRekey, Child SA cleared, old inbound SPI removed from the dataplane, silence on the wire (terminate). Revert records on established.go::maintainSA observed red for both; the author's soft/hard swap overlay turned both red, so the pair discriminates the mapping, not only the loop's existence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SADLifetimeHardTimeTerminatesTheSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_lifetime_action_test.go#L81) | unit/verify | revert, verified |
| negative | [`TestRFC4301SADLifetimeTerminatesTheSAAtTheEndOfItsInterval`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_lifetime_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestRFC4301SADLifetimeSoftTimeReplacesTheSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_lifetime_action_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestRFC4301SADLifetimeIndicatesReplaceBeforeTerminate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_sad_lifetime_test.go#L23) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-12`](#rfc4301-4.4.2.1-12)

IPsec protocol mode: tunnel or transport. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: tunnel and transport each installed as their own kernel mode. Negative: a mode that is neither refused, no state. Builder-level.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SADItemUnknownModeIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L234) | unit/verify | revert, verified |
| positive | [`TestRFC4301SADItemModeIsInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L210) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-13`](#rfc4301-4.4.2.1-13)

Stateful fragment checking flag. (§4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.2.1-13, so no unit is bound to it.

### [`RFC4301-4.4.2.1-14`](#rfc4301-4.4.2.1-14)

Bypass DF bit (T/F) -- applicable to tunnel mode SAs where both inner and outer headers are IPv4. (§4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.2.1-14, so no unit is bound to it.

### [`RFC4301-4.4.2.1-15`](#rfc4301-4.4.2.1-15)

DSCP values -- the set of DSCP values allowed for packets carried over this SA. (§4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.2.1-15, so no unit is bound to it.

### [`RFC4301-4.4.2.1-16`](#rfc4301-4.4.2.1-16)

Bypass DSCP (T/F) or map to unprotected DSCP values (array) if needed to restrict bypass of DSCP values -- applicable to tunnel mode SAs. (§4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.2.1-16, so no unit is bound to it.

### [`RFC4301-4.4.2.1-17`](#rfc4301-4.4.2.1-17)

Path MTU: any observed path MTU and aging variables. (§4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.2.1-17, so no unit is bound to it.

### [`RFC4301-4.4.2.1-18`](#rfc4301-4.4.2.1-18)

Tunnel header IP source and destination address -- both addresses must be either IPv4 or IPv6 addresses. (§4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: IPv4 and IPv6 tunnel header pairs read back unchanged. Negative: IPv4 source with IPv6 destination refused, no state (D-8 fix in xfrmStateFromParams). No legitimate caller builds a mixed pair: engine SAs use the IKE endpoints, OSPF SAs are transport. Builder-level.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301SADItemMixedFamilyTunnelHeaderIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L265) | unit/verify | revert, verified |
| positive | [`TestRFC4301SADItemTunnelHeaderAddressesAreInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go#L246) | unit/verify | revert, verified |

### [`RFC4301-4.4.2.1-2`](#rfc4301-4.4.2.1-2)

A compliant implementation MUST support both types of lifetimes, and MUST support a simultaneous use of both. (§4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-4.4.2.1-2, so no unit is bound to it.

### [`RFC4301-4.4.2.1-3`](#rfc4301-4.4.2.1-3)

Note that implementations MUST be able to handle having the counters at the ends of an SA get out of synch, e.g., because of packet loss or because the implementations at each end of the SA aren't doing things the same way. (§4.4.2.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. the sentence concerns byte-count LIFETIME counters at the two ends getting out of synch (4.4.2.1 lifetime item (a)); the units assert the anti-replay window, a different mechanism Re-judged 2026-09-30 (pass 2): the two SAD boundary units changed only by the deleted RFC4301-4.1-8 tag lines (the 4.1-8 retirement was reversed by ruling R24: Ze has manual OSPFv3 multicast SAs, row restored as RFC4301-4.1-12 on the OSPF path); bodies identical, verdict unchanged. Re-judged 2026-09-30 (pass 3): the tagged units changed only by tag lines (RFC4301-4.4-1 and 4.4.2.1-1 lines deleted from the dataplane boundary units, RFC4301-5.2-2 lines added to the engine sad_selector units); bodies identical, verdict unchanged. Re-judged 2026-09-30 (pass 4): the HEAD tags are kept as they were (ruling R26) and an RFC4301-4.4.2.1-6 tag was added beside them on the positive unit; bodies identical. Still wrong: both units assert the anti-replay window, not lifetime counters out of synch; blocked by spec-ipsec-lifetime-volume AC-7, whose implementation replaces the proof.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L71) | unit/verify | revert, verified |

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
| negative | [`TestRFC4301UnmatchedNeverDiscardsWhatTheOperatorDidNotAskToDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_unmatched_test.go#L82) | unit/verify | revert, verified |
| negative | [`TestRFC4301UnmatchedLeafRefusesAnUnknownWord`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_unmatched_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestRFC4301UnmatchedDiscardIsTheLastEntryOfEveryDatabase`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_unmatched_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestRFC4301UnmatchedLeafIsReadAsWritten`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_unmatched_test.go#L23) | unit/verify | revert, verified |

### [`RFC4301-5.1-3`](#rfc4301-5.1-3)

With regard to determining and enforcing the PMTU of an SA, the IPsec system MUST follow the steps described in Section 8.2 (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.1-3, so no unit is bound to it.

### [`RFC4301-5.1-4`](#rfc4301-5.1-4)

Match the packet headers against the cache for the SPD specified by the SPD-ID from step 1. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Step 2 (match the outbound packet headers against the SPD). Linux keeps no SPD cache, so the match is the kernel SPD lookup over the entries Ze installs through createFirstChildSA, installSPDPolicies and installUnmatched. Positive: from one source, a datagram to 10.1.0.5 meets the Child SA PROTECT entry (ESP at the tunnel endpoint under child.OutboundSPI, XfrmOutNoStates unchanged) and one to 10.3.0.5:7001 meets the operator BYPASS entry (delivered clear, no ESP). Negative: 10.3.0.5 on port 7009, differing from the BYPASS entry only by port, is not bypassed: EPERM, XfrmOutPolBlock +1, not delivered, so the match reads the headers, not the destination alone. Judge ran both units in their own user+net namespace: PASS, not skipped. Records: xfrmPolicyFromParams panic revert red both polarities; author's manual overlays (bypass->BLOCK reddens the positive, discard->ALLOW reddens the negative, scratch p7-ov-*.log) confirm each polarity reads the disposition. Re-judged 2026-09-30 (leftovers): the retired lead-in RFC4301-5.1-1's tags moved here onto the dataplane SPD boundary units, as supplementary builder-level cover of step 2's input: the positive asserts the outbound entry's out direction, 10.0.0.0/24 -> 10.1.0.0/24 prefixes and exact source port 443; the negative asserts a 0x00ff port mask is refused (nil policy) for SADirOut and SADirIn rather than widened. Claims match the assertions. Records: xfrmPolicyFromParams panic revert observed red both polarities. Judge ran both: PASS. Stack proof stays the two engine units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L192) | unit/verify | revert, verified |
| negative | [`TestRFC4301OutboundPacketIsDiscardedByTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L238) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L131) | unit/verify | revert, verified |
| positive | [`TestRFC4301OutboundPacketTakesTheDispositionOfTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L189) | unit/verify | revert, verified |

### [`RFC4301-5.1-5`](#rfc4301-5.1-5)

If there is a match, then process the packet as specified by the matching cache entry, i.e., BYPASS, DISCARD, or PROTECT using AH or ESP. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Step 3a: process as the matching entry says, BYPASS, DISCARD or PROTECT. PROTECT: the datagram reaches the remote endpoint as ESP carrying the Child SA outbound SPI, no XfrmOutNoStates. BYPASS: delivered in the clear, and no ESP copy on the raw socket. DISCARD: operator DISCARD entry refuses the write with EPERM, XfrmOutPolBlock +1, not delivered. All three dispositions asserted on kernel counters and delivery; AH is not installed by IKE (ESP only), so the PROTECT case is ESP. Records: producer revert red both polarities; overlays discard->ALLOW and bypass->BLOCK each redden exactly the unit that asserts that disposition.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301OutboundPacketIsDiscardedByTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L241) | unit/verify | revert, verified |
| positive | [`TestRFC4301OutboundPacketTakesTheDispositionOfTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L192) | unit/verify | revert, verified |

### [`RFC4301-5.1-6`](#rfc4301-5.1-6)

If no match is found in the cache, search the SPD (SPD-S and SPD-O parts) specified by SPD-ID. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Step 3b SPD search. With no kernel SPD cache every packet, including the first of a flow, is looked up in the SPD (SPD-S: the Child SA PROTECT entry; SPD-O: operator BYPASS/DISCARD and the catch-all). Positive: first packets on the PROTECT and BYPASS entries take those entries' dispositions. Negative: a packet selecting no operator entry takes the catch-all's discard (EPERM, XfrmOutPolBlock +1), not a neighbouring entry's bypass. Claims name the cache absence rather than proving a cache. Records as 5.1-4.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301OutboundPacketIsDiscardedByTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L243) | unit/verify | revert, verified |
| positive | [`TestRFC4301OutboundPacketTakesTheDispositionOfTheEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go#L195) | unit/verify | revert, verified |

### [`RFC4301-5.1-7`](#rfc4301-5.1-7)

If the SPD entry calls for PROTECT, i.e., creation of an SA, the key management mechanism (e.g., IKEv2) is invoked to create the SA. If SA creation succeeds, a new outbound (SPD-S) cache entry is created, along with outbound and inbound SAD entries, otherwise the packet is discarded. (§5.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Step 3b PROTECT case: key management invoked by a packet. Checked at the producer: a PROTECT policy is installed only by installChildSA (engine/child.go, childPolicyParams) after both SAs of a negotiated Child SA are in the SAD; parseSPDPolicy (ipsec/spd_policy.go) refuses an operator protect entry; no Ze code subscribes to XFRM acquire (XfrmMonitor and XFRMNLGRP_ACQUIRE appear only in vendor/, the only NETLINK_XFRM users are request/response in xfrm_migrate_linux.go and kernelcap). So no outbound packet starts IKEv2. Gap disclosed on the row and in Support remaining; no spec schedules traffic-triggered SA creation.

No test carries RFC4301-5.1-7, so no unit is bound to it.

### [`RFC4301-5.1.2.1-1`](#rfc4301-5.1.2.1-1)

If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain (§5.1.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4301-5.1.2.1-1, so no unit is bound to it.

### [`RFC4301-5.2-4`](#rfc4301-5.2-4)

IKE traffic MUST have an explicit BYPASS entry in the SPD (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301IKEBypassIsNeverAProtectOrAWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ike_bypass_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC4301IKETrafficHasAnExplicitBypassEntry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_ike_bypass_test.go#L21) | unit/verify | revert, verified |

### [`RFC4301-5.2-6`](#rfc4301-5.2-6)

If the packet is addressed to the IPsec device and AH or ESP is specified as the protocol, the packet is looked up in the SAD. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Step 3a SAD lookup of an ESP packet addressed to Ze. Positive: ESP on the Child SA inbound SPI, sealed with the kernel key around an inner packet inside the selectors, is delivered with XfrmInNoStates and XfrmInStateProtoError unchanged. Negative: the same packet under SPI^0x5a5a5a5a finds nothing (XfrmInNoStates +1, not delivered), so the lookup is keyed on the SPI. Judge ran both: PASS in own namespace. Records: xfrmStateFromParams panic revert red both polarities. Scope: ESP, unicast; the AH case (OSPFv3 manual SAs) is the same kernel lookup and is not probed here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301InboundESPWithNoSADMatchIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L175) | unit/verify | revert, verified |
| positive | [`TestRFC4301InboundESPIsProcessedWithTheSAItsSPISelects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L142) | unit/verify | revert, verified |

### [`RFC4301-5.2-7`](#rfc4301-5.2-7)

In either case (unicast or multicast), if there is no match, discard the traffic. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Step 3a no-match discard. Negative: an ESP packet on an SPI no SA holds is counted XfrmInNoStates +1 and never delivered. Positive: the same packet on the installed SPI is delivered, so the discard is the SAD miss and not a blanket drop. Unicast probed here; the multicast lookup miss (SPI+group) is proven under RFC4301-4.1-6 (ospf rfc4301_multicast_sa_linux_test.go) as a kernel SAD lookup, not a delivered-packet probe. Records: xfrmStateFromParams revert red both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301InboundESPWithNoSADMatchIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L177) | unit/verify | revert, verified |
| positive | [`TestRFC4301InboundESPIsProcessedWithTheSAItsSPISelects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L144) | unit/verify | revert, verified |

### [`RFC4301-5.2-8`](#rfc4301-5.2-8)

If the packet is not addressed to the device or is addressed to this device and is not AH or ESP, look up the packet header in the (appropriate) SPD-I cache. If there is a match and the packet is to be discarded or bypassed, do so. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Step 3b SPD-I lookup of a clear packet addressed to Ze. Positive: UDP from 10.3.0.5 to local port 7101 matches the operator in-BYPASS entry ranked ahead of a DISCARD catch-all: delivered, no XfrmInPolBlock. Negative: to port 7102 on the operator in-DISCARD entry with a BYPASS catch-all: XfrmInPolBlock +1, not delivered, so only the matched entry can have dropped it. Each unit isolates its disposition by the opposite catch-all. Judge ran both: PASS. Records: xfrmPolicyFromParams revert red both; overlays bypass->BLOCK / discard->ALLOW redden exactly the matching polarity (scratch p7-ovin-*.log). Not-addressed-to-device (forwarded) packets are not probed; the SPD-I lookup is the same fwd/in policy check.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301InboundClearPacketIsDiscardedByTheSPDIEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L266) | unit/verify | revert, verified |
| positive | [`TestRFC4301InboundClearPacketIsBypassedByTheSPDIEntryItMatches`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L241) | unit/verify | revert, verified |

### [`RFC4301-5.2-9`](#rfc4301-5.2-9)

(No SAs are created in response to receipt of a packet that requires IPsec protection; only BYPASS or DISCARD cache entries can be created this way.) If there is no match, discard the traffic. (§5.2)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-09-30 on the R29-a fail-closed code (RULINGS R28). Step 3b 'If there is no match, discard the traffic.' Same producer walk as RFC4301-5.2-1: the no-entry paths (VPP with no node-wide SPD under the default bypass, before first apply and after exit, a tolerated bypass install failure or no XFRM) each leave an unmatched inbound packet delivered, and with the catch-all installed the default bypass delivers it too (R27 deviation). `unmatched discard` gives the RFC behaviour where enforceable and is refused or fails the apply where not (verifyUnmatchedEnforceable, installUnmatched). Row {gap} names the paths. Not counted conformant.

No test carries RFC4301-5.2-9, so no unit is bound to it.

### [`RFC4301-5.2-10`](#rfc4301-5.2-10)

Apply AH or ESP processing as specified, using the SAD entry selected in step 3a above. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Step 4 first sentence: ESP processing with the SAD entry step 3a selected. Positive: the packet on the Child SA SPI is decrypted with that SA's key and delivered, XfrmInStateProtoError unchanged. Negative: right SPI, fresh sequence number, sealed with a zero key: ICV fails, XfrmInStateProtoError +1, not delivered, so processing uses the selected SA's key and not a pass-through. Judge ran both: PASS. Records: xfrmStateFromParams revert red both. AH processing (OSPFv3) not probed here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301InboundESPFailingIntegrityIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L204) | unit/verify | revert, verified |
| positive | [`TestRFC4301InboundESPIsProcessedWithTheSAItsSPISelects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go#L146) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. The previous wrong verdict (bypass SPD entries create no SA) is cured by units on the SA path. Positive: a protocol-ANY/ports-ANY proposal narrows (empty and configured policy) to one pair, and childPolicyParams in and out carry UpperProto 0 with both ports ANY. Negative: protocol ANY with a single port (443) or OPAQUE, on TSi or on TSr, narrows to no pair, so no Child SA is built. The producer programmableSelector (Proto 0 with a non-ANY port is refused) is shared through programmablePair by the responder path (narrowSelectors) and by the initiator's answer-adoption loop in ts_narrow.go, so both SA-building paths hold; only the responder path is driven. Revert records observed red (child.go::selectorPort for +, ts_narrow.go::programmableSelector for -). The two ipsec SPD-entry units still tagged 7.1-3 are supplementary: they prove the operator entry side, which creates no SA; moving them to a row they prove is owed when the ratchet allows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301ProtocolAnyChildSARefusesAPort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_protocol_any_sa_test.go#L45) | unit/verify | revert, verified |
| negative | [`TestRFC4301ProtocolAnyEntryRefusesAPort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc4301_spd_management_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestRFC4301ProtocolAnyChildSACarriesAnyPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_protocol_any_sa_test.go#L19) | unit/verify | revert, verified |
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
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L194) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L133) | unit/verify | revert, verified |

### [`RFC4301-7.3-4`](#rfc4301-7.3-4)

However, a receiver MUST discard non-initial fragments that arrive on an SA with non-trivial port (or ICMP type/code or MH type) selector values unless this feature has been negotiated. (§7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a non-initial fragment on an SA with non-trivial port selectors (Ze has no NON_FIRST_FRAGMENTS_ALSO negotiation). Ze's boundary is the inbound XFRM policy the kernel checks; Linux decodes no ports for a non-initial fragment, so an exact port selector refuses it. TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate asserts in.SrcPort == 443 on the inbound entry, red if the selector is widened to ANY. TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened asserts a partial mask (0x00ff) is refused for SADirIn and SADirOut, red if it is installed widened. Re-judged 2026-09-30: the two SPD boundary units changed only by the deleted RFC4301-4.4.1-9 tag lines (row retired, decorrelation is optional and Ze never decorrelates); bodies identical, verdict unchanged. Re-judged 2026-09-30 (pass 3): the tagged units changed only by tag lines (RFC4301-4.4-1 and 4.4.2.1-1 lines deleted from the dataplane boundary units, RFC4301-5.2-2 lines added to the engine sad_selector units); bodies identical, verdict unchanged. Re-judged 2026-09-30 (leftovers): the two dataplane SPD boundary units changed only by tag lines (RFC4301-5.1-1 and RFC4301-5.2-3 lines, rows retired as lead-ins, moved to RFC4301-5.1-4 and RFC4301-5.2-2); bodies identical to HEAD, judge ran both: PASS; verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L195) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L134) | unit/verify | revert, verified |

### [`RFC4301-7.3-5`](#rfc4301-7.3-5)

Also, the receiver MUST discard non-initial fragments that do not comply with the security policy applied to the overall packet. (§7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Whole stack over real XFRM in a rootless netns, tunnel-mode Child SA on UDP 10.1.0.0/16 -> 10.2.0.0/16 port 5000. Positive TestRFC4301FragmentsOfACompliantPacketAreDelivered: an in-selector inner packet split into initial and non-initial fragments, each in its own ESP packet, non-initial first: reassembled and delivered, XfrmInNoPols +0. Negative TestRFC4301FragmentsOfANonCompliantPacketAreDiscarded: same with inner source outside the selector: the non-initial fragment is discarded with the packet (XfrmInNoPols +1, nothing delivered). Linux reassembles before the inbound policy check, so the discard is the stack acting on the require-policy Ze installs. The violating dimension is the source address, which the fragment itself carries; a port-only violation (the fragment-specific hazard) is covered by RFC4301-7.3-4. The SPD boundary tags stay as supplementary; their negative (bypass carries no template) proves a neighbour. Re-judged 2026-09-30 (pass 3): the tagged units changed only by tag lines (RFC4301-4.4-1 and 4.4.2.1-1 lines deleted from the dataplane boundary units, RFC4301-5.2-2 lines added to the engine sad_selector units); bodies identical, verdict unchanged. Re-judged 2026-09-30 (leftovers): the two dataplane SPD boundary units changed only by tag lines (RFC4301-5.1-1 and RFC4301-5.2-3 lines, rows retired as lead-ins, moved to RFC4301-5.1-4 and RFC4301-5.2-2); bodies identical to HEAD, judge ran both: PASS; verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4301BoundarySPDEntryIsRefusedRatherThanWidened`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L196) | unit/verify | revert, verified |
| negative | [`TestRFC4301FragmentsOfANonCompliantPacketAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_linux_test.go#L86) | unit/verify | revert, verified |
| positive | [`TestRFC4301BoundarySPDEntryCarriesSelectorDirectionOrderAndTemplate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4301_boundary_linux_test.go#L135) | unit/verify | revert, verified |
| positive | [`TestRFC4301FragmentsOfACompliantPacketAreDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4301_fragment_linux_test.go#L57) | unit/verify | revert, verified |

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
| Signed off | 2026-09-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc4301.txt |
| Source fingerprint | 263fd9a78175c888 |
| Record | rfc/extraction/rfc4301.json |
| Mapped sentences | 71 |
| Declined as scope | 18 |
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
| `4.4.2` | not stated | 2 | walked | Site 4.4.2:2 carries one sentence with two obligations: the inbound SAD entry holds the negotiated value of each selector (RFC4301-4.4.2-1, mapped), and it holds every negotiated value when the answer carries several traffic selector pairs. The second is split out as RFC4301-4.4.2-2, a gap, which has no site of its own and is recorded unsourced. |
| `4.4.2.1` | not stated | 3 | walked | Site 4.4.2.1:1, "The following data items MUST be in the SAD:", is a lead-in to sixteen bulleted items that carry no keyword of their own, so the extractor sees one site for sixteen obligations. RFC4301-4.4.2.1-1 keeps the site and quotes the first item (the SPI); the other fifteen items are rows of their own, each quoting its item, and have no site: they are recorded unsourced (2026-09-30, rfc/corrections/rfc4301.md). |
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
| `4.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A framing sentence: it states that the Section 4.4 model is nominal and that compliance is judged by its externally observable characteristics, each of which is an obligation of its own in the Section 4.4.x rows. The SPD characteristics are RFC4301-4.4.1-1, 4.4.1-4, 4.4.1-10, 4.4.1-11, 4.4.1-12 and the 4.4.1.1 selector rows; the SAD characteristics are RFC4301-4.4.2-1 and the 4.4.2.1 data item rows; the PAD characteristics are the 4.4.3.x rows. No behavior belongs to this sentence that those rows do not already state, so a row for it only restates them. Row RFC4301-4.4-1 retired 2026-09-30 (rfc/corrections/rfc4301.md, ruling R22). | The model described below is nominal; implementations need not match details of this model as presented, but the external behavior of implementations MUST correspond to the externally observable characteristics of this model in order to be compliant. |
| `4.4.1:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | a pointer sentence: it announces that the SPD elements enumerated in this section are mandatory, which is the obligation site 4.4.1:5 maps | However, this document does specify a standard set of SPD elements that all IPsec implementations MUST support. |
| `4.4.1:10` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Decorrelation is optional, and the obligation binds only an SPD entry that is decorrelated. Section 4.4.1 states: "This RFC does not require a compliant implementation to make use of decorrelation." Ze keeps the SPD ordered and never decorrelates it: xfrmPolicyFromParams (internal/component/ike/dataplane/xfrm_linux.go) writes each SPD entry as one kernel XFRM policy carrying its priority, and the kernel performs the ordered search, so no decorrelated group exists to link. | Note that when an SPD entry is decorrelated all the resulting entries MUST be linked together, so that all members of the group derived from an individual, SPD entry (prior to decorrelation) can all be placed into caches and into the SAD at the same time. |
| `4.4.1.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the sentence defers to Section 4.4.1.1 'Name' for the forms that must be supported, which is the name-selector obligation site 4.4.1.1:5 maps | The forms that MUST be supported are described above in Section 4.4.1.1 under "Name". o PFP flags -- one per traffic selector. |
| `4.4.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | a parenthetical cross-reference to the Section 4.1 inbound SA mapping algorithm that site 4.1:1 maps | (See Section 4.1 for details on the algorithm that MUST be used for mapping inbound IPsec datagrams to SAs.) The following parameters are associated with each entry in the SAD. |
| `5.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A lead-in: it points to the list of section 5.1 steps and states no behavior of its own. Its MUST binds each step, and each step with an obligation no other row carries is a row of its own, declared unsourced for section 5.1: RFC4301-5.1-4 (step 2, the header match), RFC4301-5.1-5 (step 3a, the matching entry's disposition), RFC4301-5.1-6 (step 3b, the SPD search) and RFC4301-5.1-7 (step 3b, the PROTECT case, a gap). Row RFC4301-5.1-1 retired 2026-09-30 and its tags moved to RFC4301-5.1-4 (rfc/corrections/rfc4301.md). | IPsec MUST perform the following steps when processing outbound packets: |
| `5.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Conditional on nested-SA processing: the sentence binds only a packet that the forwarding function 'may cause ... to be passed back across the IPsec boundary, for additional IPsec processing, e.g., in support of nested SAs' (section 5.1 step 4). Ze fills no such role: it configures no nested SAs, neither in Go nor through XFRM bundles (the XFRM policy builder in internal/component/ike/dataplane/xfrm_linux.go writes one template for a PROTECT policy and none for BYPASS, so no policy chains a second SA), so no packet of Ze's re-crosses the boundary for additional IPsec processing. Row RFC4301-5.1-2 retired 2026-09-29 (rfc/corrections/rfc4301.md, ruling R15). The SPD-I entry of a bypass entry is RFC4301-4.4.1-11. | If so, there MUST be an entry in SPD-I database that permits inbound bypassing of the packet, otherwise the packet will be discarded. |
| `5.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A lead-in: it points to the list of section 5.2 inbound steps and states no behavior of its own. Its MUST binds each step, and each step with an obligation no other row carries is a row of its own: RFC4301-5.2-4 (step 2, the IKE bypass entry), RFC4301-5.2-6 and RFC4301-5.2-7 (step 3a, the SAD lookup and its no-match discard), RFC4301-5.2-8 (step 3b, the SPD-I lookup), RFC4301-5.2-9 (step 3b, the no-match discard, a gap), RFC4301-5.2-10 (step 4, processing with the selected SA) and RFC4301-5.2-2 (step 4, the selector check). Row RFC4301-5.2-3 retired 2026-09-30 and its tags moved to RFC4301-5.2-2 (rfc/corrections/rfc4301.md). | IPsec MUST perform the following steps: |
| `5.2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Conditional on nested-SA processing: the sentence binds only a packet the inbound forwarding function 'may cause ... to be sent (outbound) across the IPsec boundary for additional inbound IPsec processing, e.g., in support of nested SAs' (section 5.2). Ze fills no such role: it configures no nested SAs, neither in Go nor through XFRM bundles (the XFRM policy builder in internal/component/ike/dataplane/xfrm_linux.go writes one template for a PROTECT policy and none for BYPASS, so no policy chains a second SA), so no packet of Ze's re-crosses the boundary. Row RFC4301-5.2-5 retired 2026-09-29 (rfc/corrections/rfc4301.md, ruling R15). The SPD-O entry of a bypass entry is RFC4301-4.4.1-12. | If so, then as with ALL outbound traffic that is to be bypassed, the packet MUST be matched against an SPD-O entry. |
| `10:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a blanket conformance statement: 'All IPv4 IPsec implementations MUST comply with all requirements of this document' adds no obligation beyond the requirements the rest of the document states | All IPv4 IPsec implementations MUST comply with all requirements of this document. |
| `10:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the same blanket conformance statement for IPv6; boilerplate that names no distinct behavior | All IPv6 implementations MUST comply with all requirements of this document. |
| `D.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix D.2 is fragment-handling rationale restating the Section 7.1 rule that undefined port selectors are set to ANY | Accordingly, if a specific protocol value is used as a selector, and if that protocol has no port fields, then the port field selectors are to be ignored and ANY MUST be specified as the value for the port fields. |
| `D.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same rationale sentence for the ANY protocol case, restating the Section 7.1 rule site 7.1:3 maps | (In this context, ICMP TYPE and CODE values are lumped together as a single port field (for IKEv2 negotiation), as is the IPv6 Mobility Header TYPE value.) If the protocol selector is ANY, then this should be treated as equivalent to specifying a protocol for which no port fields are defined, and thus the port selectors should be ignored, and MUST be set to ANY. |
| `D.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix D.4 says 'this document says that implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified', an explicit restatement of the Section 7.4 stateful fragment checking obligation | To that end, this document says that implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified. |
| `D.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the Section 7.4 obligation to accept or reject such traffic through the ordinary SPD BYPASS/DISCARD conventions | An implementation also MUST permit a user or administrator to accept such traffic or reject such traffic using the SPD conventions described in Section 4.4.1. |
| `D.8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a description of the document's own structure ('this document offers 3 choices -- one MUST and two MAYs'), not an obligation on an implementation | Thus, this document offers 3 choices -- one MUST and two MAYs. |

## Superseded

No document obsoletes RFC 4301, so its obligations are stated where they were written.
