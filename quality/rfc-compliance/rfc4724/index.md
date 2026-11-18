# RFC 4724 - Graceful Restart Mechanism for BGP

Graceful Restart. Every requirement listed for RFC 4724, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 16 | of 26 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 26 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 26 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 26 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 25 | 25 of 76 tests linked to requirements, 0 without a fault test and 17 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 9 | of 26 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

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
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Graceful Restart |
| Automatic checks | Included |
| Requirements | 31 |
| Mandatory requirements (MUST level) | 26 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 9 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 76 |
| Tests linked to requirements | 76 |
| Recorded audit verdicts | 17 |
| Fault test records | 42 |
| Summary | `rfc/short/rfc4724.md` |
| Requirement table | `rfc/requirements/rfc4724.md` |
| RFC text | `rfc/full/rfc4724.txt` |

## Automatic checks

Included in automatic checks: Graceful Restart Mechanism for BGP (RFC 4724): 16 MET (GR capability advertise + last-instance negotiate, Restart-State/Forwarding-State bit encoding, End-of-RIB send + detect, mark-stale on non-NOTIFICATION drop, stale deletion on consecutive restart / Restart-Time expiry / F-bit-clear / no-GR-cap re-establish, GR-stale level-1 competes normally in best-path) + 1 single-polarity positive + 9 gap (GR-capability collision-detection override and related receiving-speaker obligations, and the no-wait-for-End-of-RIB rule ze meets with no test)

## Published support status

**Area:** Graceful Restart

**Test verification:** 9 of 26 MUST proven

**Evidence:** interop 1, unit 8

**Unmet:** RFC4724-3-5, RFC4724-4.1-1, RFC4724-4.1-2, RFC4724-4.1-3, RFC4724-4.1-5, RFC4724-4.1-6, RFC4724-4.1-8, RFC4724-4.2-1, RFC4724-4.2-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 16 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 10 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **26** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (16):** [`RFC4724-3-2`](#rfc4724-3-2), [`RFC4724-3-3`](#rfc4724-3-3), [`RFC4724-3-4`](#rfc4724-3-4), [`RFC4724-4-1`](#rfc4724-4-1), [`RFC4724-4-2`](#rfc4724-4-2), [`RFC4724-4.1-4`](#rfc4724-4.1-4), [`RFC4724-4.1-7`](#rfc4724-4.1-7), [`RFC4724-4.2-3`](#rfc4724-4.2-3), [`RFC4724-4.2-4`](#rfc4724-4.2-4), [`RFC4724-4.2-5`](#rfc4724-4.2-5), [`RFC4724-4.2-6`](#rfc4724-4.2-6), [`RFC4724-4.2-7`](#rfc4724-4.2-7), [`RFC4724-4.2-8`](#rfc4724-4.2-8), [`RFC4724-4.2-9`](#rfc4724-4.2-9), [`RFC4724-4.2-10`](#rfc4724-4.2-10), [`RFC4724-4.2-11`](#rfc4724-4.2-11)

**Annotated (including scoped evidence) (10):** [`RFC4724-3-1`](#rfc4724-3-1), [`RFC4724-3-5`](#rfc4724-3-5), [`RFC4724-4.1-1`](#rfc4724-4.1-1), [`RFC4724-4.1-2`](#rfc4724-4.1-2), [`RFC4724-4.1-3`](#rfc4724-4.1-3), [`RFC4724-4.1-5`](#rfc4724-4.1-5), [`RFC4724-4.1-6`](#rfc4724-4.1-6), [`RFC4724-4.1-8`](#rfc4724-4.1-8), [`RFC4724-4.2-1`](#rfc4724-4.2-1), [`RFC4724-4.2-2`](#rfc4724-4.2-2)

## Figures

- **MUST bar**: Ze verified 9, Tests awaiting verification 8, Not built 9
- **Overview bar**: Ze MUST verified 9, SHOULD verified 1, Tests awaiting verification 8, Not assessed 4, Not built 9

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 26 |
| MUST rows kept (the MUST bar) | 26 |
| Ze verified | 9 |
| OS verified | 0 |
| Tests awaiting verification | 8 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 9 |
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
| `RFC4724-3-1` | A BGP speaker MUST NOT include more than one instance of the Graceful Restart Capability in the capability advertisement (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestExtractGRCapabilities_CapabilityDecl`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L111). **negative:** no negative test. **{single-polarity}:** ze's GR sender emits exactly one code-64 declaration per peer (internal/component/bgp/plugins/gr/gr.go:703 extractGRCapabilities appends one CapabilityDecl per peer) and the encoder writes a single TLV (internal/core/bgp/capability/capability.go:553 WriteTo); there is no code path that emits two instances, so the more-than-one case cannot be constructed to test negatively |
| `RFC4724-3-2` | The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L478). **positive:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedRestartFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L53). **negative:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L489). **negative:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedRestartFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L54) |
| `RFC4724-3-3` | The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L484). **positive:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L73). **negative:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L498). **negative:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L74) |
| `RFC4724-3-4` | If more than one instance of the Graceful Restart Capability is carried in the capability advertisement, the receiver of the advertisement MUST ignore all but the last instance of the Graceful Restart Capability. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L62). **negative:** `unit/verify` [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L70) |
| `RFC4724-3-5` | When set (value 1), this bit indicates that the BGP speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker from the speaker before advertising routing information to the speaker. (Section 3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/component/bgp/reactor/peer_initial_sync.go::sendInitialRoutes meets it, since ze advertises its Adj-RIB-Out and End-of-RIB on reaching Established without waiting for the peer's End-of-RIB, and the only End-of-RIB timer (session_health.go, startEORTimer) raises a health warning and defers nothing; no test asserts it |
| `RFC4724-4-1` | The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L16). **positive:** `unit/verify` [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L111). **positive:** `unit/verify` [`TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L764). **positive:** `unit/verify` [`TestInitialSyncMarkerWaitsForNoProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L797). **positive:** `unit/verify` [`TestInitialSyncShutsTheQueueGateAndFreesTheRailsWithTheMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L356). **positive:** `unit/verify` [`TestPredecessorSyncWritesNothingOnSuccessor`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_rapid_reconnect_test.go#L95). **positive:** `unit/verify` [`TestRoutePushingBindingsCountBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L437). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L224). **negative:** `unit/verify` [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L23). **positive:** `interop/nightly` [`checkNoFamilyEndOfRIB`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1280) |
| `RFC4724-4-2` | It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestGRStateManagerNotificationBypass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L220). **positive:** `unit/verify` [`TestPeerCloseNamesTheNotificationTheNeighborSent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_close_test.go#L46). **positive:** `unit/verify` [`TestRFC4724NotificationCloseRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_close_reason_test.go#L25). **negative:** `unit/verify` [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L46). **negative:** `unit/verify` [`TestPeerCloseNamesTheTransportLoss`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_close_test.go#L74). **negative:** `unit/verify` [`TestRFC4724TransportCloseRetains`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_close_reason_test.go#L47) |
| `RFC4724-4.1-1` | When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's Restarting Speaker path implements only GR signaling -- it writes a restart marker and sets the R bit (internal/component/bgp/grmarker/grmarker.go, internal/component/bgp/reactor/peer.go:574) -- and does not retain its own in-memory Loc-RIB forwarding state across a process restart within the bgp packages; the Loc-RIB is rebuilt from scratch on restart |
| `RFC4724-4.1-2` | When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB and MUST mark them as stale. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** because ze does not retain its own Loc-RIB across a restart (see RFC4724-4.1-1), there is no retained own-forwarding state to mark stale; the stale-marking machinery (internal/component/bgp/plugins/rib/rib_commands.go:817 markStaleCommand) applies to routes received from a restarting peer, not to ze's own routes on ze's restart |
| `RFC4724-4.1-3` | It MUST NOT differentiate between stale and other information during forwarding. (Section 4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this governs forwarding over ze's own retained stale Loc-RIB, which ze does not build on restart (see RFC4724-4.1-1); the generic non-differentiation of level-1 stale in best-path selection (internal/component/bgp/plugins/rib/bestpath.go:308) is the Receiving Speaker path for peer routes, not ze's own routes as a Restarting Speaker |
| `RFC4724-4.1-4` | To re-establish the session with its peer, the Restarting Speaker MUST set the "Restart State" bit in the Graceful Restart Capability of the OPEN message. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4724OpenInsideTheRestartWindowSetsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L65). **positive:** `unit/verify` [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L262). **negative:** `unit/verify` [`TestRFC4724OpenOutsideTheRestartWindowClearsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L87). **negative:** `unit/verify` [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L500) |
| `RFC4724-4.1-5` | However, it MUST defer route selection for an address family until it either (a) receives the End-of-RIB marker from all its peers (excluding the ones with the "Restart State" bit set in the received capability and excluding the ones that do not advertise the graceful restart capability) or (b) the Selection_Deferral_Timer referred to below has expired. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs best-path selection as updates arrive and has no selection-deferral path keyed on End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and does not gate route selection |
| `RFC4724-4.1-6` | After the BGP speaker performs route selection, the forwarding state of the speaker MUST be updated and any previously marked stale information MUST be removed. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this is the completion step of the deferred-selection cycle ze does not run (see RFC4724-4.1-5); with no own-Loc-RIB stale state (RFC4724-4.1-1) there is no post-selection stale removal on ze's own restart |
| `RFC4724-4.1-7` | Once the initial update is complete for an address family (including the case that there is no routing update to send), the End-of-RIB marker MUST be sent. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L20). **positive:** `unit/verify` [`TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_initial_update_eor_test.go#L42). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L228). **negative:** `unit/verify` [`TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_initial_update_eor_test.go#L44) |
| `RFC4724-4.1-8` | To put an upper bound on the amount of time a router defers its route selection, an implementation MUST support a (configurable) timer that imposes this upper bound. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze exposes no Selection_Deferral_Timer configuration -- the GR YANG model (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang) carries restart-time and long-lived-stale-time only, and no code defers selection (see RFC4724-4.1-5) |
| `RFC4724-4.2-1` | In case it does not detect the termination of the old TCP session and still considers the BGP session as being established, it MUST treat the subsequent open connection from the peer as an indication of the termination of the old TCP session and act accordingly (when the Graceful Restart Capability has been received from the peer). (Section 4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze follows plain RFC 4271 Section 6.8 collision detection -- a new inbound connection while the session is Established is rejected with Cease/Connection Collision (internal/component/bgp/reactor/reactor_connection.go:134-136), with no GR-capability branch that treats the new OPEN as terminating the old session |
| `RFC4724-4.2-2` | "Acting accordingly" in this context means that the previous TCP session MUST be closed, and the new one retained. (Section 4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same RFC 4271 collision path closes the NEW connection and keeps the existing Established session (internal/component/bgp/reactor/reactor_connection.go:134-136 rejectConnectionCollisionWithSettings), the opposite of the RFC 4724 Section 4.2 override that closes the previous session and retains the new one |
| `RFC4724-4.2-3` | When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L42). **positive:** `unit/verify` [`TestRFC4724RetentionCoversEveryAdvertisedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L167). **positive:** `unit/verify` [`TestRFC4724SessionDownRetainsAndMarksRoutesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L69). **negative:** `unit/verify` [`TestGRStateManagerNoGRCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L297). **negative:** `unit/verify` [`TestRFC4724SessionDownWithoutCapabilityRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L97) |
| `RFC4724-4.2-4` | To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L272). **positive:** `unit/verify` [`TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L40). **negative:** `unit/verify` [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L286). **negative:** `unit/verify` [`TestRFC4724ConsecutiveRestartDeletesBeforeMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L51) |
| `RFC4724-4.2-5` | The router MUST NOT differentiate between stale and other routing information during forwarding. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L852). **negative:** `unit/verify` [`TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_stale_forwarding_test.go#L27) |
| `RFC4724-4.2-6` | In re-establishing the session, the "Restart State" bit in the Graceful Restart Capability of the OPEN message sent by the Receiving Speaker MUST NOT be set unless the Receiving Speaker has restarted. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestRFC4724OpenOutsideTheRestartWindowClearsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L86). **positive:** `unit/verify` [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L497). **negative:** `unit/verify` [`TestRFC4724OpenInsideTheRestartWindowSetsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L66). **negative:** `unit/verify` [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L266) |
| `RFC4724-4.2-7` | If the session does not get re-established within the "Restart Time" that the peer advertised previously, the Receiving Speaker MUST delete all the stale routes from the peer that it is retaining. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L67). **positive:** `unit/verify` [`TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L139). **positive:** `unit/verify` [`TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L115). **negative:** `unit/verify` [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L105). **negative:** `unit/verify` [`TestGRStateManagerRestartExpiryAfterReconnect`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L428). **negative:** `unit/verify` [`TestGRStateManagerRestartExpiryRejectsOldCycle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L379). **negative:** `unit/verify` [`TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L158) |
| `RFC4724-4.2-8` | Once the session is re-established, if the "Forwarding State" bit for a specific address family is not set in the newly received Graceful Restart Capability, or if a specific address family is not included in the newly received Graceful Restart Capability, or if the Graceful Restart Capability is not received in the re-established session at all, then the Receiving Speaker MUST immediately remove all the stale routes from the peer that it is retaining for that address family. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerReconnectFBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L143). **positive:** `unit/verify` [`TestRFC4724ReestablishedWithoutCapabilityPurgesOnTheStructuredPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_capability_absent_test.go#L32). **positive:** `unit/verify` [`TestRFC4724ReestablishedWithoutForwardingStatePurgesTheFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L186). **negative:** `unit/verify` [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L102). **negative:** `unit/verify` [`TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L233) |
| `RFC4724-4.2-9` | The Receiving Speaker MUST send the End-of-RIB marker once it completes the initial update for an address family (including the case that it has no routes to send) to the peer. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L22). **positive:** `unit/verify` [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L116). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L229). **negative:** `unit/verify` [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L24) |
| `RFC4724-4.2-10` | The Receiving Speaker MUST replace the stale routes by the routing updates received from the peer. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFamilyRIB_InsertClearsStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L147). **negative:** `unit/verify` [`TestFamilyRIB_InsertNewDuringStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L179) |
| `RFC4724-4.2-11` | Once the End-of-RIB marker for an address family is received from the peer, it MUST immediately remove any routes from the peer that are still marked as stale for that address family. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerEORPurge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L185). **positive:** `unit/verify` [`TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L90). **negative:** `unit/verify` [`TestGRStateManagerEORForNonGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L314). **negative:** `unit/verify` [`TestRFC4724EndOfRIBLeavesOtherFamiliesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L112) |
| `RFC4724-2-1` | Although the End-of-RIB marker is specified for the purpose of BGP graceful restart, it is noted that the generation of such a marker upon completion of the initial update would be useful for routing convergence in general, and thus the practice is recommended. (Section 2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4-3` | In addition, even if the speaker does not have the ability to preserve its forwarding state for any address family during BGP restart, it is still recommended that the speaker advertise the Graceful Restart Capability to its peer (Section 4) | RECOMMENDED | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4-4` | A BGP speaker MAY advertise the Graceful Restart Capability for an address family to its peer if it has the ability to preserve its forwarding state for the address family when BGP restarts. (Section 4) | MAY | 4 | **positive:** `unit/verify` [`TestRFC4724GRCapabilityKeepsEveryFamilyTheRIBStores`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_rib_families_test.go#L93). **positive:** `unit/verify` [`TestRFC4724GRCapabilityListsTheFamiliesOfTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_gr_capability_test.go#L101). **negative:** `unit/verify` [`TestRFC4724GRCapabilityClaimsNoFamilyItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_gr_capability_test.go#L128). **negative:** `unit/verify` [`TestRFC4724GRCapabilityOmitsAFamilyNoRIBStores`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_rib_families_test.go#L60) |
| `RFC4724-4.2-12` | In the event that it determines that its peer's forwarding state is not viable prior to the re-establishment of the session, the speaker MAY delete all the stale routes from the peer that it is retaining. (Section 4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4.2-13` | To put an upper bound on the amount of time a router retains the stale routes, an implementation MAY support a (configurable) timer that imposes this upper bound. (Section 4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4724-3-5`](#rfc4724-3-5) When set (value 1), this bit indicates that the BGP speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker from the speaker before advertising routing information to the speaker. (Section 3) | {gap}, no test | gap in proof: internal/component/bgp/reactor/peer_initial_sync.go::sendInitialRoutes meets it, since ze advertises its Adj-RIB-Out and End-of-RIB on reaching Established without waiting for the peer's End-of-RIB, and the only End-of-RIB timer (session_health.go, startEORTimer) raises a health warning and defers nothing; no test asserts it |
| [`RFC4724-4.1-1`](#rfc4724-4.1-1) When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB (Section 4.1) | {gap}, no test | ze's Restarting Speaker path implements only GR signaling -- it writes a restart marker and sets the R bit (internal/component/bgp/grmarker/grmarker.go, internal/component/bgp/reactor/peer.go:574) -- and does not retain its own in-memory Loc-RIB forwarding state across a process restart within the bgp packages; the Loc-RIB is rebuilt from scratch on restart |
| [`RFC4724-4.1-2`](#rfc4724-4.1-2) When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB and MUST mark them as stale. (Section 4.1) | {gap}, no test | because ze does not retain its own Loc-RIB across a restart (see RFC4724-4.1-1), there is no retained own-forwarding state to mark stale; the stale-marking machinery (internal/component/bgp/plugins/rib/rib_commands.go:817 markStaleCommand) applies to routes received from a restarting peer, not to ze's own routes on ze's restart |
| [`RFC4724-4.1-3`](#rfc4724-4.1-3) It MUST NOT differentiate between stale and other information during forwarding. (Section 4.1) | {gap}, no test | this governs forwarding over ze's own retained stale Loc-RIB, which ze does not build on restart (see RFC4724-4.1-1); the generic non-differentiation of level-1 stale in best-path selection (internal/component/bgp/plugins/rib/bestpath.go:308) is the Receiving Speaker path for peer routes, not ze's own routes as a Restarting Speaker |
| [`RFC4724-4.1-5`](#rfc4724-4.1-5) However, it MUST defer route selection for an address family until it either (a) receives the End-of-RIB marker from all its peers (excluding the ones with the "Restart State" bit set in the received capability and excluding the ones that do not advertise the graceful restart capability) or (b) the Selection_Deferral_Timer referred to below has expired. (Section 4.1) | {gap}, no test | ze runs best-path selection as updates arrive and has no selection-deferral path keyed on End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and does not gate route selection |
| [`RFC4724-4.1-6`](#rfc4724-4.1-6) After the BGP speaker performs route selection, the forwarding state of the speaker MUST be updated and any previously marked stale information MUST be removed. (Section 4.1) | {gap}, no test | this is the completion step of the deferred-selection cycle ze does not run (see RFC4724-4.1-5); with no own-Loc-RIB stale state (RFC4724-4.1-1) there is no post-selection stale removal on ze's own restart |
| [`RFC4724-4.1-8`](#rfc4724-4.1-8) To put an upper bound on the amount of time a router defers its route selection, an implementation MUST support a (configurable) timer that imposes this upper bound. (Section 4.1) | {gap}, no test | ze exposes no Selection_Deferral_Timer configuration -- the GR YANG model (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang) carries restart-time and long-lived-stale-time only, and no code defers selection (see RFC4724-4.1-5) |
| [`RFC4724-4.2-1`](#rfc4724-4.2-1) In case it does not detect the termination of the old TCP session and still considers the BGP session as being established, it MUST treat the subsequent open connection from the peer as an indication of the termination of the old TCP session and act accordingly (when the Graceful Restart Capability has been received from the peer). (Section 4.2) | {gap}, no test | ze follows plain RFC 4271 Section 6.8 collision detection -- a new inbound connection while the session is Established is rejected with Cease/Connection Collision (internal/component/bgp/reactor/reactor_connection.go:134-136), with no GR-capability branch that treats the new OPEN as terminating the old session |
| [`RFC4724-4.2-2`](#rfc4724-4.2-2) "Acting accordingly" in this context means that the previous TCP session MUST be closed, and the new one retained. (Section 4.2) | {gap}, no test | the same RFC 4271 collision path closes the NEW connection and keeps the existing Established session (internal/component/bgp/reactor/reactor_connection.go:134-136 rejectConnectionCollisionWithSettings), the opposite of the RFC 4724 Section 4.2 override that closes the previous session and retains the new one |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4724-3-1`](#rfc4724-3-1)

A BGP speaker MUST NOT include more than one instance of the Graceful Restart Capability in the capability advertisement (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestExtractGRCapabilities_CapabilityDecl`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L111) | unit/verify | not verified |

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
| negative | [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L70) | unit/verify | not verified |
| positive | [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L62) | unit/verify | not verified |

### [`RFC4724-3-5`](#rfc4724-3-5)

When set (value 1), this bit indicates that the BGP speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker from the speaker before advertising routing information to the speaker. (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4724-3-5, so there is no linked test.

### [`RFC4724-4-1`](#rfc4724-4-1)

The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/le/interoplab/bgp/check_rfc.go::checkNoFamilyEndOfRIB moved. Re-judged 2026-11-14 by an independent auditor who authored neither 4aa1f84ff6 nor its test change. RFC 4724 Section 4 read: "The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established." Stale unit TestInitialSyncMarkerWaitsForNoProcess re-read as committed: it names two route-pushing processes (sendUpdateOnly, sendRawOnly) plus an event-only listener, seeds the expectation set with the two pushers, and now raises the replay fence over both (raiseTestReplayFence sets replayFence and initialUpdateOwed), then runs the production sync body (runInitialSyncNow, Peer.sendInitialRoutes) with neither pusher reporting. It requires the sync to return within 2s, then asserts the fence is still up, zero reports credited (len apiSyncSignalled), and exactly the IPv4 unicast End-of-RIB bytes on the wire (eorWire equality). The deleted assertion (apiSyncReady channel still open) is replaced by fence-still-up plus no-credit over the same two silent processes: a sync that waited again for those reports, or for the fence to lower, never returns and goes red on the Eventually, and a marker missing or duplicated goes red on the byte equality. peer_initial_sync.go never reads initialUpdateOwed, so the extra assertions isolate that the marker left with the fence still raised. Question 1: the test asserts the RFC's own trigger, completion of ze's own initial update, not a process report. Question 2: yes, for a reintroduced wait or a missing marker. Question 3: the input is isolated, one negotiated family, no routes. Question 4: this unit proves the timing clause; the no-update case and every-family clause rest on the sibling positives below, unchanged. The tag's claim paragraph is unchanged and states no more than the body checks. The other cited units are byte-identical (TestInitialSyncEORReachesTheSilentFamilyToo only shifted). Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-11-14 by an independent auditor who authored neither 08fe903803 nor the uncommitted phase 2 of spec-single-owner-lifecycle-2-bgp-session. Units read: TestInitialSyncEORReachesTheSilentFamilyToo, TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily, TestInitialSyncMarkerWaitsForNoProcess, TestInitialSyncShutsTheQueueGateAndFreesTheRailsWithTheMarker, TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes (sync call only), and TestPredecessorSyncWritesNothingOnSuccessor (positive: the successor session carries exactly one UPDATE with the queued route, then the exact IPv4 unicast End-of-RIB, then no UPDATE for syncJoinProbe). checkNoFamilyEndOfRIB requires FRR's own bgp_update_receive_eor decode on the original session after the route arrived. Positives: sendInitialRoutes puts one End-of-RIB per negotiated family on the wire, including a silent family and a no-MP session. Negative TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes: while a plugin producing the initial update has not acknowledged peer-up, sendInitialRoutes is observed waiting and nothing is on the wire (EORSent 0); after the acknowledgement exactly the IPv4 unicast marker is written. Red on a marker sent ahead of the initial update or not sent after it. TestIsEndOfRIBAnyFamily is a neighbouring detector rule.

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

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerNotificationBypass, internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerRouteRetention, internal/component/bgp/plugins/gr/rfc4724_close_reason_test.go::TestRFC4724NotificationCloseRetainsNothing, internal/component/bgp/plugins/gr/rfc4724_close_reason_test.go::TestRFC4724TransportCloseRetains moved. Re-judged 2026-10-18 (independent auditor, after 4ef90497c6). Section 4: "It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message." The earlier finding (GR compared against "notification" while the reactor only ever sent "session closed" or "connection lost") is fixed: reactor/peer_close.go::sessionCloseFor returns rpc.ReasonNotification whenever notificationExchanged is set, and gr.go handleStructuredState and handleStateEvent compare against that constant. TestPeerCloseNamesTheNotificationTheNeighborSent drives a running Peer that receives a Cease and asserts the observer sees rpc.ReasonNotification with Event 25; TestRFC4724NotificationCloseRetainsNothing feeds that reason to the production GR callbacks and asserts no purge-stale, retain-routes or mark-stale dispatch, so the RIB clears the Adj-RIB-In on DOWN (rib.go, retainedPeers unset). Negatives TestPeerCloseNamesTheTransportLoss (TCP drop, rpc.ReasonTCPFailure) and TestRFC4724TransportCloseRetains (three-command sequence for TCPFailure and LocalClose) are genuine contrasts. The sending half has no tagged end-to-end unit: it rests on incrNotificationSent setting notificationExchanged (asserted by an untagged unit in reactor/peer_stats_test.go) and on the notified=true rows of untagged TestSessionCloseForCoversEveryCause. The older tagged gr_state_test.go units inject wasNotification and prove only the state-machine branch. Not covered: the text-event path handleStateEvent, which external GR plugins take.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L46) | unit/verify | not verified |
| negative | [`TestRFC4724TransportCloseRetains`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_close_reason_test.go#L47) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`TestPeerCloseNamesTheTransportLoss`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_close_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestGRStateManagerNotificationBypass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L220) | unit/verify | not verified |
| positive | [`TestRFC4724NotificationCloseRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_close_reason_test.go#L25) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestPeerCloseNamesTheNotificationTheNeighborSent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_close_test.go#L46) | unit/verify | revert, verified |

### [`RFC4724-4.1-1`](#rfc4724-4.1-1)

When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB (Section 4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4724-4.1-1, so there is no linked test.

### [`RFC4724-4.1-2`](#rfc4724-4.1-2)

When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB and MUST mark them as stale. (Section 4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4724-4.1-2, so there is no linked test.

### [`RFC4724-4.1-3`](#rfc4724-4.1-3)

It MUST NOT differentiate between stale and other information during forwarding. (Section 4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4724-4.1-3, so there is no linked test.

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

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4724-4.1-5, so there is no linked test.

### [`RFC4724-4.1-6`](#rfc4724-4.1-6)

After the BGP speaker performs route selection, the forwarding state of the speaker MUST be updated and any previously marked stale information MUST be removed. (Section 4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4724-4.1-6, so there is no linked test.

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

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4724-4.1-8, so there is no linked test.

### [`RFC4724-4.2-1`](#rfc4724-4.2-1)

In case it does not detect the termination of the old TCP session and still considers the BGP session as being established, it MUST treat the subsequent open connection from the peer as an indication of the termination of the old TCP session and act accordingly (when the Graceful Restart Capability has been received from the peer). (Section 4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4724-4.2-1, so there is no linked test.

### [`RFC4724-4.2-2`](#rfc4724-4.2-2)

"Acting accordingly" in this context means that the previous TCP session MUST be closed, and the new one retained. (Section 4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4724-4.2-2, so there is no linked test.

### [`RFC4724-4.2-3`](#rfc4724-4.2-3)

When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerNoGRCapability, internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerRouteRetention, internal/component/bgp/plugins/gr/rfc4724_retention_test.go::TestRFC4724RetentionCoversEveryAdvertisedFamily, internal/component/bgp/plugins/gr/rfc4724_retention_test.go::TestRFC4724SessionDownRetainsAndMarksRoutesStale, internal/component/bgp/plugins/gr/rfc4724_retention_test.go::TestRFC4724SessionDownWithoutCapabilityRetainsNothing moved. RFC 4724 §4.2: "When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information." Source rejudgment of all five tagged carriers: internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerRouteRetention checks activation, TestGRStateManagerNoGRCapability checks its absence; those alone are insufficient. internal/component/bgp/plugins/gr/rfc4724_retention_test.go::TestRFC4724SessionDownRetainsAndMarksRoutesStale now uses real_rib_test.go::newGRWithRealRIB and asserts received IPv4 retained at stale level 1, with an IPv6 family control retained at level 1 only when advertised; TestRFC4724SessionDownWithoutCapabilityRetainsNothing requires a pre-existing received route to disappear after DOWN without code 64. TestRFC4724RetentionCoversEveryAdvertisedFamily pins exactly IPv4+IPv6 in the state family set and the armed restart timer. Producers: gr.go::handleStateEvent -> gr_state.go::onSessionDownDeferred -> gr.go::retainPeerFamilies, then rib_commands.go::retainRoutes/markStaleCommand and rib.go::handleState. These assertions discriminate removal, missing stale marking and peer-wide overretention, rather than merely inspecting dispatched command text. Three positive tags, two negative tags; advertised versus absent capability/family are separate input cases. Both retention and stale marking clauses are checked at actual RIB storage; this says nothing about restarting-speaker Loc-RIB preservation or collision handling, whose existing gaps remain. No runtime or observed-red renewal was performed by this judge. Post-lint independent source rejudgment: both real-RIB tagged callers now omit only the invariant testPeer argument to received/down; those helpers still put testPeer in the event and verify the UPDATE entered actual received storage. The positive still asserts exact stale level 1, IPv6 retention iff advertised, and deletion of unadvertised IPv6; the no-GR negative requires actual route absence. Re-read all five carriers and handleStateEvent -> retainPeerFamilies/onSessionDownDeferred. newGRWithRealRIB reports mux.Close errors without skipping cancellation, plugin close or worker join. This adds cleanup failure visibility, not an alternative acceptable protocol outcome. Verdict remains enforced; both changed real-RIB tags currently have no discrimination record and owe first native observed-red records, not invented hashes.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRStateManagerNoGRCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L297) | unit/verify | not verified |
| negative | [`TestRFC4724SessionDownWithoutCapabilityRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L97) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L42) | unit/verify | not verified |
| positive | [`TestRFC4724RetentionCoversEveryAdvertisedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L167) | unit/verify | not verified |
| positive | [`TestRFC4724SessionDownRetainsAndMarksRoutesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L69) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC4724-4.2-4`](#rfc4724-4.2-4)

To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/gr/gr_state_test.go::TestGRConsecutiveRestartClearsPriorStale, internal/component/bgp/plugins/gr/gr_state_test.go::TestGRConsecutiveRestartClearsPriorStale#2 moved. RFC 4724 §4.2: "To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted." Read every tagged carrier: internal/component/bgp/plugins/gr/gr_state_test.go::TestGRConsecutiveRestartClearsPriorStale compares two-family then one-family state and a first-restart control; state-only evidence is supplemented by internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go::TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes and TestRFC4724ConsecutiveRestartDeletesBeforeMarking. Their rfc4724SecondRestart helper drives two actual GR DOWN cycles with the registered RIB engine, verifies both routes stale after the first, reestablishes and refreshes only 198.51.101.0/24, then requires unrefreshed 198.51.100.0/24 gone and refreshed 198.51.101.0/24 retained at stale level 1. gr.go::handleStateEvent purges before retainPeerFamilies and mark-stale; rib_commands.go::purgeStaleCommand removes old stale storage while markStaleCommand marks the new generation. Purging after marking or omitting purge violates different exact assertions. Two positive and two negative tags, including both tags on the state test; the real-RIB functions share one scenario and duplicate the two assertions, so they are not two independent experiments, but the old-stale and refreshed routes are distinct positive/control inputs rather than one assertion with two polarities. Restricted to conventional GR: RFC9494-4.2-7/-9 consecutive LLGR gaps are unchanged. No runtime or observed-red renewal was performed by this judge. Post-lint independent source rejudgment: the tagged assertion functions are unchanged by the argument simplification; their untagged rfc4724SecondRestart and realGRRIB helpers are affected. The helper still stores two routes, drops the session, proves both stale, reestablishes, refreshes only the second, and drops again. Both tagged functions require old-stale deletion and refreshed-route retention at level 1; these are distinct input-route outcomes despite duplicated functions. handleStateEvent still purges before retain/mark. Removed arguments were testPeer constants; the same received and DOWN events reach the registered RIB. Checked mux.Close cannot weaken this result. Preserve enforced and the pre-lint planned renewals for both existing records; no LLGR consecutive-restart claim is made.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L286) | unit/verify | not verified |
| negative | [`TestRFC4724ConsecutiveRestartDeletesBeforeMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L51) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L272) | unit/verify | not verified |
| positive | [`TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L40) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC4724-4.2-5`](#rfc4724-4.2-5)

The router MUST NOT differentiate between stale and other routing information during forwarding. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c25 judge). Positive TestComparePair_GRStaleCompetesNormally: a level-1 stale route with the higher LOCAL_PREF wins. New genuine negative (R1(b)) TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone drives SelectBest toward both violations in both argument orders: a fresh route with the higher LOCAL_PREF beats the stale one (staleness does not favour) and with all attributes equal the stale route from the lower address wins the tie-break (staleness costs nothing). Overlay probes: favouring level 1 reds only the negative; lowering the depreference threshold to 1 reds the positive and the tie subtest. Record: comparePair revert (-). The old negative tag on TestComparePair_LLGRStale proved RFC 9494 depreference and was removed (D-15). Proof is at the RIB best path the forwarding plane is handed; sysrib and fib-kernel hold no GR-stale distinction.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_stale_forwarding_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L852) | unit/verify | not verified |

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

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerReconnectWithFBit, internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerRestartExpiryAfterReconnect, internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerRestartExpiryRejectsOldCycle, internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerTimerExpiry, internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go::TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes, internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go::TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes, internal/component/bgp/plugins/gr/rfc4724_retention_test.go::TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking moved. Independent supplemental source rejudgment, 2026-10-06. RFC4724 Section 4.2: "If the session does not get re-established within the "Restart Time" that the peer advertised previously, the Receiving Speaker MUST delete all the stale routes from the peer that it is retaining." Read all seven tagged units: gr_state_test.go TestGRStateManagerTimerExpiry, TestGRStateManagerReconnectWithFBit, TestGRStateManagerRestartExpiryRejectsOldCycle, TestGRStateManagerRestartExpiryAfterReconnect; rfc4724_dispatch_test.go TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes and TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes; rfc4724_retention_test.go TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking. Explicit callback units prove owner/cancellation decisions, not elapsed time; the actual one-second timer dispatch and two-second timely-reconnect control prove complementary timing behavior. Zero-time test exercises both JSON state handler and structured EventKindState/SessionStateDown dispatch; it requires exact purge/retain/mark/release order, an actually populated registered RIB becoming empty, no active GR and no resurrection after RIB DOWN. real_rib_test.go consumes commands through registered bgp-rib RunEngine and checks verdicts and received route inventory, not just recorded strings. Traced onSessionDownDeferred -> completion after both handlers dispatch stale marking -> startRestartTimer anchored to original deadline -> handleTimerExpired owner/deadline guards -> wireStateCallbacks/onTimerExpired/releaseRoutes -> registered request bgp rib release-routes -> RIBManager.releaseRoutes, removing retained inventory and retention metadata and reconciling best path. Ordinary no-LLGR expiry is isolated; no absent LLGR feature is inferred. The moved zero-time unit has no current diff against HEAD; this is preexisting stale judgment debt, with history showing the integrated checkpoint and earlier delayed-DOWN repair, not a current mechanical edit. Prior enforced verdict retained. No tests, timers, mutations or gates were executed by this judge; no new source defect found.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L105) | unit/verify | not verified |
| negative | [`TestGRStateManagerRestartExpiryAfterReconnect`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L428) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`TestGRStateManagerRestartExpiryRejectsOldCycle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L379) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L158) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestGRStateManagerTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L67) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L139) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L115) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC4724-4.2-8`](#rfc4724-4.2-8)

Once the session is re-established, if the "Forwarding State" bit for a specific address family is not set in the newly received Graceful Restart Capability, or if a specific address family is not included in the newly received Graceful Restart Capability, or if the Graceful Restart Capability is not received in the re-established session at all, then the Receiving Speaker MUST immediately remove all the stale routes from the peer that it is retaining for that address family. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerReconnectFBitZero, internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerReconnectWithFBit, internal/component/bgp/plugins/gr/rfc4724_capability_absent_test.go::TestRFC4724ReestablishedWithoutCapabilityPurgesOnTheStructuredPath, internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go::TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes, internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go::TestRFC4724ReestablishedWithoutForwardingStatePurgesTheFamily moved. Re-judged 2026-10-02 (BGP c25 judge) after the D-8 fix. Producer read: gr.go forgetGRCapability (deletes peerCaps and peerLLGRCaps, RFC 4724 4.2 and RFC 9494 4.5 quoted) is called from handleOpenEvent and handleStructuredOpen only when the received OPEN has no code 64; an OPEN carrying 64 still stores the new capability, so GR state is not dropped. Opens are subscribed 'direction received' and the server filters by direction (events.go PeerScopedProcs), so Ze's own OPEN never reaches it. peerCaps is read only by gr.go's two state handlers. All three triggers are now proven at the production dispatch: F clear and family omitted (OPEN through handleEvent) purge exactly that family; capability absent purges both on the JSON path (OPEN with only cap 65) and the structured path (only cap 65, no optional parameters) through a real down/up cycle. Negative: F set for both purges nothing. Judge overlay probe removing the peerCaps delete turned exactly the three absent-capability subtests red. Records: forgetGRCapability revert (both positives), onSessionReestablished revert (negative). Not proven here: the JSON path's test hex omits the 2-byte capability header that format.JSONEncoder.Open's fallback includes for code 64; production runs the structured path.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L102) | unit/verify | not verified |
| negative | [`TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L233) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestGRStateManagerReconnectFBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L143) | unit/verify | not verified |
| positive | [`TestRFC4724ReestablishedWithoutCapabilityPurgesOnTheStructuredPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_capability_absent_test.go#L32) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC4724ReestablishedWithoutForwardingStatePurgesTheFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L186) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC4724-4.2-9`](#rfc4724-4.2-9)

The Receiving Speaker MUST send the End-of-RIB marker once it completes the initial update for an address family (including the case that it has no routes to send) to the peer. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/peer_initial_sync_test.go::TestInitialSyncEORReachesTheSilentFamilyToo moved. Re-judged 2026-11-14 by an independent auditor who authored neither 08fe903803 nor the uncommitted phase 2 of spec-single-owner-lifecycle-2-bgp-session. RFC 4724 Section 4.2 read: "The Receiving Speaker MUST send the End-of-RIB marker once it completes the initial update for an address family (including the case that it has no routes to send) to the peer." Units read: TestInitialSyncEORReachesTheSilentFamilyToo, TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes; only their sync calls changed. 08fe903803 moved the initial-sync gate (sendingInitialRoutes) and the End-of-RIB owed mark (initialSyncEOROwed) from Peer onto Session. In the tagged units named stale it replaced peer.sendInitialRoutes() with peer.runInitialSyncNow(), which is Peer.sendInitialRoutes(p.testInitialSync()) (initial_sync_helpers_test.go): the same production sync body, run on the session the fixture installed (or one built with NewSession(p.settings) and published through establishedSession as enterEstablished does); it replaced peer.<flag>.Store(v) with peer.testInitialSync().<flag>.Store(v) or setTestInitialSync, and drainAndCloseQueueGate now takes that session. git diff 08fe903803^ against the working tree shows no assertion, expected byte, driven input or tag prose changed in any of them, each under the RFC-approved line 08fe903803 carries. Questions 1, 3 and 4 answer exactly as the earlier judgement below; question 2 still rests on the same producers, whose discrimination records are retaken by ./le rfc record after this stamp. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-10-12 (independent auditor). SHIFTED by 4e731b2d96 (owner-approved lint-only reactor test edits). The tagged unit is byte-identical (gate-verified); the file-level changes are comment punctuation, slices.Concat over the same bytes, joined appends, strings.SplitSeq, newDefaultOriginatePeer taking its fixed addresses from constants with the same values, and registerTypeLenRecognizer losing a lo every caller passed as 1, none of which changes what a helper this unit calls produces. Verdict stands. Positive TestInitialSyncEORReachesTheSilentFamilyToo proves the marker is sent including for a family with no routes. Negative TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes: no End-of-RIB before the initial update for IPv4 unicast completes (plugin barrier held, sendInitialRoutes observed waiting, wire empty), then exactly that marker once it does. TestBuildEOR_IPv4Unicast proves encoding only. Rejudged 2026-10-12 after commit ce62c78c3b moved peer_initial_sync_test.go around these units: the tagged functions there were re-read and are byte-identical, their assertions and fixtures still carry the evidence this note cites, and the other cited units did not change, so the verdict stands. Rejudged 2026-10-12 after 7a637d6df5 renamed SendUpdateHeld to sendUpdateHeld in two comments of peer_initial_sync_test.go: every tagged unit cited here is byte-identical (gate-verified), the comment edits sit outside them, and the assertions and fixtures still carry the evidence this note cites, so the verdict stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L229) | unit/verify | not verified |
| negative | [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L22) | unit/verify | not verified |
| positive | [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L116) | unit/verify | not verified |

### [`RFC4724-4.2-10`](#rfc4724-4.2-10)

The Receiving Speaker MUST replace the stale routes by the routing updates received from the peer. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Insert of a stale prefix clears StaleLevel to fresh with the new attributes; the negative keeps an unrefreshed stale prefix stale while a new prefix is fresh

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestFamilyRIB_InsertNewDuringStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L179) | unit/verify | not verified |
| positive | [`TestFamilyRIB_InsertClearsStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L147) | unit/verify | not verified |

### [`RFC4724-4.2-11`](#rfc4724-4.2-11)

Once the End-of-RIB marker for an address family is received from the peer, it MUST immediately remove any routes from the peer that are still marked as stale for that address family. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerEORForNonGRPeer, internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerEORPurge, internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go::TestRFC4724EndOfRIBLeavesOtherFamiliesStale, internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go::TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes moved. Re-judged 2026-10-02 (BGP c24 judge). The production removal is now asserted: TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes (production callbacks, session-down sequence consumed) sends an IPv4 unicast End-of-RIB and asserts exactly one dispatch, purge-stale <peer> ipv4/unicast, before handleEOREvent returns; TestRFC4724EndOfRIBLeavesOtherFamiliesStale asserts no ipv6 or peer-wide purge and nothing for an End-of-RIB of a family never retained. Judge overlay probe (handleEOREvent purging regardless of onEORReceived) turned only the negative red; the author's probe (purge-stale without the family) turned both red. Rib side: TestRIBPurgeStaleFamilyCommand. HEAD gr_state_test units stay supplementary.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestGRStateManagerEORForNonGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L314) | unit/verify | not verified |
| negative | [`TestRFC4724EndOfRIBLeavesOtherFamiliesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L112) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestGRStateManagerEORPurge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L185) | unit/verify | not verified |
| positive | [`TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L90) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC4724-4-4`](#rfc4724-4-4)

A BGP speaker MAY advertise the Graceful Restart Capability for an address family to its peer if it has the ability to preserve its forwarding state for the address family when BGP restarts. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c29 judge), owner rulings 7 and 8(a). parseGRCapValue now lists a family only when nlrisplit.Supported (the RIB plugin, a hard Dependency of bgp-gr resolved by ResolveDependencies at autoload, can store and re-send it). Negative TestRFC4724GRCapabilityOmitsAFamilyNoRIBStores registers ipv4/rib-less (SAFI 241, no splitter, preconditions asserted) and pins 007800010100 for the session list and 0078 (capability still sent, no tuple) for a configured list naming only it, each with the Warn carrying peer and family; HEAD negative ClaimsNoFamilyItDoesNotCarry keeps the uncarried-family refusal. Positives: TestRFC4724GRCapabilityKeepsEveryFamilyTheRIBStores (ipv4/flow + ipv4/unicast, 00780001850000010100, no Warn) and ListsTheFamiliesOfTheSession (exact two-tuple payload). Four observed-red records (revert parseGRCapValue); author probe inverting the condition reds all four, and the pre-fix run reds only the negative, so the pair discriminates the condition itself.

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
