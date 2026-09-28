# RFC 3768 - Virtual Router Redundancy Protocol (VRRP)

Experimental. Every requirement this repository extracted from RFC 3768, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.0% | 32 of 40 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.5% | 5 of 40 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 40 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 40 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 15.2% | 12 of 79 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 40 | of 51 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 40 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 7.5% | 3 of 40 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 40 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 40 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 37 | of 40 gated MUSTs judged | 28 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 40 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 51 |
| Gated MUST-level | 40 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 79 |
| Tagged units | 79 |
| Recorded audit verdicts | 37 |
| Discrimination records | 12 |
| Summary | `rfc/short/rfc3768.md` |
| Requirement shard | `rfc/requirements/rfc3768.md` |
| RFC text | `rfc/full/rfc3768.txt` |

## Enrolment

Enrolled: VRRPv2 advertisement format, election and timers, virtual-MAC ownership, and the IPv4 Master dataplane, including redirect source selection for non-owner virtual routers.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

Opt-in via `version 2`. Whole-second Advertisement_Interval encoding, v2 advert format, the v2 receive-validation ladder (version, complete-packet, checksum, VRID, Auth Type 0, interval-mismatch discard, address-list discard), the Section 6.4 state machine (priority/master election, skew time, preemption, silent losing-advert discard), virtual-MAC ownership of the VIP via a per-group macvlan, and the v2 rejection rules (no accept-mode, no IPv6).

**What the ledger says remains:**

RFC 3768 authentication types are deliberately not implemented: RFC 9568 Section 9 removed them as providing no real security. Same VRRP experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 32 | one part of the gated population |
| Annotated instead of tested | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **40** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (32):** [`RFC3768-5.2.3-2`](#rfc3768-5.2.3-2), [`RFC3768-5.3.2-1`](#rfc3768-5.3.2-1), [`RFC3768-5.3.4-1`](#rfc3768-5.3.4-1), [`RFC3768-5.3.4-2`](#rfc3768-5.3.4-2), [`RFC3768-5.3.6-1`](#rfc3768-5.3.6-1), [`RFC3768-6.4.2-1`](#rfc3768-6.4.2-1), [`RFC3768-6.4.2-2`](#rfc3768-6.4.2-2), [`RFC3768-6.4.2-3`](#rfc3768-6.4.2-3), [`RFC3768-6.4.2-4`](#rfc3768-6.4.2-4), [`RFC3768-6.4.2-5`](#rfc3768-6.4.2-5), [`RFC3768-6.4.2-6`](#rfc3768-6.4.2-6), [`RFC3768-6.4.2-7`](#rfc3768-6.4.2-7), [`RFC3768-6.4.2-8`](#rfc3768-6.4.2-8), [`RFC3768-6.4.3-1`](#rfc3768-6.4.3-1), [`RFC3768-6.4.3-2`](#rfc3768-6.4.3-2), [`RFC3768-6.4.3-3`](#rfc3768-6.4.3-3), [`RFC3768-6.4.3-4`](#rfc3768-6.4.3-4), [`RFC3768-6.4.3-5`](#rfc3768-6.4.3-5), [`RFC3768-6.4.3-6`](#rfc3768-6.4.3-6), [`RFC3768-6.4.3-7`](#rfc3768-6.4.3-7), [`RFC3768-6.4.3-8`](#rfc3768-6.4.3-8), [`RFC3768-6.4.3-9`](#rfc3768-6.4.3-9), [`RFC3768-7.1-1`](#rfc3768-7.1-1), [`RFC3768-7.1-2`](#rfc3768-7.1-2), [`RFC3768-7.1-3`](#rfc3768-7.1-3), [`RFC3768-7.1-4`](#rfc3768-7.1-4), [`RFC3768-7.1-5`](#rfc3768-7.1-5), [`RFC3768-7.1-6`](#rfc3768-7.1-6), [`RFC3768-7.1-7`](#rfc3768-7.1-7), [`RFC3768-7.1-8`](#rfc3768-7.1-8), [`RFC3768-8.1-1`](#rfc3768-8.1-1), [`RFC3768-8.2-1`](#rfc3768-8.2-1)

**Annotated instead of tested (8):** [`RFC3768-5.2.2-1`](#rfc3768-5.2.2-1), [`RFC3768-5.2.3-1`](#rfc3768-5.2.3-1), [`RFC3768-7.2-1`](#rfc3768-7.2-1), [`RFC3768-7.2-2`](#rfc3768-7.2-2), [`RFC3768-7.2-3`](#rfc3768-7.2-3), [`RFC3768-7.2-4`](#rfc3768-7.2-4), [`RFC3768-8.3-1`](#rfc3768-8.3-1), [`RFC3768-9.2-1`](#rfc3768-9.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3768-5.2.2-1` | Routers MUST NOT forward a datagram with this destination address regardless of its TTL. (§5.2.2) | MUST NOT | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the VRRP plugin performs no IP datagram forwarding -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes each received advert into the FSM and never re-emits it, and tx scopes adverts to link-local multicast with IP_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:143 |
| `RFC3768-5.2.3-1` | The TTL MUST be set to 255. (§5.2.3) | MUST | 5.2.3 | **positive:** `unit/verify` [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L274). **negative:** no negative test. **{single-polarity}:** buildIPv4Header unconditionally sets TTL 255 at internal/plugins/vrrp/transport/transport.go:562, so no input yields a different TTL -- the rx TTL!=255 discard is the separate RFC3768-5.2.3-2 |
| `RFC3768-5.2.3-2` | A VRRP router receiving a packet with the TTL not equal to 255 MUST discard the packet. (§5.2.3, §7.1) | MUST | 5.2.3 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L48). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L533) |
| `RFC3768-5.3.2-1` | A packet with unknown type MUST be discarded. (§5.3.2) | MUST | 5.3.2 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L49). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L150) |
| `RFC3768-5.3.4-1` | The priority value for the VRRP router that owns the IP address(es) associated with the virtual router MUST be 255 (decimal). (§5.3.4) | MUST | 5.3.4 | **positive:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L771). **negative:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L772) |
| `RFC3768-5.3.4-2` | VRRP routers backing up a virtual router MUST use priority values between 1-254 (decimal). (§5.3.4) | MUST | 5.3.4 | **positive:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L316). **negative:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L317) |
| `RFC3768-5.3.6-1` | A packet with unknown authentication type or that does not match the locally configured authentication method MUST be discarded. (§5.3.6, §7.1) | MUST | 5.3.6 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L50). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L598) |
| `RFC3768-6.4.2-1` | - MUST NOT respond to ARP requests for the IP address(s) associated with the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L380). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L424) |
| `RFC3768-6.4.2-2` | - MUST discard packets with a destination link layer MAC address equal to the virtual router MAC address. (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L381). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L425) |
| `RFC3768-6.4.2-3` | - MUST NOT accept packets addressed to the IP address(es) associated with the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L382). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L426) |
| `RFC3768-6.4.2-4` | If a Shutdown event is received, then: o Cancel the Master_Down_Timer o Transition to the {Initialize} state (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L88). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L89) |
| `RFC3768-6.4.2-5` | If the Master_Down_Timer fires, then: o Send an ADVERTISEMENT o Broadcast a gratuitous ARP request containing the virtual router MAC address for each IP address associated with the virtual router o Set the Adver_Timer to Advertisement_Interval o Transition to the {Master} state (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L446). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L636) |
| `RFC3768-6.4.2-6` | If the Priority in the ADVERTISEMENT is Zero, then: o Set the Master_Down_Timer to Skew_Time (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L90). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L91) |
| `RFC3768-6.4.2-7` | If Preempt_Mode is False, or If the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: o Reset the Master_Down_Timer to Master_Down_Interval (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L92). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L93) |
| `RFC3768-6.4.2-8` | If Preempt_Mode is False, or If the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: o Reset the Master_Down_Timer to Master_Down_Interval else: o Discard the ADVERTISEMENT (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L94). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L95) |
| `RFC3768-6.4.3-1` | - MUST respond to ARP requests for the IP address(es) associated with the virtual router. (§6.4.3, §8.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L69). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L136) |
| `RFC3768-6.4.3-2` | - MUST forward packets with a destination link layer MAC address equal to the virtual router MAC address. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L422). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L383) |
| `RFC3768-6.4.3-3` | - MUST NOT accept packets addressed to the IP address(es) associated with the virtual router if it is not the IP address owner. (§6.4.3) | MUST NOT | 6.4.3 | **positive:** `unit/verify` [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L375). **negative:** `unit/verify` [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L376) |
| `RFC3768-6.4.3-4` | - MUST accept packets addressed to the IP address(es) associated with the virtual router if it is the IP address owner. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L423). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L384) |
| `RFC3768-6.4.3-5` | If a Shutdown event is received, then: o Cancel the Adver_Timer o Send an ADVERTISEMENT with Priority = 0 o Transition to the {Initialize} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L96). **positive:** `unit/verify` [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L501). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L97) |
| `RFC3768-6.4.3-6` | If the Adver_Timer fires, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L98). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L637) |
| `RFC3768-6.4.3-7` | If the Priority in the ADVERTISEMENT is Zero, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L99). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L100) |
| `RFC3768-6.4.3-8` | If the Priority in the ADVERTISEMENT is greater than the local Priority, or If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IP Address of the sender is greater than the local primary IP Address, then: o Cancel Adver_Timer o Set Master_Down_Timer to Master_Down_Interval o Transition to the {Backup} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L101). **positive:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L74). **positive:** `unit/verify` [`TestInstanceV2TieBreakDemotionRestartsTheTimers`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L176). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L102). **negative:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L75) |
| `RFC3768-6.4.3-9` | If the Priority in the ADVERTISEMENT is greater than the local Priority, or If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IP Address of the sender is greater than the local primary IP Address, then: o Cancel Adver_Timer o Set Master_Down_Timer to Master_Down_Interval o Transition to the {Backup} state else: o Discard ADVERTISEMENT (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L103). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L104) |
| `RFC3768-7.1-1` | MUST verify the VRRP version is 2. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L42). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L609) |
| `RFC3768-7.1-2` | MUST verify that the received packet contains the complete VRRP packet (including fixed fields, IP Address(es), and Authentication Data). (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L43). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L218) |
| `RFC3768-7.1-3` | MUST verify the VRRP checksum. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L44). **negative:** `unit/verify` [`TestDecodeV2ChecksumCorrupt`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L306) |
| `RFC3768-7.1-4` | MUST verify that the VRID is configured on the receiving interface and the local router is not the IP Address owner (Priority equals 255 (decimal)). (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L45). **positive:** `unit/verify` [`TestInstanceV2OwnerDiscardsAdvert`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1182). **negative:** `unit/verify` [`TestInstanceV2OwnerDiscardsAdvert`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1181). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L163) |
| `RFC3768-7.1-5` | MUST verify that the Auth Type matches the locally configured authentication method for the virtual router and perform that authentication method. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L46). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L599) |
| `RFC3768-7.1-6` | If any one of the above checks fails, the receiver MUST discard the packet (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L545). **negative:** `unit/verify` [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L525) |
| `RFC3768-7.1-7` | If the packet was not generated by the address owner (Priority does not equal 255 (decimal)), the receiver MUST drop the packet, otherwise continue processing. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceV2AddressListMatchReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L629). **positive:** `unit/verify` [`TestInstanceV2AddressListMismatchFromOwnerContinues`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1229). **negative:** `unit/verify` [`TestInstanceV2AddressListMismatchDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L590) |
| `RFC3768-7.1-8` | - MUST verify that the Adver Interval in the packet is the same as the locally configured for this virtual router (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L47). **negative:** `unit/verify` [`TestDecodeV2IntervalMismatchDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L293). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L251) |
| `RFC3768-7.2-1` | Fill in the VRRP packet fields with the appropriate virtual router configuration state - Compute the VRRP checksum (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestEncodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L87). **negative:** no negative test. **{single-polarity}:** a golden encode pins every field and the checksum -- WriteTo internal/plugins/vrrp/packet/packet.go:251 plus FillChecksum internal/plugins/vrrp/packet/checksum.go:86 -- while a corrupted-encoding rejection is the separate receive requirement RFC3768-7.1-3 |
| `RFC3768-7.2-2` | Set the source MAC address to Virtual Router MAC Address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L386). **negative:** no negative test. **{single-polarity}:** the source MAC is the virtual-router MAC from packet.VirtualMAC internal/plugins/vrrp/packet/packet.go:97 egressed by binding the tx socket to the vMAC macvlan internal/plugins/vrrp/transport/backend_linux.go:133, a deterministic derivation with no input that yields a different MAC |
| `RFC3768-7.2-3` | Set the source IP address to interface primary IP address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L216). **negative:** no negative test. **{single-polarity}:** the source IP is the parent unit primary IPv4 from resolveParentPrimaryV4 internal/plugins/vrrp/transport/transport.go:573, a deterministic selection re-resolved on address change, with no input that yields a wrong-source advert |
| `RFC3768-7.2-4` | Set the IP protocol to VRRP - Send the VRRP packet to the VRRP IP multicast group (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L275). **negative:** no negative test. **{single-polarity}:** buildIPv4Header sets IP protocol 112 at internal/plugins/vrrp/transport/transport.go:563 and SendAdvert targets 224.0.0.18 at internal/plugins/vrrp/transport/backend_linux.go:256, both constants with no input that changes them |
| `RFC3768-8.1-1` | If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.1; lowercase "must" in the RFC) | MUST | 8.1 | **positive:** `unit/verify` [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L20). **negative:** `unit/verify` [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L21) |
| `RFC3768-8.2-1` | The Master virtual router MUST NOT respond with its physical MAC address. (§8.2) | MUST NOT | 8.2 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L70). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L42). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L137). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L43) |
| `RFC3768-8.3-1` | If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the Virtual Router MAC address in the Proxy ARP message. (§8.3; lowercase "must" in the RFC) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no proxy ARP for virtual addresses -- the per-group virtual-MAC macvlan answers ARP for the VIP directly (createMacvlan internal/plugins/vrrp/register.go:329 plus the sole-responder sysctl recipe internal/plugins/vrrp/dataplane_linux.go:64), and no proxy-ARP path exists |
| `RFC3768-9.2-1` | The functional address mode of operation MUST be implemented by routers supporting VRRP on token ring. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze supports only Ethernet-family parents (ethernet, veth, bridge, dummy -- internal/plugins/vrrp/groups.go:76) over AF_PACKET macvlan transport internal/plugins/vrrp/transport/backend_linux.go, with no token-ring transport, so the functional-address mode has no applicable code path |
| `RFC3768-5.3.10-1` | The authentication string is currently only used to maintain backwards compatibility with RFC 2338. It SHOULD be set to zero on transmission and ignored on reception. (§5.3.10) | SHOULD | 5.3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-7.1-9` | If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-7.1-10` | MAY verify that "Count IP Addrs" and the list of IP Address matches the IP_Addresses configured for the VRID If the above check fails, the receiver SHOULD log the event (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-7.1-11` | MUST verify that the Adver Interval in the packet is the same as the locally configured for this virtual router If the above check fails, the receiver MUST discard the packet, SHOULD log the event (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-8.2-2` | When a VRRP router restarts or boots, it SHOULD not send any ARP messages with its physical MAC address for the IP address it owns, it should only send ARP messages that include Virtual MAC addresses. (§8.2) | SHOULD NOT | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-8.2-3` | When configuring an interface, VRRP routers should broadcast a gratuitous ARP request containing the virtual router MAC address for each IP address on that interface. - At system boot, when initializing interfaces for VRRP operation; delay gratuitous ARP requests and ARP responses until both the IP address and the virtual router MAC address are configured. (§8.2) | SHOULD | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-8.4-1` | A VRRP router SHOULD not forward packets addressed to the IP Address(es) it becomes Master for if it is not the owner. (§8.4) | SHOULD NOT | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-9.1-1` | To avoid this an implementation SHOULD configure the virtual router MAC address by adding a unicast MAC filter in the FDDI device, rather than changing its hardware MAC address. (§9.1) | SHOULD | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-7.1-12` | MAY indicate via network management that an error occurred. - MAY verify that "Count IP Addrs" and the list of IP Address matches the IP_Addresses configured for the VRID If the above check fails, the receiver SHOULD log the event and MAY indicate via network management that a misconfiguration was detected. (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-7.1-13` | MAY verify that "Count IP Addrs" and the list of IP Address matches the IP_Addresses configured for the VRID (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-9.2-2` | Additionally, routers MAY support unicast mode of operation to take advantage of newer token ring adapter implementations that support non-promiscuous reception for multiple unicast MAC addresses and to avoid both the multicast traffic and usage conflicts associated with the use of token ring functional addresses. (§9.2) | MAY | 9.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3768-5.2.2-1`](#rfc3768-5.2.2-1) Routers MUST NOT forward a datagram with this destination address regardless of its TTL. (§5.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: the VRRP plugin performs no IP datagram forwarding -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes each received advert into the FSM and never re-emits it, and tx scopes adverts to link-local multicast with IP_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:143 |
| [`RFC3768-8.3-1`](#rfc3768-8.3-1) If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the Virtual Router MAC address in the Proxy ARP message. (§8.3; lowercase "must" in the RFC) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no proxy ARP for virtual addresses -- the per-group virtual-MAC macvlan answers ARP for the VIP directly (createMacvlan internal/plugins/vrrp/register.go:329 plus the sole-responder sysctl recipe internal/plugins/vrrp/dataplane_linux.go:64), and no proxy-ARP path exists |
| [`RFC3768-9.2-1`](#rfc3768-9.2-1) The functional address mode of operation MUST be implemented by routers supporting VRRP on token ring. (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze supports only Ethernet-family parents (ethernet, veth, bridge, dummy -- internal/plugins/vrrp/groups.go:76) over AF_PACKET macvlan transport internal/plugins/vrrp/transport/backend_linux.go, with no token-ring transport, so the functional-address mode has no applicable code path |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3768-5.2.2-1`](#rfc3768-5.2.2-1)

Routers MUST NOT forward a datagram with this destination address regardless of its TTL. (§5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3768-5.2.2-1, so no unit is bound to it.

### [`RFC3768-5.2.3-1`](#rfc3768-5.2.3-1)

The TTL MUST be set to 255. (§5.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive. Forbidden: a sent advertisement with IPv4 TTL other than 255. TestSendAdvertIPv4HeaderTTLProtoDst compares the TTL octet of the built header with the literal 255, so any other value goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L274) | unit/verify | unproven |

### [`RFC3768-5.2.3-2`](#rfc3768-5.2.3-2)

A VRRP router receiving a packet with the TTL not equal to 255 MUST discard the packet. (§5.2.3, §7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): the only negative, TestNegativeReferenceBugs N4, decodes the VRRPv3 golden (goldenV3v4Hex, metaV3v4, lookupConst(VersionV3)) at TTL 64; no tagged unit feeds a VRRPv2 packet at a TTL other than 255, so a v2-only break of the TTL check (validate.go) stays green. The positive is the v2 golden at TTL 255. That the TTL row precedes the version branch is an argument about the code, not an assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L533) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L48) | unit/verify | unproven |

### [`RFC3768-5.3.2-1`](#rfc3768-5.3.2-1)

A packet with unknown type MUST be discarded. (§5.3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): the negative, TestValidationOrder '3<4 type beats vrid', sets b[0]=0x32 on a VRRPv3 packet (advV3v4, metaV3v4); no tagged unit feeds a VRRPv2 packet whose Type is not 1, so a v2-only break of the type check stays green. The positive is the v2 golden (TestDecodeGoldenV2). Blind reader BS-A: same finding.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L150) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L49) | unit/verify | unproven |

### [`RFC3768-5.3.4-1`](#rfc3768-5.3.4-1)

The priority value for the VRRP router that owns the IP address(es) associated with the virtual router MUST be 255 (decimal). (§5.3.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): TestOwnerAutoDetection asserts EffectivePriority 255 for an owner configured at 120 and 100 for a near-miss non-owner, but both groups carry no version leaf, so they are VRRPv3 (groups.go default versionV3). No tagged unit runs a VRRPv2 owner, so a v2-only break of the owner priority stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L772) | unit/verify | revert, verified |
| positive | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L771) | unit/verify | revert, verified |

### [`RFC3768-5.3.4-2`](#rfc3768-5.3.4-2)

VRRP routers backing up a virtual router MUST use priority values between 1-254 (decimal). (§5.3.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): TestBoundaryPriority asserts validateGroups rejects 0 and 255 and accepts 1 and 254, but every case omits the version leaf and so validates a VRRPv3 group (groups.go default versionV3). No VRRPv2 group is validated, so a v2-only break of the 1..254 range stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L317) | unit/verify | unproven |
| positive | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L316) | unit/verify | unproven |

### [`RFC3768-5.3.6-1`](#rfc3768-5.3.6-1)

A packet with unknown authentication type or that does not match the locally configured authentication method MUST be discarded. (§5.3.6, §7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clause 2 (Auth Type not matching the locally configured method): case N8 sets Auth Type 1 at a group configured for type 0 and fatals unless ErrAuthType; the golden type 0 decodes. Clause 1 (unknown authentication type, 3..255): no tagged case feeds one, so a check rejecting only the defined types 1 and 2 passes. Decode rejects any non-zero today (validate.go Row 10), but no assertion pins it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L598) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L50) | unit/verify | unproven |

### [`RFC3768-6.4.2-1`](#rfc3768-6.4.2-1)

- MUST NOT respond to ARP requests for the IP address(s) associated with the virtual router. (§6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): forbidden: a Backup answering ARP for the virtual address. TestInstanceStartupNonOwnerGoesBackup asserts no VIP install in Backup and TestInstanceOwnerStartupGoesMaster the install in Master, but both run testSpec() (Version versionV3); no VRRPv2 Backup is started. The assertion is also a proxy: no ARP reply is observed, only that the VIP is not installed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L424) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L380) | unit/verify | unproven |

### [`RFC3768-6.4.2-2`](#rfc3768-6.4.2-2)

- MUST discard packets with a destination link layer MAC address equal to the virtual router MAC address. (§6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the units assert only that a Backup installs no VIP. The vMAC macvlan still exists in Backup (created at apply, rp_filter=0), so frames addressed to the virtual MAC that reach a Backup are not shown discarded: no-VIP proves no local accept, not no forwarding

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L425) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L381) | unit/verify | unproven |

### [`RFC3768-6.4.2-3`](#rfc3768-6.4.2-3)

- MUST NOT accept packets addressed to the IP address(es) associated with the virtual router. (§6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): forbidden: a Backup accepting packets addressed to the virtual address. The tagged units assert no VIP install in Backup and an install in Master, but both run testSpec() (Version versionV3); no VRRPv2 Backup is started, so a v2-only break stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L426) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L382) | unit/verify | unproven |

### [`RFC3768-6.4.2-4`](#rfc3768-6.4.2-4)

If a Shutdown event is received, then: o Cancel the Master_Down_Timer o Transition to the {Initialize} state (§6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): row backup/shutdown asserts [StopTimers, EmitStateChange] and next Initialize, and row backup/advert-timer-expired/stale the non-Shutdown contrast, but both set up backupInstance(baseCfg()) and baseCfg is Version 3. No VRRPv2 Backup receives Shutdown in a tagged unit. Blind reader BS-A: same finding.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L89) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L88) | unit/verify | unproven |

### [`RFC3768-6.4.2-5`](#rfc3768-6.4.2-5)

If the Master_Down_Timer fires, then: o Send an ADVERTISEMENT o Broadcast a gratuitous ARP request containing the virtual router MAC address for each IP address associated with the virtual router o Set the Adver_Timer to Advertisement_Interval o Transition to the {Master} state (§6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestFSMMasterDownPromotion asserts only the action TYPES and order: the Adver_Timer interval is not checked against Advertisement_Interval and AnnounceFailover is not shown to carry the virtual MAC gratuitous ARP. The stale-gen negative is sound

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L636) | unit/verify | unproven |
| positive | [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L446) | unit/verify | unproven |

### [`RFC3768-6.4.2-6`](#rfc3768-6.4.2-6)

If the Priority in the ADVERTISEMENT is Zero, then: o Set the Master_Down_Timer to Skew_Time (§6.4.2)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): row backup/advert/priority-zero runs baseCfg (Version 3) and expects StartMasterDownTimer{skewDur(baseCfg(),1000)}; skewDur calls the production skewTime(3, 100, 1000), so the expected value is computed by the code under test and the v2 branch of skewTime (timers.go, 'if version == 2') never runs. RFC 3768 Section 6.1 defines Skew_Time as '( (256 - Priority) / 256 )' seconds; the unit proves the RFC 9568 Skew_Time, which scales with the adopted interval, a neighbouring rule. Blind reader BS-A: wrong.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L91) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L90) | unit/verify | unproven |

### [`RFC3768-6.4.2-7`](#rfc3768-6.4.2-7)

If Preempt_Mode is False, or If the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: o Reset the Master_Down_Timer to Master_Down_Interval (§6.4.2)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. the tagged positive (row backup/advert/adopt/equal-priority) runs a v3 instance and asserts master-down re-armed from the ADVERTISED 4000 ms interval. RFC 3768 has no interval adoption: Master_Down_Interval comes from the local Advertisement_Interval. TestV2NoIntervalAdoption asserts the v2 behaviour but carries no tag

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L93) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L92) | unit/verify | unproven |

### [`RFC3768-6.4.2-8`](#rfc3768-6.4.2-8)

If Preempt_Mode is False, or If the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: o Reset the Master_Down_Timer to Master_Down_Interval else: o Discard the ADVERTISEMENT (§6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Preempt_Mode False clause: row preempt-false-lower-priority asserts a master-down reset. Discard clause: row preempt-lower-priority-no-delay asserts no actions and Backup. Priority >= local clause: only EQUAL is run; no Backup row feeds a higher-priority advert, so an '==' comparison passes. Reset value: both reset rows run v3 (baseCfg Version 3) and expect mdDur from the ADVERTISED 4000 ms, the RFC 9568 adoption; RFC 3768 resets to the local Master_Down_Interval and no tagged row asserts the v2 value (TestV2NoIntervalAdoption does, untagged).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L95) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L94) | unit/verify | unproven |

### [`RFC3768-6.4.3-1`](#rfc3768-6.4.3-1)

- MUST respond to ARP requests for the IP address(es) associated with the virtual router. (§6.4.3, §8.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive asserts sysctl values (proxy for the ARP reply, the reply itself is not observed); the negative is teardown restoring the knobs, which is not an input violating the Master ARP obligation

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L136) | unit/verify | unproven |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L69) | unit/verify | unproven |

### [`RFC3768-6.4.3-2`](#rfc3768-6.4.3-2)

- MUST forward packets with a destination link layer MAC address equal to the virtual router MAC address. (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the units assert VIP installation on the macvlan; forwarding of frames to the virtual MAC depends on the macvlan, which exists in Backup too, so neither polarity shows forward-in-Master versus not-in-Backup

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L383) | unit/verify | unproven |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L422) | unit/verify | unproven |

### [`RFC3768-6.4.3-3`](#rfc3768-6.4.3-3)

- MUST NOT accept packets addressed to the IP address(es) associated with the virtual router if it is not the IP address owner. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a v2 non-owner Master accepting packets addressed to the virtual address. TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress (Version 2) fatals unless exactly one accept-filter call is made and errors unless accept=false for the non-owner and accept=true for the owner.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L376) | unit/verify | unproven |
| positive | [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L375) | unit/verify | unproven |

### [`RFC3768-6.4.3-4`](#rfc3768-6.4.3-4)

- MUST accept packets addressed to the IP address(es) associated with the virtual router if it is the IP address owner. (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged positive asserts only that the owner Master installs the VIP; it does not assert the accept filter is accept=true, so an owner suppression would still pass. The negative is a Backup, not an owner Master that refuses

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L384) | unit/verify | unproven |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L423) | unit/verify | unproven |

### [`RFC3768-6.4.3-5`](#rfc3768-6.4.3-5)

If a Shutdown event is received, then: o Cancel the Adver_Timer o Send an ADVERTISEMENT with Priority = 0 o Transition to the {Initialize} state (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): row master/shutdown asserts exactly StopTimers, SendAdvertZeroPriority, RemoveVIPs, EmitStateChange and next Initialize, and TestInstanceShutdownAsMasterSendsPriorityZero sees the Priority 0 advert leave the executor, but the row runs baseCfg (Version 3) and the instance test testSpec() (versionV3). No VRRPv2 Master shuts down in a tagged unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L97) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L96) | unit/verify | unproven |
| positive | [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L501) | unit/verify | unproven |

### [`RFC3768-6.4.3-6`](#rfc3768-6.4.3-6)

If the Adver_Timer fires, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): row master/advert-timer-expired/matching-gen asserts SendAdvert then StartAdvertTimer{1s}, and TestFSMStaleTimerGenerationIgnored the stale contrast, but both run baseCfg (Version 3). No VRRPv2 Master's Adver_Timer fires in a tagged unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L637) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L98) | unit/verify | unproven |

### [`RFC3768-6.4.3-7`](#rfc3768-6.4.3-7)

If the Priority in the ADVERTISEMENT is Zero, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): the positive, row master/advert/priority-zero/reset-advert-timer, asserts SendAdvert plus StartAdvertTimer{1s} and Master, but runs baseCfg (Version 3). Only the negative, row losing-lower-priority/v2-silent, is VRRPv2. No VRRPv2 Master receives a Priority 0 advert in a tagged unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L100) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L99) | unit/verify | unproven |

### [`RFC3768-6.4.3-8`](#rfc3768-6.4.3-8)

If the Priority in the ADVERTISEMENT is greater than the local Priority, or If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IP Address of the sender is greater than the local primary IP Address, then: o Cancel Adver_Timer o Set Master_Down_Timer to Master_Down_Interval o Transition to the {Backup} state (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-VRRP strict re-read 2026-09-27 (independent). Clause 'equal Priority and sender primary address greater than the local primary address': forbidden = a v2 Master comparing the wrong operand or holding Master; TestInstanceTieBreakComparesTheAdvertisementSource v2 cases assert Backup for sender 192.0.2.8 above source 192.0.2.2 and Master for sender 192.0.2.80 below source 192.0.2.100 (both above the first virtual address 192.0.2.50), so a first-virtual-address operand or an inverted compare goes red (records: revert syncSourceLocked, revert senderWinsTieBreak). Clauses 'Cancel Adver_Timer', 'Set Master_Down_Timer to Master_Down_Interval', 'Transition to Backup' on that branch: TestInstanceV2TieBreakDemotionRestartsTheTimers asserts in.advert == nil, no advertisement over two intervals, masterDown armed, Backup at MDI-1ms and Master at MDI+1ms with MDI = 3.21875 s derived from RFC 3768 Section 6.1, so a timer left armed, never armed or armed at another value goes red (hand reds DF-VRRP-6 red-timer.log, red-md.log; record revert demoteToBackup). Negative polarity: v2 matrix row master/advert/losing-lower-priority/v2-silent (nil actions, Master) and the v2 'source above sender holds' case. NOT proven for VRRPv2: the 'Priority greater than the local Priority' clause. The only tagged units feeding a higher-priority advert are the TestFSMTransitionMatrix rows master/advert/higher-priority/demote, which run baseCfg (Version 3) with IntervalMs 4000 and expect mdDur(cfg,4000), i.e. the RFC 9568 interval-adoption rule; no v2 instance ever receives a higher-priority advert in a tagged unit, so a v2-only break of that branch stays green. The predicate in masterAdvert is shared across versions, which is why the gap is narrow, but a shared predicate is an argument, not an assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L102) | unit/verify | unproven |
| negative | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L101) | unit/verify | unproven |
| positive | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestInstanceV2TieBreakDemotionRestartsTheTimers`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L176) | unit/verify | revert, verified |

### [`RFC3768-6.4.3-9`](#rfc3768-6.4.3-9)

If the Priority in the ADVERTISEMENT is greater than the local Priority, or If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IP Address of the sender is greater than the local primary IP Address, then: o Cancel Adver_Timer o Set Master_Down_Timer to Master_Down_Interval o Transition to the {Backup} state else: o Discard ADVERTISEMENT (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Discard clause: row master/advert/losing-lower-priority/v2-silent (Version 2) asserts no actions and stays Master. Demote clauses (higher priority; equal priority with greater sender IP): rows higher-priority/demote and tie-break-lost assert StopTimers, StartMasterDownTimer and Backup, but run a v3 instance and expect master-down from the advertised 4000 ms, the RFC 9568 adoption RFC 3768 does not have (why RFC3768-6.4.3-8 is wrong). No v2 demotion is asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L104) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L103) | unit/verify | unproven |

### [`RFC3768-7.1-1`](#rfc3768-7.1-1)

MUST verify the VRRP version is 2. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a version other than 2 at a v2 group. Case N9 fatals unless a v3 packet at a v2 group and a v2 packet at a v3 group each return ErrVersion; the golden v2 packet decodes at a v2 group.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L609) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L42) | unit/verify | unproven |

### [`RFC3768-7.1-2`](#rfc3768-7.1-2)

MUST verify that the received packet contains the complete VRRP packet (including fixed fields, IP Address(es), and Authentication Data). (§7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. IP Address(es) clause: case 9<10 lies Count=3 over two addresses and fatals unless ErrLength. Fixed-fields clause: no tagged v2 case feeds a payload shorter than the 8-octet header (ErrTruncated). Authentication Data clause: no tagged case shortens only the 8-octet trailer under an honest Count; the positive only shows the complete 24-octet golden decodes, so a check treating the trailer as optional (length 16 or 24 for Count 2) passes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L218) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L43) | unit/verify | unproven |

### [`RFC3768-7.1-3`](#rfc3768-7.1-3)

MUST verify the VRRP checksum. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a v2 packet with a wrong checksum. TestDecodeV2ChecksumCorrupt flips the checksum of the v2 golden and fatals unless ErrChecksum; the golden v2 verifies.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV2ChecksumCorrupt`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L306) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L44) | unit/verify | unproven |

### [`RFC3768-7.1-4`](#rfc3768-7.1-4)

MUST verify that the VRID is configured on the receiving interface and the local router is not the IP Address owner (Priority equals 255 (decimal)). (§7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-FIX re-read 2026-09-27. Owner half enforced for VRRPv2: TestInstanceV2OwnerDiscardsAdvert feeds a Priority 255 advert from a greater sender to a v2 owner and asserts ReasonOwner, no FSM event and Master held, and the same advert to a non-owner reaches the FSM. VRID half: Decode returns ErrUnknownVRID when the lookup does not resolve the VRID (TestValidationOrder 4<5) and TestDecodeGoldenV2 resolves a configured one. The clause 'on the receiving interface' is not asserted: the lookup (instance.lookup) compares the VRID with its own instance, and interface scoping rests on the per-instance socket the transport binds, which no tagged unit exercises; an advert for a VRID configured on another interface is never fed to any tagged unit. DF-VRRP-5 re-read 2026-09-27: onPacket changed only by the v3 noteOwnerConflict branch and the syncSourceLocked call before the FSM event, neither on the v2 owner path, so the owner-half assertions stand. The owner half is also proven end to end by the vrrp-v2-owner-keepalived interop scenario (keepalived at 255 from a greater source; Ze keeps the /32 and counts owner discards), red with the v2 owner return removed (DF-VRRP-4 iop-red2.log, DF-VRRP-5 iop-owner-red.log). Still weak: the 'on the receiving interface' clause has no tagged unit. RA-VRRP strict re-read 2026-09-27 (independent): agrees weak. Owner half: TestInstanceV2OwnerDiscardsAdvert asserts rxErrors == [ReasonOwner] and an empty event queue for the v2 owner, and AdvertReceived with no rx error for the v2 non-owner, so a missing discard or a discard of every advert goes red. VRID half: TestValidationOrder '4<5' asserts ErrUnknownVRID, but on a v3 packet with a lookup that answers false, and TestDecodeGoldenV2 resolves the configured VRID. 'On the receiving interface' has no assertion: no tagged unit delivers an advert whose VRID is configured on a different interface.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceV2OwnerDiscardsAdvert`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1181) | unit/verify | revert, verified |
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L163) | unit/verify | unproven |
| positive | [`TestInstanceV2OwnerDiscardsAdvert`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1182) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L45) | unit/verify | unproven |

### [`RFC3768-7.1-5`](#rfc3768-7.1-5)

MUST verify that the Auth Type matches the locally configured authentication method for the virtual router and perform that authentication method. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting an Auth Type other than the one configured method (0, No Authentication). Case N8 sets Auth Type 1 and fatals unless ErrAuthType; the golden type 0 decodes. With method 0 there is no authentication to perform.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L599) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L46) | unit/verify | unproven |

### [`RFC3768-7.1-6`](#rfc3768-7.1-6)

If any one of the above checks fails, the receiver MUST discard the packet (§7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the negative asserts only that one rx error was recorded; it never checks in.events, so deleting the return after recordRxError in onPacket (feeding a zero advert to the FSM) still passes. Its comment claims "never reaches the FSM"

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L525) | unit/verify | unproven |
| positive | [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L545) | unit/verify | unproven |

### [`RFC3768-7.1-7`](#rfc3768-7.1-7)

If the packet was not generated by the address owner (Priority does not equal 255 (decimal)), the receiver MUST drop the packet, otherwise continue processing. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. onPacket drops a v2 address-list mismatch only when the sender Priority is not 255, and otherwise logs it (address-list reason) and continues. TestInstanceV2AddressListMismatchDrops pins the drop for Priority 250 with no FSM event; TestInstanceV2AddressListMismatchFromOwnerContinues pins the Priority 255 mismatch reaching the FSM with the reason logged; TestInstanceV2AddressListMatchReachesFSM pins a matching list. Discrimination recorded for each. DF-VRRP-5 re-read 2026-09-27, strictly: clause 'not generated by the address owner (Priority does not equal 255) MUST drop': TestInstanceV2AddressListMismatchDrops feeds a Priority 250 mismatch and asserts exactly ReasonAddressList and no FSM event, so a continue goes red. Clause 'otherwise continue processing': TestInstanceV2AddressListMismatchFromOwnerContinues feeds a Priority 255 mismatch and asserts the reason logged and an AdvertReceived at priority 255, so a drop goes red. A matching list reaching the FSM is TestInstanceV2AddressListMatchReachesFSM. onPacket's later edits (v3 noteOwnerConflict, syncSourceLocked) sit off this path. Priority 254, the boundary below the owner, is not fed to any unit. RA-VRRP strict re-read 2026-09-27 (independent): agrees enforced. Forbidden 1, continuing a non-owner mismatch: TestInstanceV2AddressListMismatchDrops (Priority 250) asserts rxErrors == [ReasonAddressList] and no event, red on continue. Forbidden 2, dropping an owner mismatch: TestInstanceV2AddressListMismatchFromOwnerContinues (Priority 255) asserts the reason and an AdvertReceived at priority 255, red on drop. The code tests adv.Priority != ownerPriority (instance.go onPacket), so the 250 and 255 cases pin the predicate's two outcomes; 254 remains unfed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceV2AddressListMismatchDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L590) | unit/verify | revert, verified |
| positive | [`TestInstanceV2AddressListMatchReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L629) | unit/verify | revert, verified |
| positive | [`TestInstanceV2AddressListMismatchFromOwnerContinues`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1229) | unit/verify | revert, verified |

### [`RFC3768-7.1-8`](#rfc3768-7.1-8)

- MUST verify that the Adver Interval in the packet is the same as the locally configured for this virtual router (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a v2 packet whose Adver Int differs from the local one. TestDecodeV2IntervalMismatchDiscard and ValidationOrder case 12 fatal unless ErrV2IntervalMismatch; the golden with the equal interval decodes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV2IntervalMismatchDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L293) | unit/verify | unproven |
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L251) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L47) | unit/verify | unproven |

### [`RFC3768-7.2-1`](#rfc3768-7.2-1)

Fill in the VRRP packet fields with the appropriate virtual router configuration state - Compute the VRRP checksum (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive. Forbidden: any VRRP field or the checksum not filled from the advertisement state. TestEncodeGoldenV2 compares WriteTo plus FillChecksum output byte for byte against the v2 golden, so a wrong field or checksum goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L87) | unit/verify | unproven |

### [`RFC3768-7.2-2`](#rfc3768-7.2-2)

Set the source MAC address to Virtual Router MAC Address (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the unit pins the VirtualMAC derivation only; nothing asserts that transmitted frames carry it as source MAC (tx socket bound to the vMAC macvlan is not exercised)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L386) | unit/verify | unproven |

### [`RFC3768-7.2-3`](#rfc3768-7.2-3)

Set the source IP address to interface primary IP address (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): TestSendAdvertUsesParentPrimaryV4Source asserts the source equals the parent's address, re-resolved on update and on an address change, but every stub returns exactly one IPv4 address, so 'primary' versus a secondary address is never tested: a transport that picked the last or any address passes. The {single-polarity} marker's 'no input that yields a wrong-source advert' is untested for a multi-address parent. Blind reader BS-A: same finding.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L216) | unit/verify | unproven |

### [`RFC3768-7.2-4`](#rfc3768-7.2-4)

Set the IP protocol to VRRP - Send the VRRP packet to the VRRP IP multicast group (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. protocol and destination are compared with packet.ProtoNumber and packet.MulticastV4, the code constants, so a drift of either constant still passes; the literals 112 and 224.0.0.18 are pinned only in untagged TestEncodeGoldenV3IPv4

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L275) | unit/verify | unproven |

### [`RFC3768-8.1-1`](#rfc3768-8.1-1)

If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.1; lowercase "must" in the RFC)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a non-owner Master choosing a redirect source that does not follow the virtual router the packet was sent to. TestVRRPRedirectSourceFollowsVirtualMAC (Linux integration, two non-owner vMAC macvlans) asserts the redirect source is the VIP of the destination vMAC's group, and a frame to the physical MAC is redirected from 192.0.2.254.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L21) | unit/verify | unproven |
| positive | [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L20) | unit/verify | unproven |

### [`RFC3768-8.2-1`](#rfc3768-8.2-1)

The Master virtual router MUST NOT respond with its physical MAC address. (§8.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: the Master answering ARP for a virtual address with its physical MAC. Owner case: TestVRRPOwnerAnswersWithVirtualMACOnly (integration && linux, QEMU) resolves the owner VIP from a peer and reads ar$sha in every reply: the no-filter control captures the parent's physical MAC, the filtered phase captures only the virtual MAC, and withdrawal restores the physical answer; the producer is ownerARPReplyTerm, installed by doInstallVIPs. Non-owner case still rests on TestDataplaneApplyIPv4SetsRecipe asserting the parent arp_ignore/arp_filter recipe, a proxy: no tagged unit resolves a non-owner VIP end to end. Proven in QEMU on the 7.2 runtime kernel 2026-09-27 (DF-VRRP-3 qemu.log): green, and red in the filtered ARP phase with ownerARPReplyTerm's Drop replaced by Accept. applyDataplaneSysctls now also sets arp_ignore=8 on an IPv6 group's macvlan, which the IPv4 path this row rests on does not reach RA-VRRP strict re-read 2026-09-27 (independent): agrees weak, for a reason the earlier note omits. TestVRRPOwnerAnswersWithVirtualMACOnly calls setOwnerFilter by hand; it never runs doInstallVIPs, and its groups are Version 3. The product wiring that installs the filter when the owner becomes Master (doInstallVIPs -> deps.setOwnerFilter with ownedVIPs) is proven only by the untagged TestOwnerFilterInstalledBeforeTheAddressAndWithdrawnAfterIt, so dropping that call leaves every tagged unit green. The non-owner half is asserted at Ze's boundary (exact parent arp_ignore=1/arp_filter=1 in TestDataplaneApplyIPv4SetsRecipe, as ai/rules/rfc-compliance.md asks for a lower-layer requirement), but its tagged negative TestDataplaneRestoreOnLastGroup is a teardown property, not a physical-MAC answer. QEMU logs read: DF-VRRP-3 qemu.log green PASS, redarp FAIL 'filtered ARP: physical MAC ... answered = true', rednd FAIL 'filtered ND'; that run predates DF-VRRP-5's appendCombine edits to the frame builders, which I checked are byte-identical frames.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L137) | unit/verify | unproven |
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L43) | unit/verify | revert, verified |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L70) | unit/verify | unproven |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L42) | unit/verify | revert, verified |

### [`RFC3768-8.3-1`](#rfc3768-8.3-1)

If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the Virtual Router MAC address in the Proxy ARP message. (§8.3; lowercase "must" in the RFC)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3768-8.3-1, so no unit is bound to it.

### [`RFC3768-9.2-1`](#rfc3768-9.2-1)

The functional address mode of operation MUST be implemented by routers supporting VRRP on token ring. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3768-9.2-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc3768.txt |
| Source fingerprint | 47afb9c05728468f |
| Record | rfc/extraction/rfc3768.json |
| Mapped sentences | 25 |
| Declined as scope | 9 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.2.1` | not stated | 0 | walked | not stated |
| `5.2.2` | not stated | 1 | walked | not stated |
| `5.2.3` | not stated | 2 | walked | not stated |
| `5.2.4` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.3.1` | not stated | 0 | walked | not stated |
| `5.3.2` | not stated | 1 | walked | not stated |
| `5.3.3` | not stated | 0 | walked | not stated |
| `5.3.4` | not stated | 2 | walked | not stated |
| `5.3.5` | not stated | 0 | walked | not stated |
| `5.3.6` | not stated | 1 | walked | not stated |
| `5.3.6.1` | not stated | 0 | walked | not stated |
| `5.3.6.2` | not stated | 0 | walked | not stated |
| `5.3.6.3` | not stated | 0 | walked | not stated |
| `5.3.7` | not stated | 0 | walked | not stated |
| `5.3.8` | not stated | 0 | walked | not stated |
| `5.3.9` | not stated | 0 | walked | not stated |
| `5.3.10` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.4.1` | not stated | 0 | walked | not stated |
| `6.4.2` | not stated | 4 | walked | not stated |
| `6.4.3` | not stated | 5 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 5 | walked | not stated |
| `7.2` | not stated | 1 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 1 | walked | not stated |
| `8.2` | not stated | 2 | walked | not stated |
| `8.3` | not stated | 1 | walked | not stated |
| `8.4` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 2 | walked | not stated |
| `9.3` | not stated | 0 | walked | not stated |
| `10` | not stated | 1 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `15` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Table of contents line; the word Required is the title of Section 2.1, not an obligation. | Required Features . . . . . . . . . . . . . . . . . . . . . . 5 2.1. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section heading 'Required Features'; a heading states no obligation. | Required Features |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Overview prose with a lowercase must describing operator configuration across a LAN, not an implementation obligation: the sentence sits between 'A VRRP router may associate a virtual router with its real addresses' and 'However, there is no restriction against reusing a VRID with a different address mapping on different LANs.' | The mapping between VRID and addresses must be coordinated among all VRRP routers on a LAN. |
| `4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 4.1 is 'Sample Configuration 1'; the lowercase must describes what the example deployment does, not what an implementation owes. | In order to backup IP B, a second virtual router must be configured. |
| `7.1:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the discard half of the Adver Interval check mapped at site 7.1:4, which RFC3768-7.1-8 already carries ('discard the packet on mismatch'). | If the above check fails, the receiver MUST discard the packet, SHOULD log the event and MAY indicate via network management that a misconfiguration was detected. |
| `8.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Master ARP obligation mapped at site 6.4.3:2; RFC3768-6.4.3-1 cites both Section 6.4.3 and Section 8.2. | When a host sends an ARP request for one of the virtual router IP addresses, the Master virtual router MUST respond to the ARP request with the virtual MAC address for the virtual router. |
| `9.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose about source route bridges with a lowercase 'required'; it states that a mechanism is needed, names none, and the token ring obligation itself is site 9.2:2. | - In order to switch to a new master located on a different bridge token ring segment from the previous master when using source route bridges, a mechanism is required to update cached source route information. |
| `10:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Security Considerations prose stating the negative, that no information in VRRP messages must be kept secret; lowercase must inside a denial of a requirement. | Confidentiality is not necessary for the correct operation of VRRP and there is no information in the VRRP messages that must be kept secret from other nodes on the LAN. |
| `15:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Intellectual Property Statement boilerplate. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

RFC 3768 is obsoleted by RFC 9568.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC3768-5.2.2-1`](#rfc3768-5.2.2-1) Routers MUST NOT forward a datagram with this destination address regardless of its TTL. (§5.2.2) | restated | RFC9568-5.1.1.2-1 | RFC 9568 Section 5.1.1.2 keeps the rule for IPv4 and adds the IPv6 counterpart at RFC9568-5.1.2.2-1 for ff02::12 |
| [`RFC3768-5.2.3-1`](#rfc3768-5.2.3-1) The TTL MUST be set to 255. (§5.2.3) | restated | RFC9568-5.1.1.3-1 | RFC 9568 Section 5.1.1.3 keeps the IPv4 TTL of 255 and adds the IPv6 Hop Limit counterpart at RFC9568-5.1.2.3-1 |
| [`RFC3768-5.2.3-2`](#rfc3768-5.2.3-2) A VRRP router receiving a packet with the TTL not equal to 255 MUST discard the packet. (§5.2.3, §7.1) | restated | RFC9568-5.1.1.3-2 | RFC 9568 Section 5.1.1.3 keeps the discard rule for IPv4 and adds the IPv6 Hop Limit counterpart at RFC9568-5.1.2.3-2 |
| [`RFC3768-5.3.2-1`](#rfc3768-5.3.2-1) A packet with unknown type MUST be discarded. (§5.3.2) | restated | RFC9568-5.2.2-1 | RFC 9568 Section 5.2.2 keeps ADVERTISEMENT as the only defined type and keeps the discard rule for any other value |
| [`RFC3768-5.3.4-1`](#rfc3768-5.3.4-1) The priority value for the VRRP router that owns the IP address(es) associated with the virtual router MUST be 255 (decimal). (§5.3.4) | restated | RFC9568-5.2.4-1 | RFC 9568 Section 5.2.4 keeps Priority 255 for the router owning the Virtual Router addresses, over both address families |
| [`RFC3768-5.3.4-2`](#rfc3768-5.3.4-2) VRRP routers backing up a virtual router MUST use priority values between 1-254 (decimal). (§5.3.4) | restated | RFC9568-5.2.4-2 | RFC 9568 Section 5.2.4 keeps the 1 to 254 range for backup routers |
| [`RFC3768-5.3.6-1`](#rfc3768-5.3.6-1) A packet with unknown authentication type or that does not match the locally configured authentication method MUST be discarded. (§5.3.6, §7.1) | dropped | not stated | VRRPv3 removes authentication. RFC 9568 Section 9 states that VRRP for IPvX does not currently include any type of authentication, and its packet format carries no Auth Type field, so no unknown-or-mismatched Auth Type can be received. The octet that held Auth Type in VRRPv2 is the Reserve field of RFC 9568 Section 5.2.6 |
| [`RFC3768-6.4.2-1`](#rfc3768-6.4.2-1) - MUST NOT respond to ARP requests for the IP address(s) associated with the virtual router. (§6.4.2) | restated | RFC9568-6.4.2-1 | RFC 9568 Section 6.4.2 keeps the rule for IPv4 ARP and adds the IPv6 counterparts, RFC9568-6.4.2-2 for Neighbor Solicitations and RFC9568-6.4.2-3 for Router Advertisements |
| [`RFC3768-6.4.2-2`](#rfc3768-6.4.2-2) - MUST discard packets with a destination link layer MAC address equal to the virtual router MAC address. (§6.4.2) | restated | RFC9568-6.4.2-4 | RFC 9568 Section 6.4.2 keeps the rule that a Backup Router discards packets whose destination link-layer address is the Virtual Router MAC |
| [`RFC3768-6.4.2-3`](#rfc3768-6.4.2-3) - MUST NOT accept packets addressed to the IP address(es) associated with the virtual router. (§6.4.2) | restated | RFC9568-6.4.2-5 | RFC 9568 Section 6.4.2 keeps the rule and states it over both address families |
| [`RFC3768-6.4.2-4`](#rfc3768-6.4.2-4) If a Shutdown event is received, then: o Cancel the Master_Down_Timer o Transition to the {Initialize} state (§6.4.2) | restated | RFC9568-6.4.2-6 | RFC 9568 Section 6.4.2 keeps the Shutdown transition and renames Master_Down_Timer to Active_Down_Timer |
| [`RFC3768-6.4.2-5`](#rfc3768-6.4.2-5) If the Master_Down_Timer fires, then: o Send an ADVERTISEMENT o Broadcast a gratuitous ARP request containing the virtual router MAC address for each IP address associated with the virtual router o Set the Adver_Timer to Advertisement_Interval o Transition to the {Master} state (§6.4.2) | restated | RFC9568-6.4.2-7 | RFC 9568 Section 6.4.2 keeps the whole timer-expiry sequence, renames the timer to Active_Down_Timer and the state to Active, and adds the IPv6 unsolicited Neighbor Advertisement beside the gratuitous ARP |
| [`RFC3768-6.4.2-6`](#rfc3768-6.4.2-6) If the Priority in the ADVERTISEMENT is Zero, then: o Set the Master_Down_Timer to Skew_Time (§6.4.2) | restated | RFC9568-6.4.2-8 | RFC 9568 Section 6.4.2 keeps the Skew_Time rule for a Priority 0 advertisement |
| [`RFC3768-6.4.2-7`](#rfc3768-6.4.2-7) If Preempt_Mode is False, or If the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: o Reset the Master_Down_Timer to Master_Down_Interval (§6.4.2) | restated | RFC9568-6.4.2-9 | RFC 9568 Section 6.4.2 keeps the rule and adds one step, that the Backup Router adopts the Max Advertise Interval from the advertisement as Active_Adver_Interval before recomputing the timers |
| [`RFC3768-6.4.2-8`](#rfc3768-6.4.2-8) If Preempt_Mode is False, or If the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: o Reset the Master_Down_Timer to Master_Down_Interval else: o Discard the ADVERTISEMENT (§6.4.2) | restated | RFC9568-6.4.2-10 | RFC 9568 Section 6.4.2 keeps the discard rule for a lower-priority advertisement under Preempt_Mode |
| [`RFC3768-6.4.3-1`](#rfc3768-6.4.3-1) - MUST respond to ARP requests for the IP address(es) associated with the virtual router. (§6.4.3, §8.2) | restated | RFC9568-6.4.3-1 | RFC 9568 Section 6.4.3 keeps the ARP obligation for IPv4 and adds the IPv6 Neighbor Discovery counterparts at RFC9568-6.4.3-2, RFC9568-6.4.3-3 and RFC9568-6.4.3-4 |
| [`RFC3768-6.4.3-2`](#rfc3768-6.4.3-2) - MUST forward packets with a destination link layer MAC address equal to the virtual router MAC address. (§6.4.3) | restated | RFC9568-6.4.3-5 | RFC 9568 Section 6.4.3 keeps the rule that the Active Router forwards packets whose destination link-layer address is the Virtual Router MAC |
| [`RFC3768-6.4.3-3`](#rfc3768-6.4.3-3) - MUST NOT accept packets addressed to the IP address(es) associated with the virtual router if it is not the IP address owner. (§6.4.3) | restated | RFC9568-6.4.3-7 | RFC 9568 Section 6.4.3 keeps the prohibition and narrows it, because VRRPv3 adds Accept_Mode: the Active Router refuses such packets only when it is neither the address owner nor configured with Accept_Mode True |
| [`RFC3768-6.4.3-4`](#rfc3768-6.4.3-4) - MUST accept packets addressed to the IP address(es) associated with the virtual router if it is the IP address owner. (§6.4.3) | restated | RFC9568-6.4.3-6 | RFC 9568 Section 6.4.3 keeps the owner case and widens it to Accept_Mode True |
| [`RFC3768-6.4.3-5`](#rfc3768-6.4.3-5) If a Shutdown event is received, then: o Cancel the Adver_Timer o Send an ADVERTISEMENT with Priority = 0 o Transition to the {Initialize} state (§6.4.3) | restated | RFC9568-6.4.3-8 | RFC 9568 Section 6.4.3 keeps the Shutdown sequence unchanged |
| [`RFC3768-6.4.3-6`](#rfc3768-6.4.3-6) If the Adver_Timer fires, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | restated | RFC9568-6.4.3-9 | RFC 9568 Section 6.4.3 keeps the Adver_Timer expiry behaviour unchanged |
| [`RFC3768-6.4.3-7`](#rfc3768-6.4.3-7) If the Priority in the ADVERTISEMENT is Zero, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | restated | RFC9568-6.4.3-10 | RFC 9568 Section 6.4.3 keeps the response to a Priority 0 advertisement unchanged |
| [`RFC3768-6.4.3-8`](#rfc3768-6.4.3-8) If the Priority in the ADVERTISEMENT is greater than the local Priority, or If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IP Address of the sender is greater than the local primary IP Address, then: o Cancel Adver_Timer o Set Master_Down_Timer to Master_Down_Interval o Transition to the {Backup} state (§6.4.3) | restated | RFC9568-6.4.3-11 | RFC 9568 Section 6.4.3 keeps the rule and states that the sender address comparison is an unsigned integer comparison in network byte order |
| [`RFC3768-6.4.3-9`](#rfc3768-6.4.3-9) If the Priority in the ADVERTISEMENT is greater than the local Priority, or If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IP Address of the sender is greater than the local primary IP Address, then: o Cancel Adver_Timer o Set Master_Down_Timer to Master_Down_Interval o Transition to the {Backup} state else: o Discard ADVERTISEMENT (§6.4.3) | restated | RFC9568-6.4.3-12 | RFC 9568 Section 6.4.3 keeps the discard and adds one step, that the Active Router immediately sends an advertisement so learning bridges relearn the segment. RFC 9568 Section 1.2 lists that addition as change 5 |
| [`RFC3768-7.1-1`](#rfc3768-7.1-1) MUST verify the VRRP version is 2. (§7.1) | restated | RFC9568-7.1-1 | RFC 9568 Section 7.1 keeps the version check and changes the value it verifies from 2 to 3 |
| [`RFC3768-7.1-2`](#rfc3768-7.1-2) MUST verify that the received packet contains the complete VRRP packet (including fixed fields, IP Address(es), and Authentication Data). (§7.1) | restated | RFC9568-7.1-3 | RFC 9568 Section 7.1 keeps the completeness check over the fixed fields and the address list. The Authentication Data half of the RFC 3768 check is gone with the field |
| [`RFC3768-7.1-3`](#rfc3768-7.1-3) MUST verify the VRRP checksum. (§7.1) | restated | RFC9568-5.2.8-1 | RFC 9568 moves the checksum definition to Section 5.2.8 and extends it, because the IPv6 checksum covers a pseudo-header. RFC 9568 Section 1.2 lists that as change 4 |
| [`RFC3768-7.1-4`](#rfc3768-7.1-4) MUST verify that the VRID is configured on the receiving interface and the local router is not the IP Address owner (Priority equals 255 (decimal)). (§7.1) | restated | RFC9568-7.1-4 | RFC 9568 Section 7.1 keeps both halves of the check, that the VRID is configured on the receiving interface and that the local router is not the address owner |
| [`RFC3768-7.1-5`](#rfc3768-7.1-5) MUST verify that the Auth Type matches the locally configured authentication method for the virtual router and perform that authentication method. (§7.1) | dropped | not stated | VRRPv3 removes authentication. RFC 9568 Section 7.1 lists no Auth Type check, its packet format carries no Auth Type field, and its Section 9 states that VRRP for IPvX does not currently include any type of authentication |
| [`RFC3768-7.1-6`](#rfc3768-7.1-6) If any one of the above checks fails, the receiver MUST discard the packet (§7.1) | restated | RFC9568-7.1-6 | RFC 9568 Section 7.1 keeps the discard on a failed mandatory check, and keeps the SHOULD log and MAY network-management indication beside it |
| [`RFC3768-7.1-7`](#rfc3768-7.1-7) If the packet was not generated by the address owner (Priority does not equal 255 (decimal)), the receiver MUST drop the packet, otherwise continue processing. (§7.1) | dropped | not stated | RFC 9568 Section 7.1 removes the drop. The address-list check is a MAY at RFC9568-7.1-11, and on failure the receiver only SHOULD log the event and MAY indicate a misconfiguration. No sender-priority condition and no packet drop remain |
| [`RFC3768-7.1-8`](#rfc3768-7.1-8) - MUST verify that the Adver Interval in the packet is the same as the locally configured for this virtual router (§7.1) | restated | RFC9568-7.1-8 | RFC 9568 Section 7.1 lowers the check from MUST to SHOULD and removes the drop, stating that the mismatch will not result in the VRRP packet being dropped. RFC 9568 Section 1.2 lists that as change 8. The field is also renamed and rescaled, from Adver Int in seconds to Max Advertise Interval in centiseconds |
| [`RFC3768-7.2-1`](#rfc3768-7.2-1) Fill in the VRRP packet fields with the appropriate virtual router configuration state - Compute the VRRP checksum (§7.2) | restated | RFC9568-7.2-1 | RFC 9568 Section 7.2 keeps the fill-and-checksum step unchanged |
| [`RFC3768-7.2-2`](#rfc3768-7.2-2) Set the source MAC address to Virtual Router MAC Address (§7.2) | restated | RFC9568-7.2-2 | RFC 9568 Section 7.2 keeps the Virtual Router MAC as the source link-layer address |
| [`RFC3768-7.2-3`](#rfc3768-7.2-3) Set the source IP address to interface primary IP address (§7.2) | restated | RFC9568-7.2-3 | RFC 9568 Section 7.2 keeps the interface primary IPv4 address and adds the IPv6 case, where the source is the interface link-local address |
| [`RFC3768-7.2-4`](#rfc3768-7.2-4) Set the IP protocol to VRRP - Send the VRRP packet to the VRRP IP multicast group (§7.2) | restated | RFC9568-7.2-4 | RFC 9568 Section 7.2 keeps IP protocol 112 and adds the IPv6 destination group ff02::12 beside 224.0.0.18 |
| [`RFC3768-8.1-1`](#rfc3768-8.1-1) If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.1; lowercase "must" in the RFC) | unextracted | §8.1.1 | RFC 9568 §8.1.1 restates it in the same words, and rfc/short/rfc9568.md declares no row for it |
| [`RFC3768-8.2-1`](#rfc3768-8.2-1) The Master virtual router MUST NOT respond with its physical MAC address. (§8.2) | restated | RFC9568-8.1.2-1 | RFC 9568 Section 8.1.2 keeps the prohibition for IPv4 ARP and adds the IPv6 Neighbor Discovery counterpart at RFC9568-8.2.2-1 |
| [`RFC3768-8.3-1`](#rfc3768-8.3-1) If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the Virtual Router MAC address in the Proxy ARP message. (§8.3; lowercase "must" in the RFC) | restated | RFC9568-8.1.3-1 | RFC 9568 Section 8.1.3 keeps the Proxy ARP obligation and states it with an uppercase MUST |
| [`RFC3768-9.2-1`](#rfc3768-9.2-1) The functional address mode of operation MUST be implemented by routers supporting VRRP on token ring. (§9.2) | dropped | not stated | RFC 9568 removes token ring. Its Section 1.2 lists as change 6 that the appendices describing operation over legacy technologies, FDDI, Token Ring and ATM LAN Emulation, were removed, so RFC 9568 states no functional-address obligation |
| [`RFC3768-5.3.10-1`](#rfc3768-5.3.10-1) The authentication string is currently only used to maintain backwards compatibility with RFC 2338. It SHOULD be set to zero on transmission and ignored on reception. (§5.3.10) | dropped | not stated | VRRPv3 removes the Authentication Data field, so no obligation about its value remains. RFC 9568 Section 5.2.6 states the same zero-on-transmission and ignore-on-reception rule for its Reserve field at RFC9568-5.2.6-1, which is a different field: it occupies the octet VRRPv2 used for Auth Type |
| [`RFC3768-7.1-9`](#rfc3768-7.1-9) If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event (§7.1) | restated | RFC9568-7.1-7 | RFC 9568 Section 7.1 keeps the log on a failed mandatory check and adds rate-limiting to it |
| [`RFC3768-7.1-10`](#rfc3768-7.1-10) MAY verify that "Count IP Addrs" and the list of IP Address matches the IP_Addresses configured for the VRID If the above check fails, the receiver SHOULD log the event (§7.1) | restated | RFC9568-7.1-9 | RFC 9568 Section 7.1 keeps the log on an address-list mismatch, adds rate-limiting, and states it in one sentence covering the Max Advertise Interval mismatch as well |
| [`RFC3768-7.1-11`](#rfc3768-7.1-11) MUST verify that the Adver Interval in the packet is the same as the locally configured for this virtual router If the above check fails, the receiver MUST discard the packet, SHOULD log the event (§7.1) | restated | RFC9568-7.1-9 | RFC 9568 Section 7.1 merges this log with the address-list one into a single rate-limited recommendation, and renames the field to Max Advertise Interval |
| [`RFC3768-8.2-2`](#rfc3768-8.2-2) When a VRRP router restarts or boots, it SHOULD not send any ARP messages with its physical MAC address for the IP address it owns, it should only send ARP messages that include Virtual MAC addresses. (§8.2) | restated | RFC9568-8.1.2-3 | RFC 9568 Section 8.1.2 keeps the rule for IPv4 ARP and adds the IPv6 Neighbor Discovery counterpart at RFC9568-8.2.2-5 |
| [`RFC3768-8.2-3`](#rfc3768-8.2-3) When configuring an interface, VRRP routers should broadcast a gratuitous ARP request containing the virtual router MAC address for each IP address on that interface. - At system boot, when initializing interfaces for VRRP operation; delay gratuitous ARP requests and ARP responses until both the IP address and the virtual router MAC address are configured. (§8.2) | restated | RFC9568-8.1.2-4 | RFC 9568 Section 8.1.2 splits the sentence in two and raises one half. The gratuitous ARP on interface configuration stays a SHOULD at RFC9568-8.1.2-4, and the boot delay until both the address and the Virtual Router MAC are configured becomes a MUST at RFC9568-8.1.2-2 |
| [`RFC3768-8.4-1`](#rfc3768-8.4-1) A VRRP router SHOULD not forward packets addressed to the IP Address(es) it becomes Master for if it is not the owner. (§8.4) | restated | RFC9568-8.3.1-1 | RFC 9568 Section 8.3.1 keeps the rule and states it over both address families |
| [`RFC3768-9.1-1`](#rfc3768-9.1-1) To avoid this an implementation SHOULD configure the virtual router MAC address by adding a unicast MAC filter in the FDDI device, rather than changing its hardware MAC address. (§9.1) | dropped | not stated | RFC 9568 removes FDDI. Its Section 1.2 lists as change 6 that the appendices describing operation over legacy technologies, FDDI, Token Ring and ATM LAN Emulation, were removed, so RFC 9568 states no unicast-MAC-filter recommendation |
| [`RFC3768-7.1-12`](#rfc3768-7.1-12) MAY indicate via network management that an error occurred. - MAY verify that "Count IP Addrs" and the list of IP Address matches the IP_Addresses configured for the VRID If the above check fails, the receiver SHOULD log the event and MAY indicate via network management that a misconfiguration was detected. (§7.1) | restated | RFC9568-7.1-10 | RFC 9568 Section 7.1 keeps the network-management indication as a MAY |
| [`RFC3768-7.1-13`](#rfc3768-7.1-13) MAY verify that "Count IP Addrs" and the list of IP Address matches the IP_Addresses configured for the VRID (§7.1) | restated | RFC9568-7.1-11 | RFC 9568 Section 7.1 keeps the address-list check as a MAY and renames the field to IPvX Addr Count |
| [`RFC3768-9.2-2`](#rfc3768-9.2-2) Additionally, routers MAY support unicast mode of operation to take advantage of newer token ring adapter implementations that support non-promiscuous reception for multiple unicast MAC addresses and to avoid both the multicast traffic and usage conflicts associated with the use of token ring functional addresses. (§9.2) | dropped | not stated | RFC 9568 removes token ring. Its Section 1.2 lists as change 6 that the appendices describing operation over legacy technologies, FDDI, Token Ring and ATM LAN Emulation, were removed, so RFC 9568 states no unicast-mode permission |
