# RFC 3768 - Virtual Router Redundancy Protocol (VRRP)

Experimental. Every requirement this repository extracted from RFC 3768, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 86.1% | 31 of 36 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 13.9% | 5 of 36 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 36 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 36 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 69 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 39 | of 50 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 39 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 36 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 50 |
| Gated MUST-level | 39 |
| Obligations that bind Ze | 36 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 69 |
| Tagged units | 69 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3768.md` |
| Requirement shard | `rfc/requirements/rfc3768.md` |
| RFC text | `rfc/full/rfc3768.txt` |

## Enrolment

Enrolled: Virtual Router Redundancy Protocol v2 / VRRP (RFC 3768): 30 MET (advertisement format, priority/master election, skew time, virtual MAC, gratuitous ARP, adoption) + 5 single-polarity positive + 1 gap (accept-mode) + 3 not-applicable (deprecated authentication)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

Opt-in via `version 2`. Whole-second Advertisement_Interval encoding, v2 advert format, the v2 receive-validation ladder (version, complete-packet, checksum, VRID, Auth Type 0, interval-mismatch discard, address-list discard), the Section 6.4 state machine (priority/master election, skew time, preemption, silent losing-advert discard), virtual-MAC ownership of the VIP via a per-group macvlan, and the v2 rejection rules (no accept-mode, no IPv6).

**What the ledger says remains:**

No gap gated in [`rfc/short/rfc3768.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc3768.md). RFC 3768 authentication types are deliberately not implemented: RFC 9568 Section 9 removed them as providing no real security. Same VRRP experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 31 | one part of the gated population |
| Annotated instead of tested | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **39** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (31):** [`RFC3768-5.2.3-2`](#rfc3768-5.2.3-2), [`RFC3768-5.3.2-1`](#rfc3768-5.3.2-1), [`RFC3768-5.3.4-1`](#rfc3768-5.3.4-1), [`RFC3768-5.3.4-2`](#rfc3768-5.3.4-2), [`RFC3768-5.3.6-1`](#rfc3768-5.3.6-1), [`RFC3768-6.4.2-1`](#rfc3768-6.4.2-1), [`RFC3768-6.4.2-2`](#rfc3768-6.4.2-2), [`RFC3768-6.4.2-3`](#rfc3768-6.4.2-3), [`RFC3768-6.4.2-4`](#rfc3768-6.4.2-4), [`RFC3768-6.4.2-5`](#rfc3768-6.4.2-5), [`RFC3768-6.4.2-6`](#rfc3768-6.4.2-6), [`RFC3768-6.4.2-7`](#rfc3768-6.4.2-7), [`RFC3768-6.4.2-8`](#rfc3768-6.4.2-8), [`RFC3768-6.4.3-1`](#rfc3768-6.4.3-1), [`RFC3768-6.4.3-2`](#rfc3768-6.4.3-2), [`RFC3768-6.4.3-3`](#rfc3768-6.4.3-3), [`RFC3768-6.4.3-4`](#rfc3768-6.4.3-4), [`RFC3768-6.4.3-5`](#rfc3768-6.4.3-5), [`RFC3768-6.4.3-6`](#rfc3768-6.4.3-6), [`RFC3768-6.4.3-7`](#rfc3768-6.4.3-7), [`RFC3768-6.4.3-8`](#rfc3768-6.4.3-8), [`RFC3768-6.4.3-9`](#rfc3768-6.4.3-9), [`RFC3768-7.1-1`](#rfc3768-7.1-1), [`RFC3768-7.1-2`](#rfc3768-7.1-2), [`RFC3768-7.1-3`](#rfc3768-7.1-3), [`RFC3768-7.1-4`](#rfc3768-7.1-4), [`RFC3768-7.1-5`](#rfc3768-7.1-5), [`RFC3768-7.1-6`](#rfc3768-7.1-6), [`RFC3768-7.1-7`](#rfc3768-7.1-7), [`RFC3768-7.1-8`](#rfc3768-7.1-8), [`RFC3768-8.2-1`](#rfc3768-8.2-1)

**Annotated instead of tested (8):** [`RFC3768-5.2.2-1`](#rfc3768-5.2.2-1), [`RFC3768-5.2.3-1`](#rfc3768-5.2.3-1), [`RFC3768-7.2-1`](#rfc3768-7.2-1), [`RFC3768-7.2-2`](#rfc3768-7.2-2), [`RFC3768-7.2-3`](#rfc3768-7.2-3), [`RFC3768-7.2-4`](#rfc3768-7.2-4), [`RFC3768-8.3-1`](#rfc3768-8.3-1), [`RFC3768-9.2-1`](#rfc3768-9.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3768-5.2.2-1` | Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.2.2) | MUST NOT | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the VRRP plugin performs no IP datagram forwarding -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes each received advert into the FSM and never re-emits it, and tx scopes adverts to link-local multicast with IP_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:143 |
| `RFC3768-5.2.3-1` | Set the IP TTL of transmitted VRRP packets to 255 (§5.2.3) | MUST | 5.2.3 | **positive:** `unit/verify` [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L259). **negative:** no negative test. **{single-polarity}:** buildIPv4Header unconditionally sets TTL 255 at internal/plugins/vrrp/transport/transport.go:562, so no input yields a different TTL -- the rx TTL!=255 discard is the separate RFC3768-5.2.3-2 |
| `RFC3768-5.2.3-2` | Discard received VRRP packets whose TTL is not equal to 255 (§5.2.3, §7.1) | MUST | 5.2.3 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L48). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L526) |
| `RFC3768-5.3.2-1` | Discard packets with unknown Type; only 1 = ADVERTISEMENT is defined (§5.3.2) | MUST | 5.3.2 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L49). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L145) |
| `RFC3768-5.3.4-1` | Use Priority 255 for the VRRP router that owns the virtual router's IP address(es) (§5.3.4) | MUST | 5.3.4 | **positive:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L764). **negative:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L765) |
| `RFC3768-5.3.4-2` | Use Priority values 1-254 for VRRP routers backing up a virtual router (§5.3.4) | MUST | 5.3.4 | **positive:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L315). **negative:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L316) |
| `RFC3768-5.3.6-1` | Discard packets with unknown Auth Type or an Auth Type that does not match the locally configured authentication method (§5.3.6, §7.1) | MUST | 5.3.6 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L50). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L590) |
| `RFC3768-6.4.2-1` | Backup: never respond to ARP requests for the IP address(es) associated with the virtual router (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L316). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L356) |
| `RFC3768-6.4.2-2` | Backup: discard packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L317). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L357) |
| `RFC3768-6.4.2-3` | Backup: never accept packets addressed to the IP address(es) associated with the virtual router (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L318). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L358) |
| `RFC3768-6.4.2-4` | Backup: on Shutdown, cancel the Master_Down_Timer and transition to Initialize (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L88). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L89) |
| `RFC3768-6.4.2-5` | Backup: when the Master_Down_Timer fires, send an ADVERTISEMENT, broadcast a gratuitous ARP with the virtual router MAC for each virtual IP address, set the Adver_Timer to Advertisement_Interval, and transition to Master (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L429). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L618) |
| `RFC3768-6.4.2-6` | Backup: on an ADVERTISEMENT with Priority 0, set the Master_Down_Timer to Skew_Time (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L90). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L91) |
| `RFC3768-6.4.2-7` | Backup: on a non-zero-priority ADVERTISEMENT, if Preempt_Mode is False or the advertised Priority >= local Priority, reset the Master_Down_Timer to Master_Down_Interval (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L92). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L93) |
| `RFC3768-6.4.2-8` | Backup: on a non-zero-priority ADVERTISEMENT with Preempt_Mode True and advertised Priority < local Priority, discard the ADVERTISEMENT (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L94). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L95) |
| `RFC3768-6.4.3-1` | Master: respond to ARP requests for the IP address(es) associated with the virtual router, answering with the virtual MAC address (§6.4.3, §8.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L61). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L120) |
| `RFC3768-6.4.3-2` | Master: forward packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L354). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L319) |
| `RFC3768-6.4.3-3` | Master: never accept packets addressed to the virtual router IP address(es) when not the IP address owner (§6.4.3) | MUST NOT | 6.4.3 | **positive:** `unit/verify` [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L369). **negative:** `unit/verify` [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L370) |
| `RFC3768-6.4.3-4` | Master: accept packets addressed to the virtual router IP address(es) when the IP address owner (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L355). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L320) |
| `RFC3768-6.4.3-5` | Master: on Shutdown, cancel the Adver_Timer, send an ADVERTISEMENT with Priority = 0, and transition to Initialize (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L96). **positive:** `unit/verify` [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L429). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L97) |
| `RFC3768-6.4.3-6` | Master: when the Adver_Timer fires, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L98). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L619) |
| `RFC3768-6.4.3-7` | Master: on an ADVERTISEMENT with Priority 0, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L99). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L100) |
| `RFC3768-6.4.3-8` | Master: on an ADVERTISEMENT with higher Priority, or equal Priority and greater sender primary IP address, cancel the Adver_Timer, set the Master_Down_Timer to Master_Down_Interval, and transition to Backup (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L101). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L102) |
| `RFC3768-6.4.3-9` | Master: on a losing ADVERTISEMENT (lower Priority, or equal Priority with smaller sender address), discard it (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L103). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L104) |
| `RFC3768-7.1-1` | Rx: verify the VRRP version is 2 (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L42). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L601) |
| `RFC3768-7.1-2` | Rx: verify the received packet contains the complete VRRP packet, including fixed fields, IP address(es), and Authentication Data (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L43). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L212) |
| `RFC3768-7.1-3` | Rx: verify the VRRP checksum (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L44). **negative:** `unit/verify` [`TestDecodeV2ChecksumCorrupt`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L300) |
| `RFC3768-7.1-4` | Rx: verify the VRID is configured on the receiving interface and the local router is not the IP address owner (Priority = 255) (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L45). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L157) |
| `RFC3768-7.1-5` | Rx: verify the Auth Type matches the locally configured authentication method and perform that method (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L46). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L591) |
| `RFC3768-7.1-6` | Rx: discard the packet if any mandatory receive check fails (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L472). **negative:** `unit/verify` [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L453) |
| `RFC3768-7.1-7` | Rx: if the optional address-list check fails and the sender is not the address owner (Priority != 255), drop the packet (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceV2AddressListMatchReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L553). **negative:** `unit/verify` [`TestInstanceV2AddressListMismatchDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L514) |
| `RFC3768-7.1-8` | Rx: verify the Adver Interval in the packet equals the locally configured value; discard the packet on mismatch (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L47). **negative:** `unit/verify` [`TestDecodeV2IntervalMismatchDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L287). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L245) |
| `RFC3768-7.2-1` | Tx: fill in the VRRP fields from the virtual router configuration state and compute the VRRP checksum (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestEncodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L87). **negative:** no negative test. **{single-polarity}:** a golden encode pins every field and the checksum -- WriteTo internal/plugins/vrrp/packet/packet.go:251 plus FillChecksum internal/plugins/vrrp/packet/checksum.go:86 -- while a corrupted-encoding rejection is the separate receive requirement RFC3768-7.1-3 |
| `RFC3768-7.2-2` | Tx: set the source MAC address to the virtual router MAC address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L383). **negative:** no negative test. **{single-polarity}:** the source MAC is the virtual-router MAC from packet.VirtualMAC internal/plugins/vrrp/packet/packet.go:97 egressed by binding the tx socket to the vMAC macvlan internal/plugins/vrrp/transport/backend_linux.go:133, a deterministic derivation with no input that yields a different MAC |
| `RFC3768-7.2-3` | Tx: set the source IP address to the interface primary IP address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L202). **negative:** no negative test. **{single-polarity}:** the source IP is the parent unit primary IPv4 from resolveParentPrimaryV4 internal/plugins/vrrp/transport/transport.go:573, a deterministic selection re-resolved on address change, with no input that yields a wrong-source advert |
| `RFC3768-7.2-4` | Tx: set the IP protocol to VRRP (112) and send to the VRRP IP multicast group 224.0.0.18 (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L260). **negative:** no negative test. **{single-polarity}:** buildIPv4Header sets IP protocol 112 at internal/plugins/vrrp/transport/transport.go:563 and SendAdvert targets 224.0.0.18 at internal/plugins/vrrp/transport/backend_linux.go:256, both constants with no input that changes them |
| `RFC3768-8.2-1` | Master: never respond to host ARP requests for virtual addresses with the physical MAC address (§8.2) | MUST NOT | 8.2 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L62). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L121) |
| `RFC3768-8.3-1` | Advertise the virtual router MAC address in Proxy ARP messages sent on behalf of VRRP-protected addresses (§8.3; lowercase "must" in the RFC) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no proxy ARP for virtual addresses -- the per-group virtual-MAC macvlan answers ARP for the VIP directly (createMacvlan internal/plugins/vrrp/register.go:329 plus the sole-responder sysctl recipe internal/plugins/vrrp/dataplane_linux.go:64), and no proxy-ARP path exists |
| `RFC3768-9.2-1` | Token ring: implement the functional-address mode of operation when supporting VRRP on token ring (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze supports only Ethernet-family parents (ethernet, veth, bridge, dummy -- internal/plugins/vrrp/groups.go:76) over AF_PACKET macvlan transport internal/plugins/vrrp/transport/backend_linux.go, with no token-ring transport, so the functional-address mode has no applicable code path |
| `RFC3768-5.3.10-1` | Set Authentication Data to zero on transmission and ignore it on reception (§5.3.10) | SHOULD | 5.3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-7.1-9` | Log the event when a mandatory receive check fails (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-7.1-10` | Log the event on an address-list mismatch (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-7.1-11` | Log the event on an Adver Interval mismatch (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-8.2-2` | After restart/boot, never send ARP messages with the physical MAC address for owned virtual IP addresses (§8.2) | SHOULD NOT | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-8.2-3` | Broadcast a gratuitous ARP with the virtual router MAC for each IP address when configuring an interface, and delay ARP at boot until both the IP address and virtual MAC are configured (§8.2) | SHOULD | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-8.4-1` | Never forward packets addressed to IP addresses adopted as Master when not the owner (§8.4) | SHOULD NOT | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-9.1-1` | FDDI: configure the virtual router MAC via a unicast MAC filter rather than changing the hardware MAC address (§9.1) | SHOULD | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-7.1-12` | Indicate via network management that a receive error or misconfiguration occurred (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-7.1-13` | Verify that Count IP Addrs and the address list match the IP_Addresses configured for the VRID (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3768-9.2-2` | Token ring: support the unicast mode of operation in addition to functional addresses (§9.2) | MAY | 9.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3768-5.2.2-1`](#rfc3768-5.2.2-1) Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: the VRRP plugin performs no IP datagram forwarding -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes each received advert into the FSM and never re-emits it, and tx scopes adverts to link-local multicast with IP_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:143 |
| [`RFC3768-8.3-1`](#rfc3768-8.3-1) Advertise the virtual router MAC address in Proxy ARP messages sent on behalf of VRRP-protected addresses (§8.3; lowercase "must" in the RFC) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no proxy ARP for virtual addresses -- the per-group virtual-MAC macvlan answers ARP for the VIP directly (createMacvlan internal/plugins/vrrp/register.go:329 plus the sole-responder sysctl recipe internal/plugins/vrrp/dataplane_linux.go:64), and no proxy-ARP path exists |
| [`RFC3768-9.2-1`](#rfc3768-9.2-1) Token ring: implement the functional-address mode of operation when supporting VRRP on token ring (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze supports only Ethernet-family parents (ethernet, veth, bridge, dummy -- internal/plugins/vrrp/groups.go:76) over AF_PACKET macvlan transport internal/plugins/vrrp/transport/backend_linux.go, with no token-ring transport, so the functional-address mode has no applicable code path |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3768-5.2.2-1`](#rfc3768-5.2.2-1)

Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3768-5.2.2-1, so no unit is bound to it.

### [`RFC3768-5.2.3-1`](#rfc3768-5.2.3-1)

Set the IP TTL of transmitted VRRP packets to 255 (§5.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L259) | unit/verify | unproven |

### [`RFC3768-5.2.3-2`](#rfc3768-5.2.3-2)

Discard received VRRP packets whose TTL is not equal to 255 (§5.2.3, §7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L526) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L48) | unit/verify | unproven |

### [`RFC3768-5.3.2-1`](#rfc3768-5.3.2-1)

Discard packets with unknown Type; only 1 = ADVERTISEMENT is defined (§5.3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L145) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L49) | unit/verify | unproven |

### [`RFC3768-5.3.4-1`](#rfc3768-5.3.4-1)

Use Priority 255 for the VRRP router that owns the virtual router's IP address(es) (§5.3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L765) | unit/verify | unproven |
| positive | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L764) | unit/verify | unproven |

### [`RFC3768-5.3.4-2`](#rfc3768-5.3.4-2)

Use Priority values 1-254 for VRRP routers backing up a virtual router (§5.3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L316) | unit/verify | unproven |
| positive | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L315) | unit/verify | unproven |

### [`RFC3768-5.3.6-1`](#rfc3768-5.3.6-1)

Discard packets with unknown Auth Type or an Auth Type that does not match the locally configured authentication method (§5.3.6, §7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L590) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L50) | unit/verify | unproven |

### [`RFC3768-6.4.2-1`](#rfc3768-6.4.2-1)

Backup: never respond to ARP requests for the IP address(es) associated with the virtual router (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L356) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L316) | unit/verify | unproven |

### [`RFC3768-6.4.2-2`](#rfc3768-6.4.2-2)

Backup: discard packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L357) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L317) | unit/verify | unproven |

### [`RFC3768-6.4.2-3`](#rfc3768-6.4.2-3)

Backup: never accept packets addressed to the IP address(es) associated with the virtual router (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L358) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L318) | unit/verify | unproven |

### [`RFC3768-6.4.2-4`](#rfc3768-6.4.2-4)

Backup: on Shutdown, cancel the Master_Down_Timer and transition to Initialize (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L89) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L88) | unit/verify | unproven |

### [`RFC3768-6.4.2-5`](#rfc3768-6.4.2-5)

Backup: when the Master_Down_Timer fires, send an ADVERTISEMENT, broadcast a gratuitous ARP with the virtual router MAC for each virtual IP address, set the Adver_Timer to Advertisement_Interval, and transition to Master (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L618) | unit/verify | unproven |
| positive | [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L429) | unit/verify | unproven |

### [`RFC3768-6.4.2-6`](#rfc3768-6.4.2-6)

Backup: on an ADVERTISEMENT with Priority 0, set the Master_Down_Timer to Skew_Time (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L91) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L90) | unit/verify | unproven |

### [`RFC3768-6.4.2-7`](#rfc3768-6.4.2-7)

Backup: on a non-zero-priority ADVERTISEMENT, if Preempt_Mode is False or the advertised Priority >= local Priority, reset the Master_Down_Timer to Master_Down_Interval (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L93) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L92) | unit/verify | unproven |

### [`RFC3768-6.4.2-8`](#rfc3768-6.4.2-8)

Backup: on a non-zero-priority ADVERTISEMENT with Preempt_Mode True and advertised Priority < local Priority, discard the ADVERTISEMENT (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L95) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L94) | unit/verify | unproven |

### [`RFC3768-6.4.3-1`](#rfc3768-6.4.3-1)

Master: respond to ARP requests for the IP address(es) associated with the virtual router, answering with the virtual MAC address (§6.4.3, §8.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L120) | unit/verify | unproven |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L61) | unit/verify | unproven |

### [`RFC3768-6.4.3-2`](#rfc3768-6.4.3-2)

Master: forward packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L319) | unit/verify | unproven |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L354) | unit/verify | unproven |

### [`RFC3768-6.4.3-3`](#rfc3768-6.4.3-3)

Master: never accept packets addressed to the virtual router IP address(es) when not the IP address owner (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L370) | unit/verify | unproven |
| positive | [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L369) | unit/verify | unproven |

### [`RFC3768-6.4.3-4`](#rfc3768-6.4.3-4)

Master: accept packets addressed to the virtual router IP address(es) when the IP address owner (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L320) | unit/verify | unproven |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L355) | unit/verify | unproven |

### [`RFC3768-6.4.3-5`](#rfc3768-6.4.3-5)

Master: on Shutdown, cancel the Adver_Timer, send an ADVERTISEMENT with Priority = 0, and transition to Initialize (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L97) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L96) | unit/verify | unproven |
| positive | [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L429) | unit/verify | unproven |

### [`RFC3768-6.4.3-6`](#rfc3768-6.4.3-6)

Master: when the Adver_Timer fires, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L619) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L98) | unit/verify | unproven |

### [`RFC3768-6.4.3-7`](#rfc3768-6.4.3-7)

Master: on an ADVERTISEMENT with Priority 0, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L100) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L99) | unit/verify | unproven |

### [`RFC3768-6.4.3-8`](#rfc3768-6.4.3-8)

Master: on an ADVERTISEMENT with higher Priority, or equal Priority and greater sender primary IP address, cancel the Adver_Timer, set the Master_Down_Timer to Master_Down_Interval, and transition to Backup (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L102) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L101) | unit/verify | unproven |

### [`RFC3768-6.4.3-9`](#rfc3768-6.4.3-9)

Master: on a losing ADVERTISEMENT (lower Priority, or equal Priority with smaller sender address), discard it (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L104) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L103) | unit/verify | unproven |

### [`RFC3768-7.1-1`](#rfc3768-7.1-1)

Rx: verify the VRRP version is 2 (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L601) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L42) | unit/verify | unproven |

### [`RFC3768-7.1-2`](#rfc3768-7.1-2)

Rx: verify the received packet contains the complete VRRP packet, including fixed fields, IP address(es), and Authentication Data (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L212) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L43) | unit/verify | unproven |

### [`RFC3768-7.1-3`](#rfc3768-7.1-3)

Rx: verify the VRRP checksum (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV2ChecksumCorrupt`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L300) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L44) | unit/verify | unproven |

### [`RFC3768-7.1-4`](#rfc3768-7.1-4)

Rx: verify the VRID is configured on the receiving interface and the local router is not the IP address owner (Priority = 255) (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L157) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L45) | unit/verify | unproven |

### [`RFC3768-7.1-5`](#rfc3768-7.1-5)

Rx: verify the Auth Type matches the locally configured authentication method and perform that method (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L591) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L46) | unit/verify | unproven |

### [`RFC3768-7.1-6`](#rfc3768-7.1-6)

Rx: discard the packet if any mandatory receive check fails (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L453) | unit/verify | unproven |
| positive | [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L472) | unit/verify | unproven |

### [`RFC3768-7.1-7`](#rfc3768-7.1-7)

Rx: if the optional address-list check fails and the sender is not the address owner (Priority != 255), drop the packet (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceV2AddressListMismatchDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L514) | unit/verify | unproven |
| positive | [`TestInstanceV2AddressListMatchReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L553) | unit/verify | unproven |

### [`RFC3768-7.1-8`](#rfc3768-7.1-8)

Rx: verify the Adver Interval in the packet equals the locally configured value; discard the packet on mismatch (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV2IntervalMismatchDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L287) | unit/verify | unproven |
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L245) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L47) | unit/verify | unproven |

### [`RFC3768-7.2-1`](#rfc3768-7.2-1)

Tx: fill in the VRRP fields from the virtual router configuration state and compute the VRRP checksum (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L87) | unit/verify | unproven |

### [`RFC3768-7.2-2`](#rfc3768-7.2-2)

Tx: set the source MAC address to the virtual router MAC address (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L383) | unit/verify | unproven |

### [`RFC3768-7.2-3`](#rfc3768-7.2-3)

Tx: set the source IP address to the interface primary IP address (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L202) | unit/verify | unproven |

### [`RFC3768-7.2-4`](#rfc3768-7.2-4)

Tx: set the IP protocol to VRRP (112) and send to the VRRP IP multicast group 224.0.0.18 (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L260) | unit/verify | unproven |

### [`RFC3768-8.2-1`](#rfc3768-8.2-1)

Master: never respond to host ARP requests for virtual addresses with the physical MAC address (§8.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L121) | unit/verify | unproven |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L62) | unit/verify | unproven |

### [`RFC3768-8.3-1`](#rfc3768-8.3-1)

Advertise the virtual router MAC address in Proxy ARP messages sent on behalf of VRRP-protected addresses (§8.3; lowercase "must" in the RFC)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3768-8.3-1, so no unit is bound to it.

### [`RFC3768-9.2-1`](#rfc3768-9.2-1)

Token ring: implement the functional-address mode of operation when supporting VRRP on token ring (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3768-9.2-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 3768, so no reviewer has walked its text sentence by sentence.

## Superseded

RFC 3768 is obsoleted by RFC 9568.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC3768-5.2.2-1`](#rfc3768-5.2.2-1) Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.2.2) | restated | RFC9568-5.1.1.2-1 | RFC 9568 Section 5.1.1.2 keeps the rule for IPv4 and adds the IPv6 counterpart at RFC9568-5.1.2.2-1 for ff02::12 |
| [`RFC3768-5.2.3-1`](#rfc3768-5.2.3-1) Set the IP TTL of transmitted VRRP packets to 255 (§5.2.3) | restated | RFC9568-5.1.1.3-1 | RFC 9568 Section 5.1.1.3 keeps the IPv4 TTL of 255 and adds the IPv6 Hop Limit counterpart at RFC9568-5.1.2.3-1 |
| [`RFC3768-5.2.3-2`](#rfc3768-5.2.3-2) Discard received VRRP packets whose TTL is not equal to 255 (§5.2.3, §7.1) | restated | RFC9568-5.1.1.3-2 | RFC 9568 Section 5.1.1.3 keeps the discard rule for IPv4 and adds the IPv6 Hop Limit counterpart at RFC9568-5.1.2.3-2 |
| [`RFC3768-5.3.2-1`](#rfc3768-5.3.2-1) Discard packets with unknown Type; only 1 = ADVERTISEMENT is defined (§5.3.2) | restated | RFC9568-5.2.2-1 | RFC 9568 Section 5.2.2 keeps ADVERTISEMENT as the only defined type and keeps the discard rule for any other value |
| [`RFC3768-5.3.4-1`](#rfc3768-5.3.4-1) Use Priority 255 for the VRRP router that owns the virtual router's IP address(es) (§5.3.4) | restated | RFC9568-5.2.4-1 | RFC 9568 Section 5.2.4 keeps Priority 255 for the router owning the Virtual Router addresses, over both address families |
| [`RFC3768-5.3.4-2`](#rfc3768-5.3.4-2) Use Priority values 1-254 for VRRP routers backing up a virtual router (§5.3.4) | restated | RFC9568-5.2.4-2 | RFC 9568 Section 5.2.4 keeps the 1 to 254 range for backup routers |
| [`RFC3768-5.3.6-1`](#rfc3768-5.3.6-1) Discard packets with unknown Auth Type or an Auth Type that does not match the locally configured authentication method (§5.3.6, §7.1) | dropped | not stated | VRRPv3 removes authentication. RFC 9568 Section 9 states that VRRP for IPvX does not currently include any type of authentication, and its packet format carries no Auth Type field, so no unknown-or-mismatched Auth Type can be received. The octet that held Auth Type in VRRPv2 is the Reserve field of RFC 9568 Section 5.2.6 |
| [`RFC3768-6.4.2-1`](#rfc3768-6.4.2-1) Backup: never respond to ARP requests for the IP address(es) associated with the virtual router (§6.4.2) | restated | RFC9568-6.4.2-1 | RFC 9568 Section 6.4.2 keeps the rule for IPv4 ARP and adds the IPv6 counterparts, RFC9568-6.4.2-2 for Neighbor Solicitations and RFC9568-6.4.2-3 for Router Advertisements |
| [`RFC3768-6.4.2-2`](#rfc3768-6.4.2-2) Backup: discard packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.2) | restated | RFC9568-6.4.2-4 | RFC 9568 Section 6.4.2 keeps the rule that a Backup Router discards packets whose destination link-layer address is the Virtual Router MAC |
| [`RFC3768-6.4.2-3`](#rfc3768-6.4.2-3) Backup: never accept packets addressed to the IP address(es) associated with the virtual router (§6.4.2) | restated | RFC9568-6.4.2-5 | RFC 9568 Section 6.4.2 keeps the rule and states it over both address families |
| [`RFC3768-6.4.2-4`](#rfc3768-6.4.2-4) Backup: on Shutdown, cancel the Master_Down_Timer and transition to Initialize (§6.4.2) | restated | RFC9568-6.4.2-6 | RFC 9568 Section 6.4.2 keeps the Shutdown transition and renames Master_Down_Timer to Active_Down_Timer |
| [`RFC3768-6.4.2-5`](#rfc3768-6.4.2-5) Backup: when the Master_Down_Timer fires, send an ADVERTISEMENT, broadcast a gratuitous ARP with the virtual router MAC for each virtual IP address, set the Adver_Timer to Advertisement_Interval, and transition to Master (§6.4.2) | restated | RFC9568-6.4.2-7 | RFC 9568 Section 6.4.2 keeps the whole timer-expiry sequence, renames the timer to Active_Down_Timer and the state to Active, and adds the IPv6 unsolicited Neighbor Advertisement beside the gratuitous ARP |
| [`RFC3768-6.4.2-6`](#rfc3768-6.4.2-6) Backup: on an ADVERTISEMENT with Priority 0, set the Master_Down_Timer to Skew_Time (§6.4.2) | restated | RFC9568-6.4.2-8 | RFC 9568 Section 6.4.2 keeps the Skew_Time rule for a Priority 0 advertisement |
| [`RFC3768-6.4.2-7`](#rfc3768-6.4.2-7) Backup: on a non-zero-priority ADVERTISEMENT, if Preempt_Mode is False or the advertised Priority >= local Priority, reset the Master_Down_Timer to Master_Down_Interval (§6.4.2) | restated | RFC9568-6.4.2-9 | RFC 9568 Section 6.4.2 keeps the rule and adds one step, that the Backup Router adopts the Max Advertise Interval from the advertisement as Active_Adver_Interval before recomputing the timers |
| [`RFC3768-6.4.2-8`](#rfc3768-6.4.2-8) Backup: on a non-zero-priority ADVERTISEMENT with Preempt_Mode True and advertised Priority < local Priority, discard the ADVERTISEMENT (§6.4.2) | restated | RFC9568-6.4.2-10 | RFC 9568 Section 6.4.2 keeps the discard rule for a lower-priority advertisement under Preempt_Mode |
| [`RFC3768-6.4.3-1`](#rfc3768-6.4.3-1) Master: respond to ARP requests for the IP address(es) associated with the virtual router, answering with the virtual MAC address (§6.4.3, §8.2) | restated | RFC9568-6.4.3-1 | RFC 9568 Section 6.4.3 keeps the ARP obligation for IPv4 and adds the IPv6 Neighbor Discovery counterparts at RFC9568-6.4.3-2, RFC9568-6.4.3-3 and RFC9568-6.4.3-4 |
| [`RFC3768-6.4.3-2`](#rfc3768-6.4.3-2) Master: forward packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.3) | restated | RFC9568-6.4.3-5 | RFC 9568 Section 6.4.3 keeps the rule that the Active Router forwards packets whose destination link-layer address is the Virtual Router MAC |
| [`RFC3768-6.4.3-3`](#rfc3768-6.4.3-3) Master: never accept packets addressed to the virtual router IP address(es) when not the IP address owner (§6.4.3) | restated | RFC9568-6.4.3-7 | RFC 9568 Section 6.4.3 keeps the prohibition and narrows it, because VRRPv3 adds Accept_Mode: the Active Router refuses such packets only when it is neither the address owner nor configured with Accept_Mode True |
| [`RFC3768-6.4.3-4`](#rfc3768-6.4.3-4) Master: accept packets addressed to the virtual router IP address(es) when the IP address owner (§6.4.3) | restated | RFC9568-6.4.3-6 | RFC 9568 Section 6.4.3 keeps the owner case and widens it to Accept_Mode True |
| [`RFC3768-6.4.3-5`](#rfc3768-6.4.3-5) Master: on Shutdown, cancel the Adver_Timer, send an ADVERTISEMENT with Priority = 0, and transition to Initialize (§6.4.3) | restated | RFC9568-6.4.3-8 | RFC 9568 Section 6.4.3 keeps the Shutdown sequence unchanged |
| [`RFC3768-6.4.3-6`](#rfc3768-6.4.3-6) Master: when the Adver_Timer fires, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | restated | RFC9568-6.4.3-9 | RFC 9568 Section 6.4.3 keeps the Adver_Timer expiry behaviour unchanged |
| [`RFC3768-6.4.3-7`](#rfc3768-6.4.3-7) Master: on an ADVERTISEMENT with Priority 0, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | restated | RFC9568-6.4.3-10 | RFC 9568 Section 6.4.3 keeps the response to a Priority 0 advertisement unchanged |
| [`RFC3768-6.4.3-8`](#rfc3768-6.4.3-8) Master: on an ADVERTISEMENT with higher Priority, or equal Priority and greater sender primary IP address, cancel the Adver_Timer, set the Master_Down_Timer to Master_Down_Interval, and transition to Backup (§6.4.3) | restated | RFC9568-6.4.3-11 | RFC 9568 Section 6.4.3 keeps the rule and states that the sender address comparison is an unsigned integer comparison in network byte order |
| [`RFC3768-6.4.3-9`](#rfc3768-6.4.3-9) Master: on a losing ADVERTISEMENT (lower Priority, or equal Priority with smaller sender address), discard it (§6.4.3) | restated | RFC9568-6.4.3-12 | RFC 9568 Section 6.4.3 keeps the discard and adds one step, that the Active Router immediately sends an advertisement so learning bridges relearn the segment. RFC 9568 Section 1.2 lists that addition as change 5 |
| [`RFC3768-7.1-1`](#rfc3768-7.1-1) Rx: verify the VRRP version is 2 (§7.1) | restated | RFC9568-7.1-1 | RFC 9568 Section 7.1 keeps the version check and changes the value it verifies from 2 to 3 |
| [`RFC3768-7.1-2`](#rfc3768-7.1-2) Rx: verify the received packet contains the complete VRRP packet, including fixed fields, IP address(es), and Authentication Data (§7.1) | restated | RFC9568-7.1-3 | RFC 9568 Section 7.1 keeps the completeness check over the fixed fields and the address list. The Authentication Data half of the RFC 3768 check is gone with the field |
| [`RFC3768-7.1-3`](#rfc3768-7.1-3) Rx: verify the VRRP checksum (§7.1) | restated | RFC9568-5.2.8-1 | RFC 9568 moves the checksum definition to Section 5.2.8 and extends it, because the IPv6 checksum covers a pseudo-header. RFC 9568 Section 1.2 lists that as change 4 |
| [`RFC3768-7.1-4`](#rfc3768-7.1-4) Rx: verify the VRID is configured on the receiving interface and the local router is not the IP address owner (Priority = 255) (§7.1) | restated | RFC9568-7.1-4 | RFC 9568 Section 7.1 keeps both halves of the check, that the VRID is configured on the receiving interface and that the local router is not the address owner |
| [`RFC3768-7.1-5`](#rfc3768-7.1-5) Rx: verify the Auth Type matches the locally configured authentication method and perform that method (§7.1) | dropped | not stated | VRRPv3 removes authentication. RFC 9568 Section 7.1 lists no Auth Type check, its packet format carries no Auth Type field, and its Section 9 states that VRRP for IPvX does not currently include any type of authentication |
| [`RFC3768-7.1-6`](#rfc3768-7.1-6) Rx: discard the packet if any mandatory receive check fails (§7.1) | restated | RFC9568-7.1-6 | RFC 9568 Section 7.1 keeps the discard on a failed mandatory check, and keeps the SHOULD log and MAY network-management indication beside it |
| [`RFC3768-7.1-7`](#rfc3768-7.1-7) Rx: if the optional address-list check fails and the sender is not the address owner (Priority != 255), drop the packet (§7.1) | dropped | not stated | RFC 9568 Section 7.1 removes the drop. The address-list check is a MAY at RFC9568-7.1-11, and on failure the receiver only SHOULD log the event and MAY indicate a misconfiguration. No sender-priority condition and no packet drop remain |
| [`RFC3768-7.1-8`](#rfc3768-7.1-8) Rx: verify the Adver Interval in the packet equals the locally configured value; discard the packet on mismatch (§7.1) | restated | RFC9568-7.1-8 | RFC 9568 Section 7.1 lowers the check from MUST to SHOULD and removes the drop, stating that the mismatch will not result in the VRRP packet being dropped. RFC 9568 Section 1.2 lists that as change 8. The field is also renamed and rescaled, from Adver Int in seconds to Max Advertise Interval in centiseconds |
| [`RFC3768-7.2-1`](#rfc3768-7.2-1) Tx: fill in the VRRP fields from the virtual router configuration state and compute the VRRP checksum (§7.2) | restated | RFC9568-7.2-1 | RFC 9568 Section 7.2 keeps the fill-and-checksum step unchanged |
| [`RFC3768-7.2-2`](#rfc3768-7.2-2) Tx: set the source MAC address to the virtual router MAC address (§7.2) | restated | RFC9568-7.2-2 | RFC 9568 Section 7.2 keeps the Virtual Router MAC as the source link-layer address |
| [`RFC3768-7.2-3`](#rfc3768-7.2-3) Tx: set the source IP address to the interface primary IP address (§7.2) | restated | RFC9568-7.2-3 | RFC 9568 Section 7.2 keeps the interface primary IPv4 address and adds the IPv6 case, where the source is the interface link-local address |
| [`RFC3768-7.2-4`](#rfc3768-7.2-4) Tx: set the IP protocol to VRRP (112) and send to the VRRP IP multicast group 224.0.0.18 (§7.2) | restated | RFC9568-7.2-4 | RFC 9568 Section 7.2 keeps IP protocol 112 and adds the IPv6 destination group ff02::12 beside 224.0.0.18 |
| [`RFC3768-8.2-1`](#rfc3768-8.2-1) Master: never respond to host ARP requests for virtual addresses with the physical MAC address (§8.2) | restated | RFC9568-8.1.2-1 | RFC 9568 Section 8.1.2 keeps the prohibition for IPv4 ARP and adds the IPv6 Neighbor Discovery counterpart at RFC9568-8.2.2-1 |
| [`RFC3768-8.3-1`](#rfc3768-8.3-1) Advertise the virtual router MAC address in Proxy ARP messages sent on behalf of VRRP-protected addresses (§8.3; lowercase "must" in the RFC) | restated | RFC9568-8.1.3-1 | RFC 9568 Section 8.1.3 keeps the Proxy ARP obligation and states it with an uppercase MUST |
| [`RFC3768-9.2-1`](#rfc3768-9.2-1) Token ring: implement the functional-address mode of operation when supporting VRRP on token ring (§9.2) | dropped | not stated | RFC 9568 removes token ring. Its Section 1.2 lists as change 6 that the appendices describing operation over legacy technologies, FDDI, Token Ring and ATM LAN Emulation, were removed, so RFC 9568 states no functional-address obligation |
| [`RFC3768-5.3.10-1`](#rfc3768-5.3.10-1) Set Authentication Data to zero on transmission and ignore it on reception (§5.3.10) | dropped | not stated | VRRPv3 removes the Authentication Data field, so no obligation about its value remains. RFC 9568 Section 5.2.6 states the same zero-on-transmission and ignore-on-reception rule for its Reserve field at RFC9568-5.2.6-1, which is a different field: it occupies the octet VRRPv2 used for Auth Type |
| [`RFC3768-7.1-9`](#rfc3768-7.1-9) Log the event when a mandatory receive check fails (§7.1) | restated | RFC9568-7.1-7 | RFC 9568 Section 7.1 keeps the log on a failed mandatory check and adds rate-limiting to it |
| [`RFC3768-7.1-10`](#rfc3768-7.1-10) Log the event on an address-list mismatch (§7.1) | restated | RFC9568-7.1-9 | RFC 9568 Section 7.1 keeps the log on an address-list mismatch, adds rate-limiting, and states it in one sentence covering the Max Advertise Interval mismatch as well |
| [`RFC3768-7.1-11`](#rfc3768-7.1-11) Log the event on an Adver Interval mismatch (§7.1) | restated | RFC9568-7.1-9 | RFC 9568 Section 7.1 merges this log with the address-list one into a single rate-limited recommendation, and renames the field to Max Advertise Interval |
| [`RFC3768-8.2-2`](#rfc3768-8.2-2) After restart/boot, never send ARP messages with the physical MAC address for owned virtual IP addresses (§8.2) | restated | RFC9568-8.1.2-3 | RFC 9568 Section 8.1.2 keeps the rule for IPv4 ARP and adds the IPv6 Neighbor Discovery counterpart at RFC9568-8.2.2-5 |
| [`RFC3768-8.2-3`](#rfc3768-8.2-3) Broadcast a gratuitous ARP with the virtual router MAC for each IP address when configuring an interface, and delay ARP at boot until both the IP address and virtual MAC are configured (§8.2) | restated | RFC9568-8.1.2-4 | RFC 9568 Section 8.1.2 splits the sentence in two and raises one half. The gratuitous ARP on interface configuration stays a SHOULD at RFC9568-8.1.2-4, and the boot delay until both the address and the Virtual Router MAC are configured becomes a MUST at RFC9568-8.1.2-2 |
| [`RFC3768-8.4-1`](#rfc3768-8.4-1) Never forward packets addressed to IP addresses adopted as Master when not the owner (§8.4) | restated | RFC9568-8.3.1-1 | RFC 9568 Section 8.3.1 keeps the rule and states it over both address families |
| [`RFC3768-9.1-1`](#rfc3768-9.1-1) FDDI: configure the virtual router MAC via a unicast MAC filter rather than changing the hardware MAC address (§9.1) | dropped | not stated | RFC 9568 removes FDDI. Its Section 1.2 lists as change 6 that the appendices describing operation over legacy technologies, FDDI, Token Ring and ATM LAN Emulation, were removed, so RFC 9568 states no unicast-MAC-filter recommendation |
| [`RFC3768-7.1-12`](#rfc3768-7.1-12) Indicate via network management that a receive error or misconfiguration occurred (§7.1) | restated | RFC9568-7.1-10 | RFC 9568 Section 7.1 keeps the network-management indication as a MAY |
| [`RFC3768-7.1-13`](#rfc3768-7.1-13) Verify that Count IP Addrs and the address list match the IP_Addresses configured for the VRID (§7.1) | restated | RFC9568-7.1-11 | RFC 9568 Section 7.1 keeps the address-list check as a MAY and renames the field to IPvX Addr Count |
| [`RFC3768-9.2-2`](#rfc3768-9.2-2) Token ring: support the unicast mode of operation in addition to functional addresses (§9.2) | dropped | not stated | RFC 9568 removes token ring. Its Section 1.2 lists as change 6 that the appendices describing operation over legacy technologies, FDDI, Token Ring and ATM LAN Emulation, were removed, so RFC 9568 states no unicast-mode permission |
