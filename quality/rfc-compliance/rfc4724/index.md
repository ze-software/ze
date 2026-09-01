# RFC 4724 - Graceful Restart Mechanism for BGP

Partial. Every requirement this repository extracted from RFC 4724, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 64.0% | 16 of 25 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 4.0% | 1 of 25 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 25 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 43 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 26 | of 31 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 26 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 32.0% | 8 of 25 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 25 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 31 |
| Gated MUST-level | 26 |
| Obligations that bind Ze | 25 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 8 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 43 |
| Tagged units | 43 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4724.md` |
| Requirement shard | `rfc/requirements/rfc4724.md` |
| RFC text | `rfc/full/rfc4724.txt` |

## Enrolment

Enrolled: Graceful Restart Mechanism for BGP (RFC 4724): 16 MET (GR capability advertise + last-instance negotiate, Restart-State/Forwarding-State bit encoding, End-of-RIB send + detect, mark-stale on non-NOTIFICATION drop, stale deletion on consecutive restart / Restart-Time expiry / F-bit-clear / no-GR-cap re-establish, GR-stale level-1 competes normally in best-path) + 1 single-polarity positive + 8 gap (GR-capability collision-detection override and related receiving-speaker obligations) + 1 not-applicable

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Receiving-Speaker Graceful Restart: GR capability advertise and last-instance negotiate, Restart-State/Forwarding-State bit encoding, End-of-RIB send and detect, mark-stale on a non-NOTIFICATION drop, stale deletion on consecutive restart / Restart-Time expiry / F-bit-clear, and GR-stale routes competing normally in best-path (internal/component/bgp/plugins/gr, internal/component/bgp/plugins/rib). The send covers a session where neither speaker advertised a Multiprotocol capability. RFC 4271 carries that session as IPv4 unicast, so RFC 4724 Section 4 owes it a marker. `Negotiate` ([`internal/core/bgp/capability/negotiated.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated.go)) reads a side that advertised none as advertising ipv4/unicast. `sendInitialRoutes` ([`internal/component/bgp/reactor/peer_initial_sync.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync.go)) then sends the marker. FRR 10.3.1 decodes it in test/interop/scenarios/no-family-peer-eor-frr. The marker also waits for the plugins that push routes into the session: `setState` marks it owed, `sendInitialRoutes` closes the queueing gate and only then waits for every binding `ProcessBinding.MayPushRoutes` counts, by the `send [ update ]` rail or the `send [ raw ]` one ([`internal/component/bgp/reactor/peer_initial_sync.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync.go), peer_settings.go). [`test/plugin/initial-sync-barrier-raw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/initial-sync-barrier-raw.ci) asserts the injected route and the marker byte for byte, in that order, and goes red when the SendRaw arm of that predicate is removed. Requirements bound per line in [`rfc/short/rfc4724.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4724.md).

**What the ledger says remains**

Eight MUST gaps annotated in [`rfc/short/rfc4724.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4724.md): ze implements the Restarting-Speaker path as GR signaling only and does not retain its own Loc-RIB across a restart, so it runs no selection-deferral cycle and exposes no Selection_Deferral_Timer ([`RFC4724-4.1-1`](#rfc4724-4.1-1), 4.1-2, 4.1-3, 4.1-5, 4.1-6, 4.1-8); and on a re-established GR-capable session it follows plain RFC 4271 Section 6.8 collision detection (closes the new connection, keeps the existing session) rather than the RFC 4724 Section 4.2 override that treats the new OPEN as terminating the old session ([`RFC4724-4.2-1`](#rfc4724-4.2-1), 4.2-2).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 16 | one part of the gated population |
| Annotated instead of tested | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **26** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (16):** [`RFC4724-3-2`](#rfc4724-3-2), [`RFC4724-3-3`](#rfc4724-3-3), [`RFC4724-3-4`](#rfc4724-3-4), [`RFC4724-4-1`](#rfc4724-4-1), [`RFC4724-4-2`](#rfc4724-4-2), [`RFC4724-4.1-4`](#rfc4724-4.1-4), [`RFC4724-4.1-7`](#rfc4724-4.1-7), [`RFC4724-4.2-3`](#rfc4724-4.2-3), [`RFC4724-4.2-4`](#rfc4724-4.2-4), [`RFC4724-4.2-5`](#rfc4724-4.2-5), [`RFC4724-4.2-6`](#rfc4724-4.2-6), [`RFC4724-4.2-7`](#rfc4724-4.2-7), [`RFC4724-4.2-8`](#rfc4724-4.2-8), [`RFC4724-4.2-9`](#rfc4724-4.2-9), [`RFC4724-4.2-10`](#rfc4724-4.2-10), [`RFC4724-4.2-11`](#rfc4724-4.2-11)

**Annotated instead of tested (10):** [`RFC4724-3-1`](#rfc4724-3-1), [`RFC4724-3-5`](#rfc4724-3-5), [`RFC4724-4.1-1`](#rfc4724-4.1-1), [`RFC4724-4.1-2`](#rfc4724-4.1-2), [`RFC4724-4.1-3`](#rfc4724-4.1-3), [`RFC4724-4.1-5`](#rfc4724-4.1-5), [`RFC4724-4.1-6`](#rfc4724-4.1-6), [`RFC4724-4.1-8`](#rfc4724-4.1-8), [`RFC4724-4.2-1`](#rfc4724-4.2-1), [`RFC4724-4.2-2`](#rfc4724-4.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4724-3-1` | A BGP speaker MUST NOT include more than one instance of the Graceful Restart Capability in the capability advertisement (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestExtractGRCapabilities_CapabilityDecl`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L111). **negative:** no negative test. **{single-polarity}:** ze's GR sender emits exactly one code-64 declaration per peer (internal/component/bgp/plugins/gr/gr.go:703 extractGRCapabilities appends one CapabilityDecl per peer) and the encoder writes a single TLV (internal/core/bgp/capability/capability.go:553 WriteTo); there is no code path that emits two instances, so the more-than-one case cannot be constructed to test negatively |
| `RFC4724-3-2` | Reserved bits in Restart Flags MUST be set to zero by the sender (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L422). **negative:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L433) |
| `RFC4724-3-3` | Reserved bits in Address Family Flags MUST be set to zero by the sender (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L428). **negative:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L442) |
| `RFC4724-3-4` | If more than one instance of the Graceful Restart Capability is received, the receiver MUST ignore all but the last instance (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L62). **negative:** `unit/verify` [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L70) |
| `RFC4724-3-5` | When R bit is set, peer MUST NOT wait for End-of-RIB marker from the speaker before advertising routing information (Section 3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze advertises its Adj-RIB-Out and per-family End-of-RIB immediately on reaching Established (internal/component/bgp/reactor/peer_initial_sync.go:277,334) and has no receive-side mechanism that gates advertisement on a peer's End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and never defers advertisement, so there is no wait state for the R bit to override |
| `RFC4724-4-1` | The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update for an address family (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L16). **positive:** `unit/verify` [`TestInitialSyncBarrierCreditsOnlyTheProcessesItNames`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L760). **positive:** `unit/verify` [`TestInitialSyncClosesTheQueueGateBeforeItWaitsForRoutePushingPlugins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L342). **positive:** `unit/verify` [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L100). **positive:** `unit/verify` [`TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L726). **positive:** `unit/verify` [`TestRoutePushingBindingsCountBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L467). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L224). **positive:** `interop/nightly` [`checkNoFamilyEndOfRIB`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1196) |
| `RFC4724-4-2` | Normal BGP procedures MUST be followed when the TCP session terminates due to sending or receiving a NOTIFICATION message (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestGRStateManagerNotificationBypass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L220). **negative:** `unit/verify` [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L44) |
| `RFC4724-4.1-1` | Restarting Speaker MUST retain, if possible, the forwarding state for BGP routes in Loc-RIB (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's Restarting Speaker path implements only GR signaling -- it writes a restart marker and sets the R bit (internal/component/bgp/grmarker/grmarker.go, internal/component/bgp/reactor/peer.go:574) -- and does not retain its own in-memory Loc-RIB forwarding state across a process restart within the bgp packages; the Loc-RIB is rebuilt from scratch on restart |
| `RFC4724-4.1-2` | Restarting Speaker MUST mark retained forwarding state as stale (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** because ze does not retain its own Loc-RIB across a restart (see RFC4724-4.1-1), there is no retained own-forwarding state to mark stale; the stale-marking machinery (internal/component/bgp/plugins/rib/rib_commands.go:817 markStaleCommand) applies to routes received from a restarting peer, not to ze's own routes on ze's restart |
| `RFC4724-4.1-3` | Restarting Speaker MUST NOT differentiate between stale and other information during forwarding (Section 4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this governs forwarding over ze's own retained stale Loc-RIB, which ze does not build on restart (see RFC4724-4.1-1); the generic non-differentiation of level-1 stale in best-path selection (internal/component/bgp/plugins/rib/bestpath.go:308) is the Receiving Speaker path for peer routes, not ze's own routes as a Restarting Speaker |
| `RFC4724-4.1-4` | Restarting Speaker MUST set the "Restart State" (R) bit in the Graceful Restart Capability of the OPEN message (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L259). **negative:** `unit/verify` [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L498) |
| `RFC4724-4.1-5` | Restarting Speaker MUST defer route selection per address family until End-of-RIB from all peers or Selection_Deferral_Timer expires (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs best-path selection as updates arrive and has no selection-deferral path keyed on End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and does not gate route selection |
| `RFC4724-4.1-6` | After route selection, forwarding state MUST be updated and stale information MUST be removed (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this is the completion step of the deferred-selection cycle ze does not run (see RFC4724-4.1-5); with no own-Loc-RIB stale state (RFC4724-4.1-1) there is no post-selection stale removal on ze's own restart |
| `RFC4724-4.1-7` | Once initial update is complete, the End-of-RIB marker MUST be sent (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L20). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L228) |
| `RFC4724-4.1-8` | An implementation MUST support a configurable Selection_Deferral_Timer (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze exposes no Selection_Deferral_Timer configuration -- the GR YANG model (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang) carries restart-time and long-lived-stale-time only, and no code defers selection (see RFC4724-4.1-5) |
| `RFC4724-4.2-1` | Receiving Speaker MUST treat subsequent open connection from peer as termination of old TCP session when GR Capability was received (Section 4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze follows plain RFC 4271 Section 6.8 collision detection -- a new inbound connection while the session is Established is rejected with Cease/Connection Collision (internal/component/bgp/reactor/reactor_connection.go:134-136), with no GR-capability branch that treats the new OPEN as terminating the old session |
| `RFC4724-4.2-2` | Previous TCP session MUST be closed, and the new one retained (Section 4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same RFC 4271 collision path closes the NEW connection and keeps the existing Established session (internal/component/bgp/reactor/reactor_connection.go:134-136 rejectConnectionCollisionWithSettings), the opposite of the RFC 4724 Section 4.2 override that closes the previous session and retains the new one |
| `RFC4724-4.2-3` | Receiving Speaker MUST retain routes from peer for all address families previously in Graceful Restart Capability and MUST mark them as stale (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L40). **positive:** `unit/verify` [`TestRFC4724RetentionCoversEveryAdvertisedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L124). **positive:** `unit/verify` [`TestRFC4724SessionDownRetainsAndMarksRoutesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L73). **negative:** `unit/verify` [`TestGRStateManagerNoGRCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L297). **negative:** `unit/verify` [`TestRFC4724SessionDownWithoutCapabilityRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L96) |
| `RFC4724-4.2-4` | On consecutive restarts, previously stale routes from the peer MUST be deleted (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L272). **negative:** `unit/verify` [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L286) |
| `RFC4724-4.2-5` | Receiving Speaker MUST NOT differentiate between stale and other routing information during forwarding (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L840). **negative:** `unit/verify` [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L808) |
| `RFC4724-4.2-6` | The "Restart State" bit in the Receiving Speaker's OPEN MUST NOT be set unless the Receiving Speaker has itself restarted (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L495). **negative:** `unit/verify` [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L262) |
| `RFC4724-4.2-7` | If session not re-established within Restart Time, Receiving Speaker MUST delete all stale routes from the peer (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L65). **negative:** `unit/verify` [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L105) |
| `RFC4724-4.2-8` | If F bit not set, or AFI/SAFI missing, or no GR capability on re-establishment, Receiving Speaker MUST immediately remove all stale routes for that address family (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerReconnectFBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L143). **negative:** `unit/verify` [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L102) |
| `RFC4724-4.2-9` | Receiving Speaker MUST send End-of-RIB marker once it completes the initial update (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L22). **positive:** `unit/verify` [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L105). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L229) |
| `RFC4724-4.2-10` | Receiving Speaker MUST replace stale routes by routing updates received from the peer (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFamilyRIB_InsertClearsStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/stale_test.go#L147). **negative:** `unit/verify` [`TestFamilyRIB_InsertNewDuringStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/stale_test.go#L179) |
| `RFC4724-4.2-11` | Once End-of-RIB is received from peer, Receiving Speaker MUST immediately remove any routes still marked as stale for that address family (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerEORPurge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L185). **negative:** `unit/verify` [`TestGRStateManagerEORForNonGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L314) |
| `RFC4724-2-1` | Sending End-of-RIB upon completion of initial update is recommended even without GR (Section 2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4-3` | Advertising Graceful Restart Capability even without forwarding preservation ability is recommended (Section 4) | RECOMMENDED | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4-4` | A BGP speaker MAY advertise the Graceful Restart Capability for an address family if it can preserve forwarding state (Section 4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4.2-12` | Receiving Speaker MAY delete all stale routes if peer's forwarding state is determined non-viable (e.g., via BFD) (Section 4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4.2-13` | An implementation MAY support a configurable stale route retention timer (Section 4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4724-3-5`](#rfc4724-3-5) When R bit is set, peer MUST NOT wait for End-of-RIB marker from the speaker before advertising routing information (Section 3) | no test | no test carries this requirement id; annotated {not-applicable}: ze advertises its Adj-RIB-Out and per-family End-of-RIB immediately on reaching Established (internal/component/bgp/reactor/peer_initial_sync.go:277,334) and has no receive-side mechanism that gates advertisement on a peer's End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and never defers advertisement, so there is no wait state for the R bit to override |
| [`RFC4724-4.1-1`](#rfc4724-4.1-1) Restarting Speaker MUST retain, if possible, the forwarding state for BGP routes in Loc-RIB (Section 4.1) | {gap}, no test | ze's Restarting Speaker path implements only GR signaling -- it writes a restart marker and sets the R bit (internal/component/bgp/grmarker/grmarker.go, internal/component/bgp/reactor/peer.go:574) -- and does not retain its own in-memory Loc-RIB forwarding state across a process restart within the bgp packages; the Loc-RIB is rebuilt from scratch on restart |
| [`RFC4724-4.1-2`](#rfc4724-4.1-2) Restarting Speaker MUST mark retained forwarding state as stale (Section 4.1) | {gap}, no test | because ze does not retain its own Loc-RIB across a restart (see RFC4724-4.1-1), there is no retained own-forwarding state to mark stale; the stale-marking machinery (internal/component/bgp/plugins/rib/rib_commands.go:817 markStaleCommand) applies to routes received from a restarting peer, not to ze's own routes on ze's restart |
| [`RFC4724-4.1-3`](#rfc4724-4.1-3) Restarting Speaker MUST NOT differentiate between stale and other information during forwarding (Section 4.1) | {gap}, no test | this governs forwarding over ze's own retained stale Loc-RIB, which ze does not build on restart (see RFC4724-4.1-1); the generic non-differentiation of level-1 stale in best-path selection (internal/component/bgp/plugins/rib/bestpath.go:308) is the Receiving Speaker path for peer routes, not ze's own routes as a Restarting Speaker |
| [`RFC4724-4.1-5`](#rfc4724-4.1-5) Restarting Speaker MUST defer route selection per address family until End-of-RIB from all peers or Selection_Deferral_Timer expires (Section 4.1) | {gap}, no test | ze runs best-path selection as updates arrive and has no selection-deferral path keyed on End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and does not gate route selection |
| [`RFC4724-4.1-6`](#rfc4724-4.1-6) After route selection, forwarding state MUST be updated and stale information MUST be removed (Section 4.1) | {gap}, no test | this is the completion step of the deferred-selection cycle ze does not run (see RFC4724-4.1-5); with no own-Loc-RIB stale state (RFC4724-4.1-1) there is no post-selection stale removal on ze's own restart |
| [`RFC4724-4.1-8`](#rfc4724-4.1-8) An implementation MUST support a configurable Selection_Deferral_Timer (Section 4.1) | {gap}, no test | ze exposes no Selection_Deferral_Timer configuration -- the GR YANG model (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang) carries restart-time and long-lived-stale-time only, and no code defers selection (see RFC4724-4.1-5) |
| [`RFC4724-4.2-1`](#rfc4724-4.2-1) Receiving Speaker MUST treat subsequent open connection from peer as termination of old TCP session when GR Capability was received (Section 4.2) | {gap}, no test | ze follows plain RFC 4271 Section 6.8 collision detection -- a new inbound connection while the session is Established is rejected with Cease/Connection Collision (internal/component/bgp/reactor/reactor_connection.go:134-136), with no GR-capability branch that treats the new OPEN as terminating the old session |
| [`RFC4724-4.2-2`](#rfc4724-4.2-2) Previous TCP session MUST be closed, and the new one retained (Section 4.2) | {gap}, no test | the same RFC 4271 collision path closes the NEW connection and keeps the existing Established session (internal/component/bgp/reactor/reactor_connection.go:134-136 rejectConnectionCollisionWithSettings), the opposite of the RFC 4724 Section 4.2 override that closes the previous session and retains the new one |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4724-3-1`](#rfc4724-3-1)

A BGP speaker MUST NOT include more than one instance of the Graceful Restart Capability in the capability advertisement (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtractGRCapabilities_CapabilityDecl`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L111) | unit/verify | unproven |

### [`RFC4724-3-2`](#rfc4724-3-2)

Reserved bits in Restart Flags MUST be set to zero by the sender (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L433) | unit/verify | unproven |
| positive | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L422) | unit/verify | unproven |

### [`RFC4724-3-3`](#rfc4724-3-3)

Reserved bits in Address Family Flags MUST be set to zero by the sender (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L442) | unit/verify | unproven |
| positive | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L428) | unit/verify | unproven |

### [`RFC4724-3-4`](#rfc4724-3-4)

If more than one instance of the Graceful Restart Capability is received, the receiver MUST ignore all but the last instance (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L70) | unit/verify | unproven |
| positive | [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L62) | unit/verify | unproven |

### [`RFC4724-3-5`](#rfc4724-3-5)

When R bit is set, peer MUST NOT wait for End-of-RIB marker from the speaker before advertising routing information (Section 3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-3-5, so no unit is bound to it.

### [`RFC4724-4-1`](#rfc4724-4-1)

The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update for an address family (Section 4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L224) | unit/verify | unproven |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L16) | unit/verify | unproven |
| positive | [`TestInitialSyncBarrierCreditsOnlyTheProcessesItNames`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L760) | unit/verify | unproven |
| positive | [`TestInitialSyncClosesTheQueueGateBeforeItWaitsForRoutePushingPlugins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L342) | unit/verify | unproven |
| positive | [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L100) | unit/verify | unproven |
| positive | [`TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L726) | unit/verify | unproven |
| positive | [`TestRoutePushingBindingsCountBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L467) | unit/verify | unproven |
| positive | [`checkNoFamilyEndOfRIB`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1196) | interop/nightly | unproven |

### [`RFC4724-4-2`](#rfc4724-4-2)

Normal BGP procedures MUST be followed when the TCP session terminates due to sending or receiving a NOTIFICATION message (Section 4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L44) | unit/verify | unproven |
| positive | [`TestGRStateManagerNotificationBypass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L220) | unit/verify | unproven |

### [`RFC4724-4.1-1`](#rfc4724-4.1-1)

Restarting Speaker MUST retain, if possible, the forwarding state for BGP routes in Loc-RIB (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-1, so no unit is bound to it.

### [`RFC4724-4.1-2`](#rfc4724-4.1-2)

Restarting Speaker MUST mark retained forwarding state as stale (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-2, so no unit is bound to it.

### [`RFC4724-4.1-3`](#rfc4724-4.1-3)

Restarting Speaker MUST NOT differentiate between stale and other information during forwarding (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-3, so no unit is bound to it.

### [`RFC4724-4.1-4`](#rfc4724-4.1-4)

Restarting Speaker MUST set the "Restart State" (R) bit in the Graceful Restart Capability of the OPEN message (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L498) | unit/verify | unproven |
| positive | [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L259) | unit/verify | unproven |

### [`RFC4724-4.1-5`](#rfc4724-4.1-5)

Restarting Speaker MUST defer route selection per address family until End-of-RIB from all peers or Selection_Deferral_Timer expires (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-5, so no unit is bound to it.

### [`RFC4724-4.1-6`](#rfc4724-4.1-6)

After route selection, forwarding state MUST be updated and stale information MUST be removed (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-6, so no unit is bound to it.

### [`RFC4724-4.1-7`](#rfc4724-4.1-7)

Once initial update is complete, the End-of-RIB marker MUST be sent (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L228) | unit/verify | unproven |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L20) | unit/verify | unproven |

### [`RFC4724-4.1-8`](#rfc4724-4.1-8)

An implementation MUST support a configurable Selection_Deferral_Timer (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-8, so no unit is bound to it.

### [`RFC4724-4.2-1`](#rfc4724-4.2-1)

Receiving Speaker MUST treat subsequent open connection from peer as termination of old TCP session when GR Capability was received (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.2-1, so no unit is bound to it.

### [`RFC4724-4.2-2`](#rfc4724-4.2-2)

Previous TCP session MUST be closed, and the new one retained (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.2-2, so no unit is bound to it.

### [`RFC4724-4.2-3`](#rfc4724-4.2-3)

Receiving Speaker MUST retain routes from peer for all address families previously in Graceful Restart Capability and MUST mark them as stale (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerNoGRCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L297) | unit/verify | unproven |
| negative | [`TestRFC4724SessionDownWithoutCapabilityRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L96) | unit/verify | unproven |
| positive | [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L40) | unit/verify | unproven |
| positive | [`TestRFC4724RetentionCoversEveryAdvertisedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L124) | unit/verify | unproven |
| positive | [`TestRFC4724SessionDownRetainsAndMarksRoutesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L73) | unit/verify | unproven |

### [`RFC4724-4.2-4`](#rfc4724-4.2-4)

On consecutive restarts, previously stale routes from the peer MUST be deleted (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L286) | unit/verify | unproven |
| positive | [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L272) | unit/verify | unproven |

### [`RFC4724-4.2-5`](#rfc4724-4.2-5)

Receiving Speaker MUST NOT differentiate between stale and other routing information during forwarding (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L808) | unit/verify | unproven |
| positive | [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L840) | unit/verify | unproven |

### [`RFC4724-4.2-6`](#rfc4724-4.2-6)

The "Restart State" bit in the Receiving Speaker's OPEN MUST NOT be set unless the Receiving Speaker has itself restarted (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L262) | unit/verify | unproven |
| positive | [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L495) | unit/verify | unproven |

### [`RFC4724-4.2-7`](#rfc4724-4.2-7)

If session not re-established within Restart Time, Receiving Speaker MUST delete all stale routes from the peer (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L105) | unit/verify | unproven |
| positive | [`TestGRStateManagerTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L65) | unit/verify | unproven |

### [`RFC4724-4.2-8`](#rfc4724-4.2-8)

If F bit not set, or AFI/SAFI missing, or no GR capability on re-establishment, Receiving Speaker MUST immediately remove all stale routes for that address family (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L102) | unit/verify | unproven |
| positive | [`TestGRStateManagerReconnectFBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L143) | unit/verify | unproven |

### [`RFC4724-4.2-9`](#rfc4724-4.2-9)

Receiving Speaker MUST send End-of-RIB marker once it completes the initial update (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L229) | unit/verify | unproven |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L22) | unit/verify | unproven |
| positive | [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L105) | unit/verify | unproven |

### [`RFC4724-4.2-10`](#rfc4724-4.2-10)

Receiving Speaker MUST replace stale routes by routing updates received from the peer (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFamilyRIB_InsertNewDuringStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/stale_test.go#L179) | unit/verify | unproven |
| positive | [`TestFamilyRIB_InsertClearsStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/stale_test.go#L147) | unit/verify | unproven |

### [`RFC4724-4.2-11`](#rfc4724-4.2-11)

Once End-of-RIB is received from peer, Receiving Speaker MUST immediately remove any routes still marked as stale for that address family (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerEORForNonGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L314) | unit/verify | unproven |
| positive | [`TestGRStateManagerEORPurge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L185) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 4724, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4724, so its obligations are stated where they were written.
