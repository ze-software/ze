# RFC 3768 - Virtual Router Redundancy Protocol (VRRP)

Experimental. Every requirement this repository extracted from RFC 3768, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.5% | 33 of 41 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.2% | 5 of 41 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 41 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 41 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 41 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 57.7% | 82 of 142 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 41 | of 52 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 41 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 7.3% | 3 of 41 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 41 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 41 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 41 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 52 |
| Gated MUST-level | 41 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 142 |
| Tagged units | 142 |
| Recorded audit verdicts | 38 |
| Discrimination records | 82 |
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
| Positive and negative tests | 33 | one part of the gated population |
| Annotated (including scoped evidence) | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **41** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (33):** [`RFC3768-5.2.3-2`](#rfc3768-5.2.3-2), [`RFC3768-5.3.2-1`](#rfc3768-5.3.2-1), [`RFC3768-5.3.4-1`](#rfc3768-5.3.4-1), [`RFC3768-5.3.4-2`](#rfc3768-5.3.4-2), [`RFC3768-5.3.6-1`](#rfc3768-5.3.6-1), [`RFC3768-6.4.2-1`](#rfc3768-6.4.2-1), [`RFC3768-6.4.2-2`](#rfc3768-6.4.2-2), [`RFC3768-6.4.2-3`](#rfc3768-6.4.2-3), [`RFC3768-6.4.2-4`](#rfc3768-6.4.2-4), [`RFC3768-6.4.2-5`](#rfc3768-6.4.2-5), [`RFC3768-6.4.2-6`](#rfc3768-6.4.2-6), [`RFC3768-6.4.2-7`](#rfc3768-6.4.2-7), [`RFC3768-6.4.2-8`](#rfc3768-6.4.2-8), [`RFC3768-6.4.3-1`](#rfc3768-6.4.3-1), [`RFC3768-6.4.3-2`](#rfc3768-6.4.3-2), [`RFC3768-6.4.3-3`](#rfc3768-6.4.3-3), [`RFC3768-6.4.3-4`](#rfc3768-6.4.3-4), [`RFC3768-6.4.3-5`](#rfc3768-6.4.3-5), [`RFC3768-6.4.3-6`](#rfc3768-6.4.3-6), [`RFC3768-6.4.3-7`](#rfc3768-6.4.3-7), [`RFC3768-6.4.3-8`](#rfc3768-6.4.3-8), [`RFC3768-6.4.3-9`](#rfc3768-6.4.3-9), [`RFC3768-7.1-1`](#rfc3768-7.1-1), [`RFC3768-7.1-2`](#rfc3768-7.1-2), [`RFC3768-7.1-3`](#rfc3768-7.1-3), [`RFC3768-7.1-4`](#rfc3768-7.1-4), [`RFC3768-7.1-5`](#rfc3768-7.1-5), [`RFC3768-7.1-6`](#rfc3768-7.1-6), [`RFC3768-7.1-7`](#rfc3768-7.1-7), [`RFC3768-7.1-8`](#rfc3768-7.1-8), [`RFC3768-8.1-1`](#rfc3768-8.1-1), [`RFC3768-8.2-1`](#rfc3768-8.2-1), [`RFC3768-8.2-4`](#rfc3768-8.2-4)

**Annotated (including scoped evidence) (8):** [`RFC3768-5.2.2-1`](#rfc3768-5.2.2-1), [`RFC3768-5.2.3-1`](#rfc3768-5.2.3-1), [`RFC3768-7.2-1`](#rfc3768-7.2-1), [`RFC3768-7.2-2`](#rfc3768-7.2-2), [`RFC3768-7.2-3`](#rfc3768-7.2-3), [`RFC3768-7.2-4`](#rfc3768-7.2-4), [`RFC3768-8.3-1`](#rfc3768-8.3-1), [`RFC3768-9.2-1`](#rfc3768-9.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3768-5.2.2-1` | Routers MUST NOT forward a datagram with this destination address regardless of its TTL. (§5.2.2) | MUST NOT | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the VRRP plugin performs no IP datagram forwarding -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes each received advert into the FSM and never re-emits it, and tx scopes adverts to link-local multicast with IP_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:143 |
| `RFC3768-5.2.3-1` | The TTL MUST be set to 255. (§5.2.3) | MUST | 5.2.3 | **positive:** `unit/verify` [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L274). **negative:** no negative test. **{single-polarity}:** buildIPv4Header unconditionally sets TTL 255 at internal/plugins/vrrp/transport/transport.go:562, so no input yields a different TTL -- the rx TTL!=255 discard is the separate RFC3768-5.2.3-2 |
| `RFC3768-5.2.3-2` | A VRRP router receiving a packet with the TTL not equal to 255 MUST discard the packet. (§5.2.3, §7.1) | MUST | 5.2.3 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L48). **positive:** `unit/verify` [`TestDecodeV2DiscardsTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L38). **negative:** `unit/verify` [`TestDecodeV2DiscardsTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L39). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L533) |
| `RFC3768-5.3.2-1` | A packet with unknown type MUST be discarded. (§5.3.2) | MUST | 5.3.2 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L49). **positive:** `unit/verify` [`TestDecodeV2DiscardsUnknownType`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L59). **negative:** `unit/verify` [`TestDecodeV2DiscardsUnknownType`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L60). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L150) |
| `RFC3768-5.3.4-1` | The priority value for the VRRP router that owns the IP address(es) associated with the virtual router MUST be 255 (decimal). (§5.3.4) | MUST | 5.3.4 | **positive:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L781). **positive:** `unit/verify` [`TestV2OwnerRunsAtPriority255`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L47). **negative:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L782). **negative:** `unit/verify` [`TestV2OwnerRunsAtPriority255`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L48) |
| `RFC3768-5.3.4-2` | VRRP routers backing up a virtual router MUST use priority values between 1-254 (decimal). (§5.3.4) | MUST | 5.3.4 | **positive:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L316). **positive:** `unit/verify` [`TestV2BoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L89). **negative:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L317). **negative:** `unit/verify` [`TestV2BoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L90) |
| `RFC3768-5.3.6-1` | A packet with unknown authentication type or that does not match the locally configured authentication method MUST be discarded. (§5.3.6, §7.1) | MUST | 5.3.6 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L50). **positive:** `unit/verify` [`TestDecodeV2DiscardsUnknownOrUnconfiguredAuthType`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L84). **negative:** `unit/verify` [`TestDecodeV2DiscardsUnknownOrUnconfiguredAuthType`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L85). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L598) |
| `RFC3768-6.4.2-1` | - MUST NOT respond to ARP requests for the IP address(s) associated with the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L395). **positive:** `unit/verify` [`TestV2BackupHoldsNoVirtualAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L137). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L432). **negative:** `unit/verify` [`TestV2BackupHoldsNoVirtualAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L138) |
| `RFC3768-6.4.2-2` | - MUST discard packets with a destination link layer MAC address equal to the virtual router MAC address. (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L111). **positive:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L125). **negative:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L112). **negative:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L126) |
| `RFC3768-6.4.2-3` | - MUST NOT accept packets addressed to the IP address(es) associated with the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L396). **positive:** `unit/verify` [`TestV2BackupHoldsNoVirtualAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L139). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L433). **negative:** `unit/verify` [`TestV2BackupHoldsNoVirtualAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L140) |
| `RFC3768-6.4.2-4` | If a Shutdown event is received, then: o Cancel the Master_Down_Timer o Transition to the {Initialize} state (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L88). **positive:** `unit/verify` [`TestV2BackupShutdown`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L54). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L89). **negative:** `unit/verify` [`TestV2BackupShutdown`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L55) |
| `RFC3768-6.4.2-5` | If the Master_Down_Timer fires, then: o Send an ADVERTISEMENT o Broadcast a gratuitous ARP request containing the virtual router MAC address for each IP address associated with the virtual router o Set the Adver_Timer to Advertisement_Interval o Transition to the {Master} state (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestAnnounceMasterFramesPerAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_announce_verdict_test.go#L51). **positive:** `unit/verify` [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L446). **positive:** `unit/verify` [`TestPromotionSetsAdverTimerToOwnInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L86). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L636). **negative:** `unit/verify` [`TestPromotionSetsAdverTimerToOwnInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L87) |
| `RFC3768-6.4.2-6` | If the Priority in the ADVERTISEMENT is Zero, then: o Set the Master_Down_Timer to Skew_Time (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L90). **positive:** `unit/verify` [`TestV2BackupPriorityZeroSetsSkewTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L130). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L91). **negative:** `unit/verify` [`TestV2BackupPriorityZeroSetsSkewTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L131) |
| `RFC3768-6.4.2-7` | If Preempt_Mode is False, or If the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: o Reset the Master_Down_Timer to Master_Down_Interval (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L92). **positive:** `unit/verify` [`TestV2BackupResetsOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L155). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L93). **negative:** `unit/verify` [`TestV2BackupResetsOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L156) |
| `RFC3768-6.4.2-8` | If Preempt_Mode is False, or If the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: o Reset the Master_Down_Timer to Master_Down_Interval else: o Discard the ADVERTISEMENT (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L94). **positive:** `unit/verify` [`TestV2BackupResetsOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L157). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L95). **negative:** `unit/verify` [`TestV2BackupResetsOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L158) |
| `RFC3768-6.4.3-1` | - MUST respond to ARP requests for the IP address(es) associated with the virtual router. (§6.4.3, §8.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L69). **positive:** `unit/verify` [`TestPromotionInstallsIPv4AddressOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/promote_install_test.go#L30). **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L75). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L136). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L76) |
| `RFC3768-6.4.3-2` | - MUST forward packets with a destination link layer MAC address equal to the virtual router MAC address. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L117). **positive:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L131). **negative:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L118). **negative:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L132) |
| `RFC3768-6.4.3-3` | - MUST NOT accept packets addressed to the IP address(es) associated with the virtual router if it is not the IP address owner. (§6.4.3) | MUST NOT | 6.4.3 | **positive:** `unit/verify` [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L377). **negative:** `unit/verify` [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L378) |
| `RFC3768-6.4.3-4` | - MUST accept packets addressed to the IP address(es) associated with the virtual router if it is the IP address owner. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L379). **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L431). **negative:** `unit/verify` [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L380). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L397) |
| `RFC3768-6.4.3-5` | If a Shutdown event is received, then: o Cancel the Adver_Timer o Send an ADVERTISEMENT with Priority = 0 o Transition to the {Initialize} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L96). **positive:** `unit/verify` [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L504). **positive:** `unit/verify` [`TestV2MasterShutdown`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L243). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L97). **negative:** `unit/verify` [`TestV2MasterShutdown`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L244) |
| `RFC3768-6.4.3-6` | If the Adver_Timer fires, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L98). **positive:** `unit/verify` [`TestV2MasterAdverTimerFires`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L198). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L637). **negative:** `unit/verify` [`TestV2MasterAdverTimerFires`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L199) |
| `RFC3768-6.4.3-7` | If the Priority in the ADVERTISEMENT is Zero, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L99). **positive:** `unit/verify` [`TestV2MasterPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L219). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L100). **negative:** `unit/verify` [`TestV2MasterPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L220) |
| `RFC3768-6.4.3-8` | If the Priority in the ADVERTISEMENT is greater than the local Priority, or If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IP Address of the sender is greater than the local primary IP Address, then: o Cancel Adver_Timer o Set Master_Down_Timer to Master_Down_Interval o Transition to the {Backup} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L101). **positive:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L74). **positive:** `unit/verify` [`TestInstanceV2TieBreakDemotionRestartsTheTimers`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L176). **positive:** `unit/verify` [`TestV2MasterYieldsToAHigherPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L183). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L102). **negative:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L75). **negative:** `unit/verify` [`TestV2MasterYieldsToAHigherPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L184) |
| `RFC3768-6.4.3-9` | If the Priority in the ADVERTISEMENT is greater than the local Priority, or If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IP Address of the sender is greater than the local primary IP Address, then: o Cancel Adver_Timer o Set Master_Down_Timer to Master_Down_Interval o Transition to the {Backup} state else: o Discard ADVERTISEMENT (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L103). **positive:** `unit/verify` [`TestV2MasterDemotesOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L246). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L104). **negative:** `unit/verify` [`TestV2MasterDemotesOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L247) |
| `RFC3768-7.1-1` | MUST verify the VRRP version is 2. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L42). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L609) |
| `RFC3768-7.1-2` | MUST verify that the received packet contains the complete VRRP packet (including fixed fields, IP Address(es), and Authentication Data). (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L43). **positive:** `unit/verify` [`TestDecodeV2DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L110). **negative:** `unit/verify` [`TestDecodeV2DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L111). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L218) |
| `RFC3768-7.1-3` | MUST verify the VRRP checksum. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L44). **negative:** `unit/verify` [`TestDecodeV2ChecksumCorrupt`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L306) |
| `RFC3768-7.1-4` | MUST verify that the VRID is configured on the receiving interface and the local router is not the IP Address owner (Priority equals 255 (decimal)). (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L45). **positive:** `unit/verify` [`TestInstanceV2OwnerDiscardsAdvert`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1190). **positive:** `unit/verify` [`TestRxAdvertScopedToReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_rx_iface_verdict_linux_test.go#L130). **positive:** `unit/verify` [`TestRxVRIDCheckedOnTheReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_rx_interface_test.go#L25). **negative:** `unit/verify` [`TestInstanceV2OwnerDiscardsAdvert`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1189). **negative:** `unit/verify` [`TestRxAdvertScopedToReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_rx_iface_verdict_linux_test.go#L131). **negative:** `unit/verify` [`TestRxVRIDCheckedOnTheReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_rx_interface_test.go#L26). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L163) |
| `RFC3768-7.1-5` | MUST verify that the Auth Type matches the locally configured authentication method for the virtual router and perform that authentication method. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L46). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L599) |
| `RFC3768-7.1-6` | If any one of the above checks fails, the receiver MUST discard the packet (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L106). **positive:** `unit/verify` [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L553). **negative:** `unit/verify` [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L528). **negative:** `unit/verify` [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L105) |
| `RFC3768-7.1-7` | If the packet was not generated by the address owner (Priority does not equal 255 (decimal)), the receiver MUST drop the packet, otherwise continue processing. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceV2AddressListMatchReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L637). **positive:** `unit/verify` [`TestInstanceV2AddressListMismatchFromOwnerContinues`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1237). **negative:** `unit/verify` [`TestInstanceV2AddressListMismatchDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L598) |
| `RFC3768-7.1-8` | - MUST verify that the Adver Interval in the packet is the same as the locally configured for this virtual router (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L47). **negative:** `unit/verify` [`TestDecodeV2IntervalMismatchDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L293). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L251) |
| `RFC3768-7.2-1` | Fill in the VRRP packet fields with the appropriate virtual router configuration state - Compute the VRRP checksum (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestEncodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L87). **negative:** no negative test. **{single-polarity}:** a golden encode pins every field and the checksum -- WriteTo internal/plugins/vrrp/packet/packet.go:251 plus FillChecksum internal/plugins/vrrp/packet/checksum.go:86 -- while a corrupted-encoding rejection is the separate receive requirement RFC3768-7.1-3 |
| `RFC3768-7.2-2` | Set the source MAC address to Virtual Router MAC Address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L388). **positive:** `unit/verify` [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L39). **negative:** no negative test. **{single-polarity}:** the source MAC is the virtual-router MAC from packet.VirtualMAC internal/plugins/vrrp/packet/packet.go:97 egressed by binding the tx socket to the vMAC macvlan internal/plugins/vrrp/transport/backend_linux.go:133, a deterministic derivation with no input that yields a different MAC |
| `RFC3768-7.2-3` | Set the source IP address to interface primary IP address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L216). **positive:** `unit/verify` [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L42). **negative:** no negative test. **{single-polarity}:** the source IP is the parent unit primary IPv4 from resolveParentPrimaryV4 internal/plugins/vrrp/transport/transport.go:573, a deterministic selection re-resolved on address change, with no input that yields a wrong-source advert |
| `RFC3768-7.2-4` | Set the IP protocol to VRRP - Send the VRRP packet to the VRRP IP multicast group (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L275). **positive:** `unit/verify` [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L45). **negative:** no negative test. **{single-polarity}:** buildIPv4Header sets IP protocol 112 at internal/plugins/vrrp/transport/transport.go:563 and SendAdvert targets 224.0.0.18 at internal/plugins/vrrp/transport/backend_linux.go:256, both constants with no input that changes them |
| `RFC3768-8.1-1` | If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.1; lowercase "must" in the RFC) | MUST | 8.1 | **positive:** `unit/verify` [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L20). **negative:** `unit/verify` [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L21) |
| `RFC3768-8.2-1` | The Master virtual router MUST NOT respond with its physical MAC address. (§8.2) | MUST NOT | 8.2 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L70). **positive:** `unit/verify` [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L170). **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L63). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L42). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L137). **negative:** `unit/verify` [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L171). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L64). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L43) |
| `RFC3768-8.2-4` | When a host sends an ARP request for one of the virtual router IP addresses, the Master virtual router MUST respond to the ARP request with the virtual MAC address for the virtual router. (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L71). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L56). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L72). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L57) |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestDecodeV2DiscardsTTLNot255 feeds the VRRPv2 golden at TTL 255 (accepted) and the same bytes at TTL 0/1/64/254 (each ErrTTL); only meta.TTL differs, so the buffer isolates the rule. The discard after a Decode error is the shared return in instance.onPacket, proven for VRRPv2 by TestInstanceRxFailedCheckIsDiscarded (RFC3768-7.1-6).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV2DiscardsTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L39) | unit/verify | revert, verified |
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L533) | unit/verify | unproven |
| positive | [`TestDecodeV2DiscardsTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L48) | unit/verify | unproven |

### [`RFC3768-5.3.2-1`](#rfc3768-5.3.2-1)

A packet with unknown type MUST be discarded. (§5.3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestDecodeV2DiscardsUnknownType: VRRPv2 Type 1 accepted; Type 0, 2, 15 with a recomputed checksum each ErrType, so the checksum cannot mask the type check. Discard plumbing as RFC3768-5.2.3-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV2DiscardsUnknownType`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L60) | unit/verify | revert, verified |
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L150) | unit/verify | unproven |
| positive | [`TestDecodeV2DiscardsUnknownType`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L49) | unit/verify | unproven |

### [`RFC3768-5.3.4-1`](#rfc3768-5.3.4-1)

The priority value for the VRRP router that owns the IP address(es) associated with the virtual router MUST be 255 (decimal). (§5.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestV2OwnerRunsAtPriority255 extracts a group with version 2 and fatals unless Version reached the spec as v2; the owner configured at 120 must have EffectivePriority 255 under decrements 0, 1, 119 and 4064 and must advertise Priority 255 on startup; a v2 non-owner keeps 120 (negative). Records: revert EffectivePriority, both polarities, observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L782) | unit/verify | revert, verified |
| negative | [`TestV2OwnerRunsAtPriority255`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L781) | unit/verify | revert, verified |
| positive | [`TestV2OwnerRunsAtPriority255`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L47) | unit/verify | revert, verified |

### [`RFC3768-5.3.4-2`](#rfc3768-5.3.4-2)

VRRP routers backing up a virtual router MUST use priority values between 1-254 (decimal). (§5.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestV2BoundaryPriority validates version-2 groups (version asserted at extraction): 1 and 254 accepted, 0 and 255 refused with the range-check text 'is out of range; configure 1..254' (so no other check can satisfy the case), and a v2 non-owner at 100 floors at 1 under decrements 99, 100, 4064. Records: revert validateGroup, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L317) | unit/verify | unproven |
| negative | [`TestV2BoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L316) | unit/verify | unproven |
| positive | [`TestV2BoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L89) | unit/verify | revert, verified |

### [`RFC3768-5.3.6-1`](#rfc3768-5.3.6-1)

A packet with unknown authentication type or that does not match the locally configured authentication method MUST be discarded. (§5.3.6, §7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): Both clauses now fed: Auth Type 1 and 2 (defined, not the local method 0) and 3, 128, 255 (unknown) each ErrAuthType with a recomputed checksum; Auth Type 0 accepted. Discard plumbing as RFC3768-5.2.3-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV2DiscardsUnknownOrUnconfiguredAuthType`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L85) | unit/verify | revert, verified |
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L598) | unit/verify | unproven |
| positive | [`TestDecodeV2DiscardsUnknownOrUnconfiguredAuthType`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L50) | unit/verify | unproven |

### [`RFC3768-6.4.2-1`](#rfc3768-6.4.2-1)

- MUST NOT respond to ARP requests for the IP address(s) associated with the virtual router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): Re-read: the unit's body is unchanged; only the 6.4.2 and 6.4.3 Virtual Router MAC proxy tags left it for TestVRRPBackupDoesNotForwardVirtualMACFrames, so the verdict stands. Prior note: vrrp judge 2026-09-29 (independent): Forbidden: a v2 Backup answering ARP for the virtual address. TestV2BackupHoldsNoVirtualAddress starts a Version 2 non-owner, requires Backup, zero address installs and no acceptance filter; the v2 owner contrast is Master with exactly one install on the virtual-MAC device. Asserted at Ze's boundary (the address Ze installs for the kernel ARP responder), the lower-layer form ai/rules/rfc-compliance.md asks for; no ARP is observed on the wire. Records: revert doInstallVIPs, both polarities. The Backup's virtual-MAC macvlan is up (D-8) but holds no address, so it has nothing to answer for.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L432) | unit/verify | unproven |
| negative | [`TestV2BackupHoldsNoVirtualAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L138) | unit/verify | revert, verified |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L395) | unit/verify | unproven |
| positive | [`TestV2BackupHoldsNoVirtualAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L137) | unit/verify | revert, verified |

### [`RFC3768-6.4.2-2`](#rfc3768-6.4.2-2)

- MUST discard packets with a destination link layer MAC address equal to the virtual router MAC address. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuation 3): Wiring now proven on the host: TestBackupDiscardFollowsTheState drives the FSM and asserts the worker sets the discard on the virtual router MAC device at start (idle worker: on then off), entering Backup adds no call, promotion withdraws it once after the only install, and a higher-priority advert demotes and sets it again before the only removal (records observed red on doRemoveVIPs; the run revert was also observed red but the ledger keeps one record per unit/polarity). Wire effect: TestVRRPBackupDoesNotForwardVirtualMACFrames (QEMU guest) forwards a transit datagram sent to the Virtual Router MAC without the nft drop and stops it with it (records on backupFilterTables). The host unit's negative tag is a state contrast; the violating negative (no discard, frame forwarded) is the QEMU control.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L112) | unit/verify | revert, verified |
| negative | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L126) | unit/verify | revert, verified |
| positive | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L111) | unit/verify | revert, verified |
| positive | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L125) | unit/verify | revert, verified |

### [`RFC3768-6.4.2-3`](#rfc3768-6.4.2-3)

- MUST NOT accept packets addressed to the IP address(es) associated with the virtual router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): Re-read: the unit's body is unchanged; only the 6.4.2 and 6.4.3 Virtual Router MAC proxy tags left it for TestVRRPBackupDoesNotForwardVirtualMACFrames, so the verdict stands. Prior note: vrrp judge 2026-09-29 (independent): Forbidden: a v2 Backup accepting packets addressed to the virtual address. Same unit as 6.4.2-1: a Version 2 Backup installs no address (so the kernel has no local route for it) and hands no acceptance filter; the v2 Master contrast installs it. Records: revert doInstallVIPs, both polarities. Forwarding of virtual-MAC frames by a Backup is the separate RFC3768-6.4.2-2 defect (D-8), not this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L433) | unit/verify | unproven |
| negative | [`TestV2BackupHoldsNoVirtualAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L140) | unit/verify | revert, verified |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L396) | unit/verify | unproven |
| positive | [`TestV2BackupHoldsNoVirtualAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L139) | unit/verify | revert, verified |

### [`RFC3768-6.4.2-4`](#rfc3768-6.4.2-4)

If a Shutdown event is received, then: o Cancel the Master_Down_Timer o Transition to the {Initialize} state (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestV2BackupShutdown runs a VRRPv2 Backup: Shutdown yields exactly [StopTimers, Backup->Initialize] and state Initialize; the canceled timer's later expiry does nothing. The exact action list pins both the cancel and the transition.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L89) | unit/verify | unproven |
| negative | [`TestV2BackupShutdown`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L55) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L88) | unit/verify | unproven |
| positive | [`TestV2BackupShutdown`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L54) | unit/verify | revert, verified |

### [`RFC3768-6.4.2-5`](#rfc3768-6.4.2-5)

If the Master_Down_Timer fires, then: o Send an ADVERTISEMENT o Broadcast a gratuitous ARP request containing the virtual router MAC address for each IP address associated with the virtual router o Set the Adver_Timer to Advertisement_Interval o Transition to the {Master} state (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestPromotionSetsAdverTimerToOwnInterval: a VRRPv2 Backup's live expiry yields exactly SendAdvert{100,2000}, InstallVIPs, AnnounceFailover, StartAdvertTimer{2s}, Backup->Master; an unarmed generation promotes nothing. TestAnnounceMasterFramesPerAddress checks each announced frame against literals: broadcast ARP request per VIP, sender and target hardware address 00-00-5e-00-01-0a, target protocol address the VIP, exactly the VIP set.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L636) | unit/verify | unproven |
| negative | [`TestPromotionSetsAdverTimerToOwnInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L446) | unit/verify | unproven |
| positive | [`TestPromotionSetsAdverTimerToOwnInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L86) | unit/verify | revert, verified |
| positive | [`TestAnnounceMasterFramesPerAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_announce_verdict_test.go#L51) | unit/verify | revert, verified |

### [`RFC3768-6.4.2-6`](#rfc3768-6.4.2-6)

If the Priority in the ADVERTISEMENT is Zero, then: o Set the Master_Down_Timer to Skew_Time (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestV2BackupPriorityZeroSetsSkewTime: Priority 0 arms the literal 0.609375 s ((256-100)/256, not the v3 1.21875 s); non-zero Priority 1 arms the literal 6.609375 s Master_Down_Interval. Expectations no longer come from skewTime.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L91) | unit/verify | unproven |
| negative | [`TestV2BackupPriorityZeroSetsSkewTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L131) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L90) | unit/verify | unproven |
| positive | [`TestV2BackupPriorityZeroSetsSkewTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L130) | unit/verify | revert, verified |

### [`RFC3768-6.4.2-7`](#rfc3768-6.4.2-7)

If Preempt_Mode is False, or If the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: o Reset the Master_Down_Timer to Master_Down_Interval (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestV2BackupResetsOrDiscards on VRRPv2: equal, greater and Preempt-off lower each arm the literal local 6.609375 s and stay Backup; Preempt-on lower produces nothing and the earlier timer still promotes. No interval adoption is asserted for v2 (the old wrong v3 row is no longer the evidence).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L93) | unit/verify | unproven |
| negative | [`TestV2BackupResetsOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L156) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L92) | unit/verify | unproven |
| positive | [`TestV2BackupResetsOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L155) | unit/verify | revert, verified |

### [`RFC3768-6.4.2-8`](#rfc3768-6.4.2-8)

If Preempt_Mode is False, or If the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: o Reset the Master_Down_Timer to Master_Down_Interval else: o Discard the ADVERTISEMENT (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): Same unit as RFC3768-6.4.2-7: the greater-priority case is now fed, so an '==' comparison fails; the discard branch is proven by no action plus the old timer still promoting.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L95) | unit/verify | unproven |
| negative | [`TestV2BackupResetsOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L158) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L94) | unit/verify | unproven |
| positive | [`TestV2BackupResetsOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L157) | unit/verify | revert, verified |

### [`RFC3768-6.4.3-1`](#rfc3768-6.4.3-1)

- MUST respond to ARP requests for the IP address(es) associated with the virtual router. (§6.4.3, §8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp closure judge 2026-09-30 (independent) re-judged: the tagged units changed only by added RFC requirement tag comment lines for the new rows (8.1.2-6 / 8.2.2-8 / 8.2-4 / 8.1.1-2); test bodies byte-identical, records current, verdict unchanged. Prior note: vrrp judge 2026-09-29 (independent, continuation 3): Product install now proven: TestPromotionInstallsIPv4AddressOnVirtualMACDevice (v2 subtest) asserts a Backup installs nothing and the promoted non-owner makes exactly one install, on the virtual-MAC macvlan zv4-2-10 (never the parent), of exactly 192.0.2.1/24 (record observed red on doInstallVIPs). TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly (QEMU guest) proves that installed state answers ARP and that no reply is seen with the address removed (records on vipCIDRs). The dataplane_linux_test sysctl tags stay without records.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L136) | unit/verify | unproven |
| negative | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L69) | unit/verify | unproven |
| positive | [`TestPromotionInstallsIPv4AddressOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/promote_install_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L75) | unit/verify | revert, verified |

### [`RFC3768-6.4.3-2`](#rfc3768-6.4.3-2)

- MUST forward packets with a destination link layer MAC address equal to the virtual router MAC address. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuation 3): Wiring now proven on the host: TestBackupDiscardFollowsTheState asserts promotion withdraws the discard exactly once and only after the address install, and that it holds before promotion and after demotion (records observed red on doInstallVIPs and doRemoveVIPs). Wire effect: TestVRRPBackupDoesNotForwardVirtualMACFrames (QEMU guest) forwards the datagram sent to the Virtual Router MAC once the filter is withdrawn and the address installed, and not while the filter holds (records on clearBackupFilter).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L118) | unit/verify | revert, verified |
| negative | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L131) | unit/verify | revert, verified |

### [`RFC3768-6.4.3-3`](#rfc3768-6.4.3-3)

- MUST NOT accept packets addressed to the IP address(es) associated with the virtual router if it is not the IP address owner. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 re-read after RFC3768-6.4.3-4 tags were added to TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress (body unchanged): Forbidden: a v2 non-owner Master accepting packets addressed to the virtual address. The Version 2 test fatals unless exactly one accept-filter call is made and errors unless accept=false for the non-owner and accept=true for the owner.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L378) | unit/verify | unproven |
| positive | [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L377) | unit/verify | unproven |

### [`RFC3768-6.4.3-4`](#rfc3768-6.4.3-4)

- MUST accept packets addressed to the IP address(es) associated with the virtual router if it is the IP address owner. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): Re-read: the unit's body is unchanged; only the 6.4.2 and 6.4.3 Virtual Router MAC proxy tags left it for TestVRRPBackupDoesNotForwardVirtualMACFrames, so the verdict stands. Prior note: vrrp judge 2026-09-29 (independent): TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress promotes a Version 2 group to Master and requires exactly one acceptance-filter call with accept=true for the owner (positive) and accept=false for the non-owner (negative), so an owner suppression or an acceptance granted to every Master goes red. Records: revert EffectiveAcceptMode, both polarities. The older tags on TestInstanceOwnerStartupGoesMaster/NonOwnerGoesBackup remain and add nothing.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L380) | unit/verify | revert, verified |
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L397) | unit/verify | unproven |
| positive | [`TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L379) | unit/verify | revert, verified |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L431) | unit/verify | unproven |

### [`RFC3768-6.4.3-5`](#rfc3768-6.4.3-5)

If a Shutdown event is received, then: o Cancel the Adver_Timer o Send an ADVERTISEMENT with Priority = 0 o Transition to the {Initialize} state (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestV2MasterShutdown: a Version 2 owner Master receives Shutdown; the last advert carries Priority 0, state is Initialize, in.advert is nil, and after 10 s of fake clock no timer event and no further advert appear (Cancel Adver_Timer, Send Priority 0, Transition to Initialize, each asserted). Contrast: a v2 Backup shutting down sends nothing. Records: revert cancelTimers, both polarities. The FSM matrix row master/shutdown still runs v3.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L97) | unit/verify | unproven |
| negative | [`TestV2MasterShutdown`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L244) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L96) | unit/verify | unproven |
| positive | [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L504) | unit/verify | unproven |
| positive | [`TestV2MasterShutdown`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L243) | unit/verify | revert, verified |

### [`RFC3768-6.4.3-6`](#rfc3768-6.4.3-6)

If the Adver_Timer fires, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestV2MasterAdverTimerFires on VRRPv2: the live expiry yields exactly SendAdvert{100,2000}, StartAdvertTimer{2s}; an unarmed generation yields nothing.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L637) | unit/verify | unproven |
| negative | [`TestV2MasterAdverTimerFires`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L98) | unit/verify | unproven |
| positive | [`TestV2MasterAdverTimerFires`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L198) | unit/verify | revert, verified |

### [`RFC3768-6.4.3-7`](#rfc3768-6.4.3-7)

If the Priority in the ADVERTISEMENT is Zero, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestV2MasterPriorityZero on VRRPv2: Priority 0 yields exactly SendAdvert{100,2000}, StartAdvertTimer{2s} and stays Master; a non-zero lower-priority advert yields nothing.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L100) | unit/verify | unproven |
| negative | [`TestV2MasterPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L220) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L99) | unit/verify | unproven |
| positive | [`TestV2MasterPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L219) | unit/verify | revert, verified |

### [`RFC3768-6.4.3-8`](#rfc3768-6.4.3-8)

If the Priority in the ADVERTISEMENT is greater than the local Priority, or If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IP Address of the sender is greater than the local primary IP Address, then: o Cancel Adver_Timer o Set Master_Down_Timer to Master_Down_Interval o Transition to the {Backup} state (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): the previously missing v2 'Priority greater' clause is now TestV2MasterYieldsToAHigherPriority: a Version 2 Master (source 192.0.2.2) gets Priority 250 from the LOWER sender 192.0.2.1, so only priority decides; it must be Backup, in.advert nil, silent for two intervals, Backup at MDI-1ms and Master at MDI+1ms with MDI = 3 s + 56/256 s (RFC 3768 6.1); a Priority 150 advert from the greater 192.0.2.8 holds Master with the timer armed (negative). Records: revert demoteToBackup, both polarities. The equal-priority tie-break clause stays proven by TestInstanceTieBreakComparesTheAdvertisementSource and TestInstanceV2TieBreakDemotionRestartsTheTimers (records syncSourceLocked, senderWinsTieBreak, demoteToBackup).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L102) | unit/verify | unproven |
| negative | [`TestV2MasterYieldsToAHigherPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L184) | unit/verify | revert, verified |
| negative | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L101) | unit/verify | unproven |
| positive | [`TestV2MasterYieldsToAHigherPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_v2_group_test.go#L183) | unit/verify | revert, verified |
| positive | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestInstanceV2TieBreakDemotionRestartsTheTimers`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L176) | unit/verify | revert, verified |

### [`RFC3768-6.4.3-9`](#rfc3768-6.4.3-9)

If the Priority in the ADVERTISEMENT is greater than the local Priority, or If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IP Address of the sender is greater than the local primary IP Address, then: o Cancel Adver_Timer o Set Master_Down_Timer to Master_Down_Interval o Transition to the {Backup} state else: o Discard ADVERTISEMENT (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestV2MasterDemotesOrDiscards on VRRPv2: higher priority and equal priority from 192.0.2.200 (greater than .10 only unsigned) each yield exactly StopTimers, RemoveVIPs, StartMasterDownTimer{6.609375s literal}, Master->Backup; equal from a smaller sender and lower priority yield nothing and stay Master.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L104) | unit/verify | unproven |
| negative | [`TestV2MasterDemotesOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L247) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L103) | unit/verify | unproven |
| positive | [`TestV2MasterDemotesOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L246) | unit/verify | revert, verified |

### [`RFC3768-7.1-1`](#rfc3768-7.1-1)

MUST verify the VRRP version is 2. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a version other than 2 at a v2 group. Case N9 fatals unless a v3 packet at a v2 group and a v2 packet at a v3 group each return ErrVersion; the golden v2 packet decodes at a v2 group.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L609) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L42) | unit/verify | unproven |

### [`RFC3768-7.1-2`](#rfc3768-7.1-2)

MUST verify that the received packet contains the complete VRRP packet (including fixed fields, IP Address(es), and Authentication Data). (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): All three parts fed for VRRPv2: cut inside the 8-octet fixed fields (4, 7) ErrTruncated; Authentication Data cut under an honest Count 2 (16, 20, 23 octets, checksum recomputed) ErrLength; Count 3 over two addresses ErrLength; the complete 24-octet golden is accepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV2DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L111) | unit/verify | revert, verified |
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L218) | unit/verify | unproven |
| positive | [`TestDecodeV2DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L110) | unit/verify | revert, verified |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): Re-judged with the added guest unit. VRID half: TestRxVRIDCheckedOnTheReceivingInterface (engine dispatchRx, instance lookup) and TestRxAdvertScopedToReceivingInterface (a VRID 10 advert sent on the VRID-20 interface never reaches the VRID 10 instance and is refused there with ErrUnknownVRID; sent on the VRID 10 interface it arrives and decodes). Owner half: TestInstanceV2OwnerDiscardsAdvert. Every unit has an observed-red record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceV2OwnerDiscardsAdvert`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1189) | unit/verify | revert, verified |
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L163) | unit/verify | unproven |
| negative | [`TestRxVRIDCheckedOnTheReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_rx_interface_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestRxAdvertScopedToReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_rx_iface_verdict_linux_test.go#L131) | unit/verify | revert, verified |
| positive | [`TestInstanceV2OwnerDiscardsAdvert`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1190) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L45) | unit/verify | unproven |
| positive | [`TestRxVRIDCheckedOnTheReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc3768_rx_interface_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRxAdvertScopedToReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_rx_iface_verdict_linux_test.go#L130) | unit/verify | revert, verified |

### [`RFC3768-7.1-5`](#rfc3768-7.1-5)

MUST verify that the Auth Type matches the locally configured authentication method for the virtual router and perform that authentication method. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting an Auth Type other than the one configured method (0, No Authentication). Case N8 sets Auth Type 1 and fatals unless ErrAuthType; the golden type 0 decodes. With method 0 there is no authentication to perform.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L599) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L46) | unit/verify | unproven |

### [`RFC3768-7.1-6`](#rfc3768-7.1-6)

If any one of the above checks fails, the receiver MUST discard the packet (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): TestInstanceRxFailedCheckIsDiscarded: every RFC 3768 7.1 check for a Version 2 group (TTL, version, fixed fields, auth data, checksum, VRID, Auth Type, owner) gets its own buffer (checksum refilled so only that check fails), its exact reason, no FSM event and unchanged state; the unbroken packet reaches AdvertReceived. The unit changed only by a v3 case; records on onPacket re-recorded by the judge after that change, observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L528) | unit/verify | revert, verified |
| negative | [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L553) | unit/verify | unproven |
| positive | [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L106) | unit/verify | revert, verified |

### [`RFC3768-7.1-7`](#rfc3768-7.1-7)

If the packet was not generated by the address owner (Priority does not equal 255 (decimal)), the receiver MUST drop the packet, otherwise continue processing. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. onPacket drops a v2 address-list mismatch only when the sender Priority is not 255, and otherwise logs it (address-list reason) and continues. TestInstanceV2AddressListMismatchDrops pins the drop for Priority 250 with no FSM event; TestInstanceV2AddressListMismatchFromOwnerContinues pins the Priority 255 mismatch reaching the FSM with the reason logged; TestInstanceV2AddressListMatchReachesFSM pins a matching list. Discrimination recorded for each. DF-VRRP-5 re-read 2026-09-27, strictly: clause 'not generated by the address owner (Priority does not equal 255) MUST drop': TestInstanceV2AddressListMismatchDrops feeds a Priority 250 mismatch and asserts exactly ReasonAddressList and no FSM event, so a continue goes red. Clause 'otherwise continue processing': TestInstanceV2AddressListMismatchFromOwnerContinues feeds a Priority 255 mismatch and asserts the reason logged and an AdvertReceived at priority 255, so a drop goes red. A matching list reaching the FSM is TestInstanceV2AddressListMatchReachesFSM. onPacket's later edits (v3 noteOwnerConflict, syncSourceLocked) sit off this path. Priority 254, the boundary below the owner, is not fed to any unit. RA-VRRP strict re-read 2026-09-27 (independent): agrees enforced. Forbidden 1, continuing a non-owner mismatch: TestInstanceV2AddressListMismatchDrops (Priority 250) asserts rxErrors == [ReasonAddressList] and no event, red on continue. Forbidden 2, dropping an owner mismatch: TestInstanceV2AddressListMismatchFromOwnerContinues (Priority 255) asserts the reason and an AdvertReceived at priority 255, red on drop. The code tests adv.Priority != ownerPriority (instance.go onPacket), so the 250 and 255 cases pin the predicate's two outcomes; 254 remains unfed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceV2AddressListMismatchDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L598) | unit/verify | revert, verified |
| positive | [`TestInstanceV2AddressListMatchReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L637) | unit/verify | revert, verified |
| positive | [`TestInstanceV2AddressListMismatchFromOwnerContinues`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1237) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp final judge 2026-09-29 (independent): re-read after the comment-only edit to TestTxAdvertV4OnWire (body unchanged): the guest test captures a transmitted VRRPv2 frame on the peer veth and compares the source MAC with the literal 00-00-5e-00-01-0a. Single-polarity positive per the row annotation (constant derivation).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L388) | unit/verify | unproven |
| positive | [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L39) | unit/verify | revert, verified |

### [`RFC3768-7.2-3`](#rfc3768-7.2-3)

Set the source IP address to interface primary IP address (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp final judge 2026-09-29 (independent): re-read after the comment-only edit to TestTxAdvertV4OnWire (body unchanged): it adds a lower secondary 192.0.2.5/24 after the primary 192.0.2.251/24 and requires the captured VRRPv2 source to be the primary, so a lowest-address or any-address selection fails. Single-polarity positive per the row annotation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L216) | unit/verify | unproven |

### [`RFC3768-7.2-4`](#rfc3768-7.2-4)

Set the IP protocol to VRRP - Send the VRRP packet to the VRRP IP multicast group (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp final judge 2026-09-29 (independent): re-read after the comment-only edit to TestTxAdvertV4OnWire (body unchanged): it compares the captured VRRPv2 IP protocol and destination with the literals 112 and 224.0.0.18. Single-polarity positive per the row annotation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestSendAdvertIPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L275) | unit/verify | unproven |

### [`RFC3768-8.1-1`](#rfc3768-8.1-1)

If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.1; lowercase "must" in the RFC)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-30 (independent) re-judged: the unit changed only by added tag comment lines for RFC9568-8.1.1-1/-2; body byte-identical. Discrimination records for this row's tags written this pass (none existed). TestVRRPRedirectSourceFollowsVirtualMAC (Linux guest integration, two non-owner virtual-MAC macvlans under the product sysctl recipe, icmp_errors_use_inbound_ifaddr reset to 0 before applyDataplaneSysctls) asserts the IPv4 redirect for a packet sent to a virtual MAC is sourced from that virtual router's VIP despite a reverse route via the real interface; the negative sends to the physical MAC and sees 192.0.2.254, so the capture separates the two. Both polarities recorded observed red on applyDataplaneSysctls.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L20) | unit/verify | revert, verified |

### [`RFC3768-8.2-1`](#rfc3768-8.2-1)

The Master virtual router MUST NOT respond with its physical MAC address. (§8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp closure judge 2026-09-30 (independent) re-judged: the tagged units changed only by added RFC requirement tag comment lines for the new rows (8.1.2-6 / 8.2.2-8 / 8.2-4 / 8.1.1-2); test bodies byte-identical, records current, verdict unchanged. Prior note: vrrp judge 2026-09-29 (independent, continuation 3): Re-read: the owner units changed only by added tag comments; bodies unchanged. Verdict unchanged. Owner: TestOwnerFilterWiredOnPromotionForEveryFamily (ipv4-v2) drives an owner to setOwnerFilter with the parent and exactly the owned address before install, non-owner names none (records on doInstallVIPs); TestVRRPOwnerAnswersWithVirtualMACOnly proves the wire effect with a physical-MAC control. Non-owner: TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly (QEMU guest) captures the physical MAC without applyDataplaneSysctls and only the virtual MAC with it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L137) | unit/verify | unproven |
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L43) | unit/verify | revert, verified |
| negative | [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L171) | unit/verify | revert, verified |
| negative | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L70) | unit/verify | unproven |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L170) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L63) | unit/verify | revert, verified |

### [`RFC3768-8.2-4`](#rfc3768-8.2-4)

When a host sends an ARP request for one of the virtual router IP addresses, the Master virtual router MUST respond to the ARP request with the virtual MAC address for the virtual router. (§8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp closure judge 2026-09-30 (independent): TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly (Linux integration) captures on the wire that with the product recipe (applyDataplaneSysctls) a LAN host's ARP request for the non-owner VIP is answered with the virtual MAC only; the control (recipe absent) sees the physical MAC answer and, with the address removed, no answer, so the capture discriminates. TestVRRPOwnerAnswersWithVirtualMACOnly: with the owner filter (ownerARPReplyTerm) the owner's ARP replies carry the virtual MAC; without it (ownerFilterTables) the physical MAC is observed. Each tagged unit has an observed-red record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L57) | unit/verify | revert, verified |
| negative | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L71) | unit/verify | revert, verified |

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
| Mapped sentences | 26 |
| Declined as scope | 8 |
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
| [`RFC3768-8.2-4`](#rfc3768-8.2-4) When a host sends an ARP request for one of the virtual router IP addresses, the Master virtual router MUST respond to the ARP request with the virtual MAC address for the virtual router. (§8.2) | restated | RFC9568-8.1.2-6 | RFC 9568 Section 8.1.2 keeps the obligation, names the Master the Active Router, and states that the ARP response indicates the Virtual Router MAC address |
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
