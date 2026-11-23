# RFC 4724 - Graceful Restart Mechanism for BGP

Graceful Restart. Every requirement listed for RFC 4724, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 24 | of 26 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 26 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 26 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 26 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 26 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 73 | 73 of 100 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 25 | of 26 mandatory requirements reviewed | 1 weak, wrong or unimplemented, 3 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 26 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 26 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 26 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 26 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 26 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 26 | of 31 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 26 mandatory requirements: they add up to 26. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Graceful Restart |
| Automatic checks | Included |
| Requirements | 31 |
| Mandatory requirements (MUST level) | 26 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 100 |
| Tests linked to requirements | 100 |
| Recorded audit verdicts | 25 |
| Fault test records | 73 |
| Summary | `rfc/short/rfc4724.md` |
| Requirement table | `rfc/requirements/rfc4724.md` |
| RFC text | `rfc/full/rfc4724.txt` |

## Automatic checks

Included in automatic checks: Graceful Restart Mechanism for BGP (RFC 4724): all 26 MUST-level requirements are implemented and proven across internal/component/bgp (GR capability advertise/negotiate, Restart-State/Forwarding-State bits, End-of-RIB send/detect, Receiving Speaker stale retention/expiry/purge, GR Established collision override without NOTIFICATION, and Restarting Speaker per-family route selection deferral + configurable Selection_Deferral_Timer) and internal/plugins/fib/kernel (Restarting Speaker forwarding-state preservation, stale marking, non-differentiation during forwarding, and post-selection update and stale removal).

## Published support status

**Area:** Graceful Restart

**Test verification:** 26 of 26 MUST proven

**Evidence:** interop 1, unit 25

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 24 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 2 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **26** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (24):** [`RFC4724-3-2`](#rfc4724-3-2), [`RFC4724-3-3`](#rfc4724-3-3), [`RFC4724-3-4`](#rfc4724-3-4), [`RFC4724-3-5`](#rfc4724-3-5), [`RFC4724-4-1`](#rfc4724-4-1), [`RFC4724-4-2`](#rfc4724-4-2), [`RFC4724-4.1-1`](#rfc4724-4.1-1), [`RFC4724-4.1-2`](#rfc4724-4.1-2), [`RFC4724-4.1-4`](#rfc4724-4.1-4), [`RFC4724-4.1-5`](#rfc4724-4.1-5), [`RFC4724-4.1-6`](#rfc4724-4.1-6), [`RFC4724-4.1-7`](#rfc4724-4.1-7), [`RFC4724-4.1-8`](#rfc4724-4.1-8), [`RFC4724-4.2-1`](#rfc4724-4.2-1), [`RFC4724-4.2-2`](#rfc4724-4.2-2), [`RFC4724-4.2-3`](#rfc4724-4.2-3), [`RFC4724-4.2-4`](#rfc4724-4.2-4), [`RFC4724-4.2-5`](#rfc4724-4.2-5), [`RFC4724-4.2-6`](#rfc4724-4.2-6), [`RFC4724-4.2-7`](#rfc4724-4.2-7), [`RFC4724-4.2-8`](#rfc4724-4.2-8), [`RFC4724-4.2-9`](#rfc4724-4.2-9), [`RFC4724-4.2-10`](#rfc4724-4.2-10), [`RFC4724-4.2-11`](#rfc4724-4.2-11)

**Annotated (including scoped evidence) (2):** [`RFC4724-3-1`](#rfc4724-3-1), [`RFC4724-4.1-3`](#rfc4724-4.1-3)

## Figures

- **MUST bar**: Ze verified 26
- **Overview bar**: Ze MUST verified 26, SHOULD verified 1, Not assessed 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 26 |
| MUST rows kept (the MUST bar) | 26 |
| Ze verified | 26 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD verified | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4724.json](../../../data/rfc/rfc4724.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4724-3-1` | A BGP speaker MUST NOT include more than one instance of the Graceful Restart Capability in the capability advertisement (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestExtractGRCapabilities_CapabilityDecl`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L118). **negative:** no negative test. **{single-polarity}:** ze's GR sender emits exactly one code-64 declaration per peer (internal/component/bgp/plugins/gr/gr.go:703 extractGRCapabilities appends one CapabilityDecl per peer) and the encoder writes a single TLV (internal/core/bgp/capability/capability.go:553 WriteTo); there is no code path that emits two instances, so the more-than-one case cannot be constructed to test negatively |
| `RFC4724-3-2` | The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L478). **positive:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedRestartFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L53). **negative:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L489). **negative:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedRestartFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L54) |
| `RFC4724-3-3` | The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L484). **positive:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L73). **negative:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L498). **negative:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L74) |
| `RFC4724-3-4` | If more than one instance of the Graceful Restart Capability is carried in the capability advertisement, the receiver of the advertisement MUST ignore all but the last instance of the Graceful Restart Capability. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L62). **negative:** `unit/verify` [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L70) |
| `RFC4724-3-5` | When set (value 1), this bit indicates that the BGP speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker from the speaker before advertising routing information to the speaker. (Section 3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC4724PeerRestartStateBitDoesNotDelayInitialRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rbit_no_wait_test.go#L34). **positive:** `unit/verify` [`TestRFC4724RestartStateBitPeerIsNotWaitedForUnderUpdateDelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rbit_update_delay_test.go#L121). **negative:** `unit/verify` [`TestRFC4724PeerRestartStateBitDoesNotDelayInitialRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rbit_no_wait_test.go#L38). **negative:** `unit/verify` [`TestRFC4724RestartStateBitClearPeerIsWaitedForUnderUpdateDelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rbit_update_delay_test.go#L158) |
| `RFC4724-4-1` | The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L16). **positive:** `unit/verify` [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L111). **positive:** `unit/verify` [`TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L764). **positive:** `unit/verify` [`TestInitialSyncMarkerWaitsForNoProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L797). **positive:** `unit/verify` [`TestInitialSyncShutsTheQueueGateAndFreesTheRailsWithTheMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L356). **positive:** `unit/verify` [`TestPredecessorSyncWritesNothingOnSuccessor`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rapid_reconnect_test.go#L95). **positive:** `unit/verify` [`TestRoutePushingBindingsCountBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L437). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L224). **negative:** `unit/verify` [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L23). **positive:** `interop/nightly` [`checkNoFamilyEndOfRIB`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1280) |
| `RFC4724-4-2` | It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestGRStateManagerNotificationBypass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L220). **positive:** `unit/verify` [`TestPeerCloseNamesTheNotificationTheNeighborSent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_close_test.go#L46). **positive:** `unit/verify` [`TestRFC4724NotificationCloseRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_close_reason_test.go#L25). **negative:** `unit/verify` [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L46). **negative:** `unit/verify` [`TestPeerCloseNamesTheTransportLoss`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_close_test.go#L74). **negative:** `unit/verify` [`TestRFC4724TransportCloseRetains`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_close_reason_test.go#L47) |
| `RFC4724-4.1-1` | When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L87). **negative:** `unit/verify` [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L88) |
| `RFC4724-4.1-2` | When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB and MUST mark them as stale. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L89). **negative:** `unit/verify` [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L90) |
| `RFC4724-4.1-3` | It MUST NOT differentiate between stale and other information during forwarding. (Section 4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L91). **negative:** no negative test. **{single-polarity}:** fib-kernel's startupSweep (internal/plugins/fib/kernel/fibkernel.go) retains every surviving kernel route in place with zero backend modifications, replacements, or metric downgrades, so stale routes forward identically to fresh routes with no code path that differentiates stale forwarding entries |
| `RFC4724-4.1-4` | To re-establish the session with its peer, the Restarting Speaker MUST set the "Restart State" bit in the Graceful Restart Capability of the OPEN message. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4724OpenInsideTheRestartWindowSetsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L65). **positive:** `unit/verify` [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L262). **negative:** `unit/verify` [`TestRFC4724OpenOutsideTheRestartWindowClearsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L87). **negative:** `unit/verify` [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L500) |
| `RFC4724-4.1-5` | However, it MUST defer route selection for an address family until it either (a) receives the End-of-RIB marker from all its peers (excluding the ones with the "Restart State" bit set in the received capability and excluding the ones that do not advertise the graceful restart capability) or (b) the Selection_Deferral_Timer referred to below has expired. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4724RIBSelectionDeferralPerFamilyAndTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_selection_deferral_test.go#L89). **positive:** `unit/verify` [`TestRFC4724SelectionDeferralTimerAndExclusions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restarting_test.go#L33). **negative:** `unit/verify` [`TestRFC4724RIBSelectionDeferralPerFamilyAndTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_selection_deferral_test.go#L90). **negative:** `unit/verify` [`TestRFC4724SelectionDeferralTimerAndExclusions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restarting_test.go#L34) |
| `RFC4724-4.1-6` | After the BGP speaker performs route selection, the forwarding state of the speaker MUST be updated and any previously marked stale information MUST be removed. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L92). **negative:** `unit/verify` [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L93) |
| `RFC4724-4.1-7` | Once the initial update is complete for an address family (including the case that there is no routing update to send), the End-of-RIB marker MUST be sent. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L20). **positive:** `unit/verify` [`TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_initial_update_eor_test.go#L42). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L228). **negative:** `unit/verify` [`TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_initial_update_eor_test.go#L44) |
| `RFC4724-4.1-8` | To put an upper bound on the amount of time a router defers its route selection, an implementation MUST support a (configurable) timer that imposes this upper bound. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4724RIBSelectionDeferralPerFamilyAndTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_selection_deferral_test.go#L91). **positive:** `unit/verify` [`TestRFC4724SelectionDeferralTimerAndExclusions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restarting_test.go#L31). **negative:** `unit/verify` [`TestRFC4724RIBSelectionDeferralPerFamilyAndTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_selection_deferral_test.go#L92). **negative:** `unit/verify` [`TestRFC4724SelectionDeferralTimerAndExclusions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restarting_test.go#L32) |
| `RFC4724-4.2-1` | In case it does not detect the termination of the old TCP session and still considers the BGP session as being established, it MUST treat the subsequent open connection from the peer as an indication of the termination of the old TCP session and act accordingly (when the Graceful Restart Capability has been received from the peer). (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_collision_test.go#L96). **negative:** `unit/verify` [`TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_collision_test.go#L97) |
| `RFC4724-4.2-2` | "Acting accordingly" in this context means that the previous TCP session MUST be closed, and the new one retained. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_collision_test.go#L98). **negative:** `unit/verify` [`TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_collision_test.go#L99) |
| `RFC4724-4.2-3` | When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L42). **positive:** `unit/verify` [`TestRFC4724RetentionCoversEveryAdvertisedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L167). **positive:** `unit/verify` [`TestRFC4724SessionDownRetainsAndMarksRoutesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L69). **negative:** `unit/verify` [`TestGRStateManagerNoGRCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L297). **negative:** `unit/verify` [`TestRFC4724SessionDownWithoutCapabilityRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L97) |
| `RFC4724-4.2-4` | To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L272). **positive:** `unit/verify` [`TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L40). **negative:** `unit/verify` [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L286). **negative:** `unit/verify` [`TestRFC4724ConsecutiveRestartDeletesBeforeMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L51) |
| `RFC4724-4.2-5` | The router MUST NOT differentiate between stale and other routing information during forwarding. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L852). **negative:** `unit/verify` [`TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_stale_forwarding_test.go#L27) |
| `RFC4724-4.2-6` | In re-establishing the session, the "Restart State" bit in the Graceful Restart Capability of the OPEN message sent by the Receiving Speaker MUST NOT be set unless the Receiving Speaker has restarted. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestRFC4724OpenOutsideTheRestartWindowClearsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L86). **positive:** `unit/verify` [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L497). **negative:** `unit/verify` [`TestRFC4724OpenInsideTheRestartWindowSetsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L66). **negative:** `unit/verify` [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L266) |
| `RFC4724-4.2-7` | If the session does not get re-established within the "Restart Time" that the peer advertised previously, the Receiving Speaker MUST delete all the stale routes from the peer that it is retaining. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L67). **positive:** `unit/verify` [`TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L139). **positive:** `unit/verify` [`TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L115). **negative:** `unit/verify` [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L105). **negative:** `unit/verify` [`TestGRStateManagerRestartExpiryAfterReconnect`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L428). **negative:** `unit/verify` [`TestGRStateManagerRestartExpiryRejectsOldCycle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L379). **negative:** `unit/verify` [`TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L158) |
| `RFC4724-4.2-8` | Once the session is re-established, if the "Forwarding State" bit for a specific address family is not set in the newly received Graceful Restart Capability, or if a specific address family is not included in the newly received Graceful Restart Capability, or if the Graceful Restart Capability is not received in the re-established session at all, then the Receiving Speaker MUST immediately remove all the stale routes from the peer that it is retaining for that address family. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerReconnectFBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L143). **positive:** `unit/verify` [`TestRFC4724ReestablishedWithoutCapabilityPurgesOnTheStructuredPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_capability_absent_test.go#L32). **positive:** `unit/verify` [`TestRFC4724ReestablishedWithoutForwardingStatePurgesTheFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L186). **negative:** `unit/verify` [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L102). **negative:** `unit/verify` [`TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L233) |
| `RFC4724-4.2-9` | The Receiving Speaker MUST send the End-of-RIB marker once it completes the initial update for an address family (including the case that it has no routes to send) to the peer. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L22). **positive:** `unit/verify` [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L116). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L229). **negative:** `unit/verify` [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L24) |
| `RFC4724-4.2-10` | The Receiving Speaker MUST replace the stale routes by the routing updates received from the peer. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFamilyRIB_InsertClearsStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L147). **negative:** `unit/verify` [`TestFamilyRIB_InsertNewDuringStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L179) |
| `RFC4724-4.2-11` | Once the End-of-RIB marker for an address family is received from the peer, it MUST immediately remove any routes from the peer that are still marked as stale for that address family. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerEORPurge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L185). **positive:** `unit/verify` [`TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L90). **positive:** `unit/verify` [`TestRFC9494ConsecutiveRestartEORDeletesEveryConnectionsStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go#L200). **negative:** `unit/verify` [`TestGRStateManagerEORForNonGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L314). **negative:** `unit/verify` [`TestRFC4724EndOfRIBLeavesOtherFamiliesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L112) |
| `RFC4724-2-1` | Although the End-of-RIB marker is specified for the purpose of BGP graceful restart, it is noted that the generation of such a marker upon completion of the initial update would be useful for routing convergence in general, and thus the practice is recommended. (Section 2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4-3` | In addition, even if the speaker does not have the ability to preserve its forwarding state for any address family during BGP restart, it is still recommended that the speaker advertise the Graceful Restart Capability to its peer (Section 4) | RECOMMENDED | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4-4` | A BGP speaker MAY advertise the Graceful Restart Capability for an address family to its peer if it has the ability to preserve its forwarding state for the address family when BGP restarts. (Section 4) | MAY | 4 | **positive:** `unit/verify` [`TestRFC4724GRCapabilityKeepsEveryFamilyTheRIBStores`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_rib_families_test.go#L93). **positive:** `unit/verify` [`TestRFC4724GRCapabilityListsTheFamiliesOfTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_gr_capability_test.go#L101). **negative:** `unit/verify` [`TestRFC4724GRCapabilityClaimsNoFamilyItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_gr_capability_test.go#L128). **negative:** `unit/verify` [`TestRFC4724GRCapabilityOmitsAFamilyNoRIBStores`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_rib_families_test.go#L60) |
| `RFC4724-4.2-12` | In the event that it determines that its peer's forwarding state is not viable prior to the re-establishment of the session, the speaker MAY delete all the stale routes from the peer that it is retaining. (Section 4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4.2-13` | To put an upper bound on the amount of time a router retains the stale routes, an implementation MAY support a (configurable) timer that imposes this upper bound. (Section 4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 4724 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4724-3-1`](#rfc4724-3-1)

A BGP speaker MUST NOT include more than one instance of the Graceful Restart Capability in the capability advertisement (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestExtractGRCapabilities_CapabilityDecl`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L118) | unit/verify | revert, verified |

### [`RFC4724-3-2`](#rfc4724-3-2)

The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sender half: TestGracefulRestartEncodeReservedBits (WriteTo masks to R bit and 12-bit time). Receiver half: TestRFC4724ReceiverIgnoresReservedRestartFlags parses the nibble with reserved 0x3 set, R clear and set, and requires the result equal to the clean parse. Judge breaks: R bit read with mask 0x9000, and Restart Time read with mask 0x3FFF, each turned the test red. Only 0x3 used because RFC 8538 assigned the N bit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L489) | unit/verify | not verified |
| negative | [`TestRFC4724ReceiverIgnoresReservedRestartFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L478) | unit/verify | not verified |
| positive | [`TestRFC4724ReceiverIgnoresReservedRestartFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L53) | unit/verify | revert, verified |

### [`RFC4724-3-3`](#rfc4724-3-3)

The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sender half: TestGracefulRestartEncodeReservedBits (only F bit 0x80 per family). Receiver half: TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags sets all seven reserved bits (0x7F) with F clear and set and requires the parse equal to the clean one. Judge break: ForwardingState read as flags != 0 turned it red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L498) | unit/verify | not verified |
| negative | [`TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L484) | unit/verify | not verified |
| positive | [`TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L73) | unit/verify | revert, verified |

### [`RFC4724-3-4`](#rfc4724-3-4)

If more than one instance of the Graceful Restart Capability is carried in the capability advertisement, the receiver of the advertisement MUST ignore all but the last instance of the Graceful Restart Capability. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. two GR instances with distinct restart time and family; Negotiate must keep the second; fails on first-wins or merge. The negative tag is the complement of the positive's assertion on the same input (two hats), not an independent case

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L62) | unit/verify | revert, verified |

### [`RFC4724-3-5`](#rfc4724-3-5)

When set (value 1), this bit indicates that the BGP speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker from the speaker before advertising routing information to the speaker. (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724PeerRestartStateBitDoesNotDelayInitialRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rbit_no_wait_test.go#L38) | unit/verify | revert, verified |
| negative | [`TestRFC4724RestartStateBitClearPeerIsWaitedForUnderUpdateDelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rbit_update_delay_test.go#L158) | unit/verify | revert, verified |
| positive | [`TestRFC4724PeerRestartStateBitDoesNotDelayInitialRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rbit_no_wait_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC4724RestartStateBitPeerIsNotWaitedForUnderUpdateDelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rbit_update_delay_test.go#L121) | unit/verify | revert, verified |

### [`RFC4724-4-1`](#rfc4724-4-1)

The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-14 by an independent auditor who authored neither 4aa1f84ff6 nor its test change. RFC 4724 Section 4 read: "The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established." Stale unit TestInitialSyncMarkerWaitsForNoProcess re-read as committed: it names two route-pushing processes (sendUpdateOnly, sendRawOnly) plus an event-only listener, seeds the expectation set with the two pushers, and now raises the replay fence over both (raiseTestReplayFence sets replayFence and initialUpdateOwed), then runs the production sync body (runInitialSyncNow, Peer.sendInitialRoutes) with neither pusher reporting. It requires the sync to return within 2s, then asserts the fence is still up, zero reports credited (len apiSyncSignalled), and exactly the IPv4 unicast End-of-RIB bytes on the wire (eorWire equality). The deleted assertion (apiSyncReady channel still open) is replaced by fence-still-up plus no-credit over the same two silent processes: a sync that waited again for those reports, or for the fence to lower, never returns and goes red on the Eventually, and a marker missing or duplicated goes red on the byte equality. peer_initial_sync.go never reads initialUpdateOwed, so the extra assertions isolate that the marker left with the fence still raised. Question 1: the test asserts the RFC's own trigger, completion of ze's own initial update, not a process report. Question 2: yes, for a reintroduced wait or a missing marker. Question 3: the input is isolated, one negotiated family, no routes. Question 4: this unit proves the timing clause; the no-update case and every-family clause rest on the sibling positives below, unchanged. The tag's claim paragraph is unchanged and states no more than the body checks. The other cited units are byte-identical (TestInitialSyncEORReachesTheSilentFamilyToo only shifted). Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-11-14 by an independent auditor who authored neither 08fe903803 nor the uncommitted phase 2 of spec-single-owner-lifecycle-2-bgp-session. Units read: TestInitialSyncEORReachesTheSilentFamilyToo, TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily, TestInitialSyncMarkerWaitsForNoProcess, TestInitialSyncShutsTheQueueGateAndFreesTheRailsWithTheMarker, TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes (sync call only), and TestPredecessorSyncWritesNothingOnSuccessor (positive: the successor session carries exactly one UPDATE with the queued route, then the exact IPv4 unicast End-of-RIB, then no UPDATE for syncJoinProbe). checkNoFamilyEndOfRIB requires FRR's own bgp_update_receive_eor decode on the original session after the route arrived. Positives: sendInitialRoutes puts one End-of-RIB per negotiated family on the wire, including a silent family and a no-MP session. Negative TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes: while a plugin producing the initial update has not acknowledged peer-up, sendInitialRoutes is observed waiting and nothing is on the wire (EORSent 0); after the acknowledgement exactly the IPv4 unicast marker is written. Red on a marker sent ahead of the initial update or not sent after it. TestIsEndOfRIBAnyFamily is a neighbouring detector rule.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L224) | unit/verify | not verified |
| negative | [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L16) | unit/verify | not verified |
| positive | [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L111) | unit/verify | not verified |
| positive | [`TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L764) | unit/verify | not verified |
| positive | [`TestInitialSyncMarkerWaitsForNoProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L797) | unit/verify | revert, verified |
| positive | [`TestInitialSyncShutsTheQueueGateAndFreesTheRailsWithTheMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L356) | unit/verify | not verified |
| positive | [`TestRoutePushingBindingsCountBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L437) | unit/verify | not verified |
| positive | [`TestPredecessorSyncWritesNothingOnSuccessor`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rapid_reconnect_test.go#L95) | unit/verify | mutant, verified |
| positive | [`checkNoFamilyEndOfRIB`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1280) | interop/nightly | not verified |

### [`RFC4724-4-2`](#rfc4724-4-2)

It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit after 26d70dfa0d against RFC 4724 Section 4. handleStructuredState still takes onSessionDownPlan's plan, which short-circuits a NOTIFICATION close (rpc.ReasonForbidsRetention) to activated=false; the only change on that branch is dropEarlierRetained, a map deletion, and a prior cycle's purge-stale now reaches every retained connection through dispatchRetained, which for a single restart is the previous connection alone. No retain-routes or mark-stale is sent for a NOTIFICATION close. Tagged roots unchanged: TestRFC4724NotificationCloseRetainsNothing requires zero dispatched commands; TestRFC4724TransportCloseRetains requires exact source-scoped retain-routes and mark-stale for tcp-failure and local-close; the peer_close pair distinguishes Cease from transport loss on a running Peer; the state-manager pair contrasts activation. Sent-NOTIFICATION handling shares sessionCloseFor, covered by the untagged TestSessionCloseForCoversEveryCause, as before.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L46) | unit/verify | not verified |
| negative | [`TestRFC4724TransportCloseRetains`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_close_reason_test.go#L47) | unit/verify | revert, verified |
| negative | [`TestPeerCloseNamesTheTransportLoss`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_close_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestGRStateManagerNotificationBypass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L220) | unit/verify | not verified |
| positive | [`TestRFC4724NotificationCloseRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_close_reason_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestPeerCloseNamesTheNotificationTheNeighborSent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_close_test.go#L46) | unit/verify | revert, verified |

### [`RFC4724-4.1-1`](#rfc4724-4.1-1)

When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 4724 Section 4.1. TestRFC4724RestartingSpeakerForwardingState proves the full shutdown-and-restart retention lifecycle: parseFIBConfig defaults FlushOnStop to false; fib.run(ctx, false, ready) retains installed 198.51.100.0/24 across shutdown with 0 backend deletes; fib.run(ctx, true, ready) flushes 198.51.100.0/24 on shutdown (negative); and startupSweep() retains pre-restart surviving kernel routes without deleting them.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L87) | unit/verify | revert, verified |

### [`RFC4724-4.1-2`](#rfc4724-4.1-2)

When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB and MUST mark them as stale. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 4724 Section 4.1. TestRFC4724RestartingSpeakerForwardingState proves both shutdown retention and restart stale-marking: startupSweep() retains both surviving kernel routes with 0 deletes and marks both in fib.retained and fib.staleTables (positive); processEvent clears staleTables on refreshed 198.51.100.0/24 and leaves newly installed post-restart 198.18.0.0/24 unmarked (negative); and post-selection sweep deletes only the unrefreshed stale prefix.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L89) | unit/verify | revert, verified |

### [`RFC4724-4.1-3`](#rfc4724-4.1-3)

It MUST NOT differentiate between stale and other information during forwarding. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 4724 Section 4.1 and {single-polarity: positive}. startupSweep() only records stale ownership in memory and performs zero backend adds, replaces, or deletes, leaving surviving kernel forwarding entries identical to fresh routes with no differentiating code path. TestRFC4724RestartingSpeakerForwardingState asserts backend.deleted==0, backend.replaced==0, and backend.added==0 after startupSweep().

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L91) | unit/verify | revert, verified |

### [`RFC4724-4.1-4`](#rfc4724-4.1-4)

To re-establish the session with its peer, the Restarting Speaker MUST set the "Restart State" bit in the Graceful Restart Capability of the OPEN message. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged independently against RFC 4724 Section 4.1: "To re-establish the session with its peer, the Restarting Speaker MUST set the "Restart State" bit in the Graceful Restart Capability of the OPEN message." TestRFC4724OpenInsideTheRestartWindowSetsRestartState supplies a future restart deadline and a plugin code-64 value with R clear, builds and parses OPEN, and asserts R set, Restart Time 120 and the unchanged IPv4/unicast tuple. TestRFC4724OpenOutsideTheRestartWindowClearsRestartState supplies cold-start and expired deadlines and asserts R clear with Time 120, so unconditional setting and unconditional omission both fail. The test-only wire view delegates to Peer.pluginCapabilitySnapshot; that production function captures declarations once and invokes restartFlagsFor before buildOpenFromSnapshot packs its captured capabilities. Supplementary TestSetRBitOnCapability and TestSetRBitTimeGatePattern pin the SetRBit edit and copy preservation; their copied time condition is not the production-window proof. The fixture adaptation and helper-comment correction preserve these inputs, tags and assertions. Discrimination freshness is checked separately by the main thread; this rejudgment makes no new execution claim.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L500) | unit/verify | revert, verified |
| negative | [`TestRFC4724OpenOutsideTheRestartWindowClearsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L262) | unit/verify | revert, verified |
| positive | [`TestRFC4724OpenInsideTheRestartWindowSetsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L65) | unit/verify | revert, verified |

### [`RFC4724-4.1-5`](#rfc4724-4.1-5)

However, it MUST defer route selection for an address family until it either (a) receives the End-of-RIB marker from all its peers (excluding the ones with the "Restart State" bit set in the received capability and excluding the ones that do not advertise the graceful restart capability) or (b) the Selection_Deferral_Timer referred to below has expired. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 4724 Section 4.1. All three previous defects are fixed: (1) parseSelectionDeferralConfig now uses configjson.ForEachPeer + configjson.PeerRemoteIP, skipping dynamic group templates and DynamicPeerIP, and TestRFC4724RIBSelectionDeferralPerFamilyAndTimer exercises real Ze BGP config JSON (named standalone peers, grouped peer, dynamic group template, string leaves); (2) knownFamilies ensures route-less converged families (proven via IPv4Multicast) are marked released while other families remain deferred; (3) both TestRFC4724RIBSelectionDeferralPerFamilyAndTimer and TestRFC4724SelectionDeferralTimerAndExclusions use 2 non-excluded GR peers plus 1 R=1 peer and 1 non-GR peer, proving partial EoR (1 of 2), duplicate EoR, and unnegotiated-family EoR keep deferral active while 2 of 2 releases per-family without waiting for R=1 or non-GR peers, and remaining deferred families release at Selection_Deferral_Timer expiry.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724RIBSelectionDeferralPerFamilyAndTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_selection_deferral_test.go#L90) | unit/verify | revert, verified |
| negative | [`TestRFC4724SelectionDeferralTimerAndExclusions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restarting_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC4724RIBSelectionDeferralPerFamilyAndTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_selection_deferral_test.go#L89) | unit/verify | revert, verified |
| positive | [`TestRFC4724SelectionDeferralTimerAndExclusions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restarting_test.go#L33) | unit/verify | revert, verified |

### [`RFC4724-4.1-6`](#rfc4724-4.1-6)

After the BGP speaker performs route selection, the forwarding state of the speaker MUST be updated and any previously marked stale information MUST be removed. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 4724 Section 4.1. Stage 2 -> Stage 5 startup race is fixed cleanly via subscribe-before-read and write-before-emit: emitSelectionDeferred calls ribevents.NoteSelectionDeferred(maxDelay) before emitting SelectionDeferred, emitSelectionComplete and selectionDeferral.stop call ribevents.ClearSelectionDeferred() before emitting SelectionComplete, and fibKernel.waitAndSweepStale reads ribevents.ActiveSelectionDeferral() immediately after subscribing to both events to seed initialDelay past active+fallbackDelay. TestRFC4724RestartingSpeakerForwardingState replicates the real Stage 2 -> Stage 5 order (NoteSelectionDeferred + emit BEFORE starting waitAndSweepStale), verifies no premature sweep past the 25ms fallback delay, proves processEvent updates forwarding state for 198.51.100.0/24 and adds 198.18.0.0/24, and proves ClearSelectionDeferred + SelectionComplete immediately sweeps unrefreshed stale 203.0.113.0/24 while preserving refreshed and fresh routes (negative).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L93) | unit/verify | revert, verified |
| positive | [`TestRFC4724RestartingSpeakerForwardingState`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go#L92) | unit/verify | revert, verified |

### [`RFC4724-4.1-7`](#rfc4724-4.1-7)

Once the initial update is complete for an address family (including the case that there is no routing update to send), the End-of-RIB marker MUST be sent. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-14 by an independent auditor who authored neither 08fe903803 nor the uncommitted phase 2 of spec-single-owner-lifecycle-2-bgp-session. RFC 4724 Section 4.1 read: "Once the initial update is complete for an address family (including the case that there is no routing update to send), the End-of-RIB marker MUST be sent." Unit read: TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete (both tags); only its two sync calls changed. 08fe903803 moved the initial-sync gate (sendingInitialRoutes) and the End-of-RIB owed mark (initialSyncEOROwed) from Peer onto Session. In the tagged units named stale it replaced peer.sendInitialRoutes() with peer.runInitialSyncNow(), which is Peer.sendInitialRoutes(p.testInitialSync()) (initial_sync_helpers_test.go): the same production sync body, run on the session the fixture installed (or one built with NewSession(p.settings) and published through establishedSession as enterEstablished does); it replaced peer.<flag>.Store(v) with peer.testInitialSync().<flag>.Store(v) or setTestInitialSync, and drainAndCloseQueueGate now takes that session. git diff 08fe903803^ against the working tree shows no assertion, expected byte, driven input or tag prose changed in any of them, each under the RFC-approved line 08fe903803 carries. Questions 1, 3 and 4 answer exactly as the earlier judgement below; question 2 still rests on the same producers, whose discrimination records are retaken by ./le rfc record after this stamp. Verdict unchanged: enforced. Earlier judgement follows. New unit TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete drives peer.sendInitialRoutes on an established peer and reads the socket. Positive: empty table -> the wire is exactly the IPv4 unicast EoR (the RFC's "including the case that there is no routing update to send"); with default-originate -> [UPDATE 0.0.0.0/0, EoR], EORSent 1. Negative: no EoR precedes the route and exactly one follows it (a marker before completion violates "Once the initial update is complete"). Observed-red records +/- (revert peer_initial_sync.go::sendEORFamilies); both polarities share one break, which reddens the unit as a whole rather than the ordering clause alone. Message-level EoR encoding units stay as supplementary.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L228) | unit/verify | not verified |
| negative | [`TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_initial_update_eor_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L20) | unit/verify | not verified |
| positive | [`TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_initial_update_eor_test.go#L42) | unit/verify | revert, verified |

### [`RFC4724-4.1-8`](#rfc4724-4.1-8)

To put an upper bound on the amount of time a router defers its route selection, an implementation MUST support a (configurable) timer that imposes this upper bound. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 4724 Section 4.1. TestRFC4724RIBSelectionDeferralPerFamilyAndTimer configures bgp update-delay max-delay using Ze's real string-serialized YANG leaf across two distinct positive values ("45" -> held at 44s, released + selected into Loc-RIB + SelectionComplete at 45s; "90" -> held at 89s, released + selected at 90s) and "0" (negative -> does not arm deferral, selects immediately). TestRFC4724SelectionDeferralTimerAndExclusions likewise proves 45s, 90s, and 0 in reactor updateDelayHold.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724RIBSelectionDeferralPerFamilyAndTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_selection_deferral_test.go#L92) | unit/verify | revert, verified |
| negative | [`TestRFC4724SelectionDeferralTimerAndExclusions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restarting_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC4724RIBSelectionDeferralPerFamilyAndTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_selection_deferral_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestRFC4724SelectionDeferralTimerAndExclusions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restarting_test.go#L31) | unit/verify | revert, verified |

### [`RFC4724-4.2-1`](#rfc4724-4.2-1)

In case it does not detect the termination of the old TCP session and still considers the BGP session as being established, it MUST treat the subsequent open connection from the peer as an indication of the termination of the old TCP session and act accordingly (when the Graceful Restart Capability has been received from the peer). (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/rfc4724_collision_test.go::TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification, internal/component/bgp/reactor/rfc4724_collision_test.go::TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification#2 moved. Independent audit against RFC 4724 Section 4.2 and Section 5. TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification establishes a real passive session over wire OPEN+KEEPALIVE with peer GR capability carrying 1 AFI/SAFI (local BGP ID > remote BGP ID to isolate from RFC 4271 tie-break) and proves detectCollision returns (true, true) and finishCollisionHandoff treats the subsequent connection as termination of the old TCP session (0 bytes / io.EOF on old socket, no NOTIFICATION, new connection retained). Negative subtests isolate both no-GR and 0-family empty GR capabilities (with local ID < remote ID), asserting detectCollision returns (false, false) and no-GR rejects the new connection with NOTIFICATION Cease/Connection Collision (6/7) while keeping the old session open.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_collision_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_collision_test.go#L96) | unit/verify | revert, verified |

### [`RFC4724-4.2-2`](#rfc4724-4.2-2)

"Acting accordingly" in this context means that the previous TCP session MUST be closed, and the new one retained. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/rfc4724_collision_test.go::TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification, internal/component/bgp/reactor/rfc4724_collision_test.go::TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification#2 moved. Independent audit against RFC 4724 Section 4.2. Positive subtest drives Peer.finishCollisionHandoff on an Established session that received GR families and proves the previous TCP connection is closed with 0 bytes read and io.EOF (no NOTIFICATION sent) while the new connection remains retained in peer.inbound and readable/writable (byte written by newClient received on newServer). Negative subtest drives finishCollisionHandoff without GR families and proves the new connection receives NOTIFICATION Cease/Connection Collision (6/7) while the previous TCP session remains open (oldSession.Conn() != nil).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_collision_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_collision_test.go#L98) | unit/verify | revert, verified |

### [`RFC4724-4.2-3`](#rfc4724-4.2-3)

When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit after 26d70dfa0d against RFC 4724 Section 4.2. First DOWN: retainPeerFamilies sends the same retain-routes on-down families for every family of the received GR capability, now reading the RIB answer (an empty earlier-connections list records nothing), then handleStructuredState sends the same peer-wide mark-stale with the Restart Time. newGRWithRealRIB now passes the RIB's real answers, so TestRFC4724SessionDownRetainsAndMarksRoutesStale exercises the answer-reading path and still requires IPv4 kept at stale level 1 with unadvertised IPv6 deleted, and both families at level 1 when both are advertised; TestRFC4724SessionDownWithoutCapabilityRetainsNothing requires deletion without the capability; TestRFC4724RetentionCoversEveryAdvertisedFamily and the state-manager pair are unchanged controls. The commit also closes a consecutive-restart hole (routes a later connection sent for a family already in LLGR were retained unmarked), now marked by restalePreservedLLGR and proven by the untagged-for-this-row real-RIB root TestRFC9494ConsecutiveRestartMarksTheLaterConnectionsRoutes.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRStateManagerNoGRCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L297) | unit/verify | not verified |
| negative | [`TestRFC4724SessionDownWithoutCapabilityRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L42) | unit/verify | not verified |
| positive | [`TestRFC4724RetentionCoversEveryAdvertisedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L167) | unit/verify | not verified |
| positive | [`TestRFC4724SessionDownRetainsAndMarksRoutesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L69) | unit/verify | revert, verified |

### [`RFC4724-4.2-4`](#rfc4724-4.2-4)

To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit after 26d70dfa0d. For a conventional consecutive restart onSessionDownPlan still sets purgeAllPrevious and handleStructuredState sends purge-stale to the previous cycle (dispatchRetained: the previous connection alone when no LLGR family was preserved) before retain-routes and mark-stale. The RIB no longer refuses a retention while the previous connection still owns routes (retainPhysicalStateLocked keeps them as earlier connections), so the purge is now the only thing that deletes the old generation; the real-RIB roots TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes and TestRFC4724ConsecutiveRestartDeletesBeforeMarking, which now run through the RIB's real answers, still require the unrefreshed 198.51.100.0/24 absent and the refreshed 198.51.101.0/24 at stale level 1 after the second drop, so an omitted purge (routes kept as earlier) or a purge after marking (refreshed route lost) both go red. TestGRConsecutiveRestartClearsPriorStale (both tags) still contrasts the replaced stale set with a fresh peer's first drop. RFC 9494 Section 4.2 overrides deletion only for families in a running LLST period, row RFC9494-4.2-7.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L286) | unit/verify | not verified |
| negative | [`TestRFC4724ConsecutiveRestartDeletesBeforeMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L51) | unit/verify | revert, verified |
| positive | [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L272) | unit/verify | not verified |
| positive | [`TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L40) | unit/verify | revert, verified |

### [`RFC4724-4.2-5`](#rfc4724-4.2-5)

The router MUST NOT differentiate between stale and other routing information during forwarding. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c25 judge). Positive TestComparePair_GRStaleCompetesNormally: a level-1 stale route with the higher LOCAL_PREF wins. New genuine negative (R1(b)) TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone drives SelectBest toward both violations in both argument orders: a fresh route with the higher LOCAL_PREF beats the stale one (staleness does not favour) and with all attributes equal the stale route from the lower address wins the tie-break (staleness costs nothing). Overlay probes: favouring level 1 reds only the negative; lowering the depreference threshold to 1 reds the positive and the tie subtest. Record: comparePair revert (-). The old negative tag on TestComparePair_LLGRStale proved RFC 9494 depreference and was removed (D-15). Proof is at the RIB best path the forwarding plane is handed; sysrib and fib-kernel hold no GR-stale distinction.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_stale_forwarding_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L852) | unit/verify | revert, verified |

### [`RFC4724-4.2-6`](#rfc4724-4.2-6)

In re-establishing the session, the "Restart State" bit in the Graceful Restart Capability of the OPEN message sent by the Receiving Speaker MUST NOT be set unless the Receiving Speaker has restarted. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged independently against RFC 4724 Section 4.2: "In re-establishing the session, the "Restart State" bit in the Graceful Restart Capability of the OPEN message sent by the Receiving Speaker MUST NOT be set unless the Receiving Speaker has restarted." TestRFC4724OpenOutsideTheRestartWindowClearsRestartState parses OPEN after cold-start and expired deadlines and asserts R clear with Restart Time 120. Its opposite, TestRFC4724OpenInsideTheRestartWindowSetsRestartState, parses the same configured payload inside the restart window and asserts R set, Time 120 and the unchanged IPv4/unicast tuple, so a producer that never sets R does not pass the pair. The test-only wire view still invokes the production Peer.pluginCapabilitySnapshot and restartFlagsFor; buildOpenFromSnapshot encodes that captured offer. The supplementary grmarker units prove the R-bit edit and preservation of the caller's unmodified payload, not the runtime window gate. The fixture adaptation and helper-comment correction preserve the original test inputs, tags and assertions. Discrimination freshness is checked separately by the main thread; this rejudgment makes no new execution claim.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L266) | unit/verify | revert, verified |
| negative | [`TestRFC4724OpenInsideTheRestartWindowSetsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L497) | unit/verify | revert, verified |
| positive | [`TestRFC4724OpenOutsideTheRestartWindowClearsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L86) | unit/verify | revert, verified |

### [`RFC4724-4.2-7`](#rfc4724-4.2-7)

If the session does not get re-established within the "Restart Time" that the peer advertised previously, the Receiving Speaker MUST delete all the stale routes from the peer that it is retaining. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit after 26d70dfa0d. releaseRoutes now sends release-routes through dispatchRetained and then forgetEarlierRetained; with no earlier connection recorded (every plain restart) that is exactly the one release-routes to the retained connection, as before. A peer only holds earlier connections while an LLGR family's period runs, where RFC 9494 Section 4.2 replaces this sentence, so the multi-connection release is not this row's. Tagged roots unchanged: TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes requires release-routes owner 701 connection 41 after a 1s Restart Time; TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking requires retain, mark-stale then release order and real-RIB deletion on both event paths; TestGRStateManagerTimerExpiry the state removal; negatives TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes (nothing dispatched for 2s after a timely reconnect), TestGRStateManagerReconnectWithFBit, TestGRStateManagerRestartExpiryRejectsOldCycle and TestGRStateManagerRestartExpiryAfterReconnect.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L105) | unit/verify | not verified |
| negative | [`TestGRStateManagerRestartExpiryAfterReconnect`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L428) | unit/verify | revert, verified |
| negative | [`TestGRStateManagerRestartExpiryRejectsOldCycle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L379) | unit/verify | revert, verified |
| negative | [`TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L158) | unit/verify | mutant, verified |
| positive | [`TestGRStateManagerTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L139) | unit/verify | mutant, verified |
| positive | [`TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L115) | unit/verify | revert, verified |

### [`RFC4724-4.2-8`](#rfc4724-4.2-8)

Once the session is re-established, if the "Forwarding State" bit for a specific address family is not set in the newly received Graceful Restart Capability, or if a specific address family is not included in the newly received Graceful Restart Capability, or if the Graceful Restart Capability is not received in the re-established session at all, then the Receiving Speaker MUST immediately remove all the stale routes from the peer that it is retaining for that address family. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 4724 Section 4.2 and current gr.go/gr_state.go (post-18b491ae23). Section 4.2 requires immediate removal of retained stale routes for a family on re-establishment when the new GR capability clears F, omits the family, or is absent altogether. In onSessionReestablished, newCap==nil stops all timers and purges every stale family, while reconnectKeepsFamily returns false whenever !grForwarding[fam], purging that family and stopping any LLST timer on it; handleStructuredState dispatches source purge-stale per purged family. All 5 tagged units verified: TestRFC4724ReestablishedWithoutForwardingStatePurgesTheFamily covers all three triggers (F clear, family omitted, cap 65 only) through JSON handleEvent OPEN + UP; TestRFC4724ReestablishedWithoutCapabilityPurgesOnTheStructuredPath covers cap-65-only and empty-optional-params on the structured OPEN/DOWN/OPEN/UP path; TestGRStateManagerReconnectFBitZero tests per-family F=0 purge vs F=1 keep; TestGRStateManagerReconnectWithFBit and TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes verify F=1 keeps stale routes with zero purges.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L102) | unit/verify | not verified |
| negative | [`TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L233) | unit/verify | mutant, verified |
| positive | [`TestGRStateManagerReconnectFBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L143) | unit/verify | not verified |
| positive | [`TestRFC4724ReestablishedWithoutCapabilityPurgesOnTheStructuredPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_capability_absent_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC4724ReestablishedWithoutForwardingStatePurgesTheFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L186) | unit/verify | mutant, verified |

### [`RFC4724-4.2-9`](#rfc4724-4.2-9)

The Receiving Speaker MUST send the End-of-RIB marker once it completes the initial update for an address family (including the case that it has no routes to send) to the peer. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-14 by an independent auditor who authored neither 08fe903803 nor the uncommitted phase 2 of spec-single-owner-lifecycle-2-bgp-session. RFC 4724 Section 4.2 read: "The Receiving Speaker MUST send the End-of-RIB marker once it completes the initial update for an address family (including the case that it has no routes to send) to the peer." Units read: TestInitialSyncEORReachesTheSilentFamilyToo, TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes; only their sync calls changed. 08fe903803 moved the initial-sync gate (sendingInitialRoutes) and the End-of-RIB owed mark (initialSyncEOROwed) from Peer onto Session. In the tagged units named stale it replaced peer.sendInitialRoutes() with peer.runInitialSyncNow(), which is Peer.sendInitialRoutes(p.testInitialSync()) (initial_sync_helpers_test.go): the same production sync body, run on the session the fixture installed (or one built with NewSession(p.settings) and published through establishedSession as enterEstablished does); it replaced peer.<flag>.Store(v) with peer.testInitialSync().<flag>.Store(v) or setTestInitialSync, and drainAndCloseQueueGate now takes that session. git diff 08fe903803^ against the working tree shows no assertion, expected byte, driven input or tag prose changed in any of them, each under the RFC-approved line 08fe903803 carries. Questions 1, 3 and 4 answer exactly as the earlier judgement below; question 2 still rests on the same producers, whose discrimination records are retaken by ./le rfc record after this stamp. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-10-12 (independent auditor). SHIFTED by 4e731b2d96 (owner-approved lint-only reactor test edits). The tagged unit is byte-identical (gate-verified); the file-level changes are comment punctuation, slices.Concat over the same bytes, joined appends, strings.SplitSeq, newDefaultOriginatePeer taking its fixed addresses from constants with the same values, and registerTypeLenRecognizer losing a lo every caller passed as 1, none of which changes what a helper this unit calls produces. Verdict stands. Positive TestInitialSyncEORReachesTheSilentFamilyToo proves the marker is sent including for a family with no routes. Negative TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes: no End-of-RIB before the initial update for IPv4 unicast completes (plugin barrier held, sendInitialRoutes observed waiting, wire empty), then exactly that marker once it does. TestBuildEOR_IPv4Unicast proves encoding only. Rejudged 2026-10-12 after commit ce62c78c3b moved peer_initial_sync_test.go around these units: the tagged functions there were re-read and are byte-identical, their assertions and fixtures still carry the evidence this note cites, and the other cited units did not change, so the verdict stands. Rejudged 2026-10-12 after 7a637d6df5 renamed SendUpdateHeld to sendUpdateHeld in two comments of peer_initial_sync_test.go: every tagged unit cited here is byte-identical (gate-verified), the comment edits sit outside them, and the assertions and fixtures still carry the evidence this note cites, so the verdict stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L229) | unit/verify | not verified |
| negative | [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L116) | unit/verify | not verified |

### [`RFC4724-4.2-10`](#rfc4724-4.2-10)

The Receiving Speaker MUST replace the stale routes by the routing updates received from the peer. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Insert of a stale prefix clears StaleLevel to fresh with the new attributes; the negative keeps an unrefreshed stale prefix stale while a new prefix is fresh

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestFamilyRIB_InsertNewDuringStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L179) | unit/verify | revert, verified |
| positive | [`TestFamilyRIB_InsertClearsStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L147) | unit/verify | revert, verified |

### [`RFC4724-4.2-11`](#rfc4724-4.2-11)

Once the End-of-RIB marker for an address family is received from the peer, it MUST immediately remove any routes from the peer that are still marked as stale for that address family. (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), stale-unit: internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go::TestRFC9494ConsecutiveRestartEORDeletesEveryConnectionsStaleRoutes moved. Independent re-audit after 26d70dfa0d. For a single restart handleEOREvent still sends exactly one purge-stale for the End-of-RIB family to the retained connection, and the tagged roots still pin it: TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes requires the exact single command, TestRFC4724EndOfRIBLeavesOtherFamiliesStale no IPv6 or whole-peer purge and nothing for an unretained family, and the state-manager pair the per-family bookkeeping. FINDING, whole requirement: after a consecutive restart during the LLGR period, the routes still marked stale for the family belong to more than one connection (41 and 42), and all are routes from the peer. 26d70dfa0d changed handleEOREvent from dispatchSource to dispatchRetained because the purge named only the latest connection and left connection 41's stale routes in place; every tagged root of this row stayed green under that defect and would stay green if the change were reverted, because none drives a second retained connection. The untagged real-RIB root TestRFC9494ConsecutiveRestartEORDeletesEveryConnectionsStaleRoutes does require both stale routes absent and the refreshed one fresh after the third connection's End-of-RIB; tagging it as this row's positive, with its discrimination record, would close the finding.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRStateManagerEORForNonGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L314) | unit/verify | not verified |
| negative | [`TestRFC4724EndOfRIBLeavesOtherFamiliesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L112) | unit/verify | mutant, verified |
| positive | [`TestGRStateManagerEORPurge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L185) | unit/verify | not verified |
| positive | [`TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L90) | unit/verify | mutant, verified |
| positive | [`TestRFC9494ConsecutiveRestartEORDeletesEveryConnectionsStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go#L200) | unit/verify | revert, verified |

### [`RFC4724-4-4`](#rfc4724-4-4)

A BGP speaker MAY advertise the Graceful Restart Capability for an address family to its peer if it has the ability to preserve its forwarding state for the address family when BGP restarts. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 4724 Section 4 ('A BGP speaker MAY advertise the Graceful Restart Capability for an address family to its peer if it has the ability to preserve its forwarding state for the address family when BGP restarts') after moving code-64 parameters into graceful-restart > short-lived. parseGRCapValue (gr_capability.go) emits '0000' when short-lived is absent, and when short-lived is present lists carried session families (or short-lived > family subset) gated by family.LookupFamily and nlrisplit.Supported(fam) with F-bit clear at config time (restartFlagsFor sets F on restart). All 4 tagged tests drive the full production config chain (LoadConfig -> ToPluginMap -> ExtractConfigSubtree -> refuseUncarriedGRFamilies -> extractGRCapabilities): positives TestRFC4724GRCapabilityListsTheFamiliesOfTheSession ('012c0001010000020100') and TestRFC4724GRCapabilityKeepsEveryFamilyTheRIBStores ('00780001850000010100', empty warnings); negatives TestRFC4724GRCapabilityClaimsNoFamilyItDoesNotCarry ('007800020100', no uncarried ipv4 tuple, F=0) and TestRFC4724GRCapabilityOmitsAFamilyNoRIBStores (runtime ipv4/rib-less SAFI 241 with !nlrisplit.Supported omitted with Warn naming peer and family, yielding '007800010100' for session list and '0078' for configured list naming only it). All 4 units carry current parseGRCapValue revert records (producer-sha a3291e8e472c81e7) in rfc/discrimination/rfc4724/RFC4724-4-4.json.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724GRCapabilityClaimsNoFamilyItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_gr_capability_test.go#L128) | unit/verify | revert, verified |
| negative | [`TestRFC4724GRCapabilityOmitsAFamilyNoRIBStores`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_rib_families_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC4724GRCapabilityListsTheFamiliesOfTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_gr_capability_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestRFC4724GRCapabilityKeepsEveryFamilyTheRIBStores`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_rib_families_test.go#L93) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc4724.txt |
| Source fingerprint | 2941f918826e837a |
| Record | rfc/extraction/rfc4724.json |
| Mapped sentences | 25 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 5 | reviewed | not stated |
| `4` | not stated | 3 | reviewed | not stated |
| `4.1` | not stated | 7 | reviewed | not stated |
| `4.2` | not stated | 11 | reviewed | not stated |
| `5` | not stated | 2 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `10.1` | not stated | 0 | reviewed | not stated |
| `10.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A pointer sentence that hands the reader to sections 4.1 and 4.2. It adds no obligation of its own; the procedures it promises are the sites in those two sections. | The following sections detail the procedures that MUST be followed by the Restarting Speaker as well as the Receiving Speaker once the Restarting Speaker restarts. |
| `5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The quoted [BGP-4] sentence that section 5 removes. It sits under 'Replace this text:' and the next site carries the replacement, so this is the text RFC 4724 deletes rather than an obligation it imposes. | In response to an indication that the TCP connection is successfully established (Event 16 or Event 17), the second connection SHALL be tracked until it sends an OPEN message. |
| `5:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The replacement keeps [BGP-4] Section 8.2.2's own collision handling for the case the Graceful Restart Capability was not received; section 5 opens 'The specific state machine modifications to [BGP-4], Section 8.2.2, are as follows.' RFC 4724 adds only the guard. The branch it does add, where the capability WAS received, is recorded as RFC4724-4.2-1 and RFC4724-4.2-2. | with If the Graceful Restart Capability with one or more AFIs/SAFIs has not been received for the session, then in response to an indication that a TCP connection is successfully established (Event 16 or Event 17), the second connection SHALL be tracked until it sends an OPEN message. |

## Superseded

No document obsoletes RFC 4724, so its obligations are stated where they were written.
