# RFC 4724 - Graceful Restart Mechanism for BGP

Partial. Every requirement this repository extracted from RFC 4724, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 61.5% | 16 of 26 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3.8% | 1 of 26 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 26 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 4.4% | 2 of 45 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 26 | of 31 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 26 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 3.8% | 1 of 26 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 26 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 26 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 30.8% | 8 of 26 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 17 | of 26 gated MUSTs judged | 14 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 26 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 31 |
| Gated MUST-level | 26 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 8 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 45 |
| Tagged units | 45 |
| Recorded audit verdicts | 17 |
| Discrimination records | 2 |
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
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **26** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (16):** [`RFC4724-3-2`](#rfc4724-3-2), [`RFC4724-3-3`](#rfc4724-3-3), [`RFC4724-3-4`](#rfc4724-3-4), [`RFC4724-4-1`](#rfc4724-4-1), [`RFC4724-4-2`](#rfc4724-4-2), [`RFC4724-4.1-4`](#rfc4724-4.1-4), [`RFC4724-4.1-7`](#rfc4724-4.1-7), [`RFC4724-4.2-3`](#rfc4724-4.2-3), [`RFC4724-4.2-4`](#rfc4724-4.2-4), [`RFC4724-4.2-5`](#rfc4724-4.2-5), [`RFC4724-4.2-6`](#rfc4724-4.2-6), [`RFC4724-4.2-7`](#rfc4724-4.2-7), [`RFC4724-4.2-8`](#rfc4724-4.2-8), [`RFC4724-4.2-9`](#rfc4724-4.2-9), [`RFC4724-4.2-10`](#rfc4724-4.2-10), [`RFC4724-4.2-11`](#rfc4724-4.2-11)

**Annotated instead of tested (10):** [`RFC4724-3-1`](#rfc4724-3-1), [`RFC4724-3-5`](#rfc4724-3-5), [`RFC4724-4.1-1`](#rfc4724-4.1-1), [`RFC4724-4.1-2`](#rfc4724-4.1-2), [`RFC4724-4.1-3`](#rfc4724-4.1-3), [`RFC4724-4.1-5`](#rfc4724-4.1-5), [`RFC4724-4.1-6`](#rfc4724-4.1-6), [`RFC4724-4.1-8`](#rfc4724-4.1-8), [`RFC4724-4.2-1`](#rfc4724-4.2-1), [`RFC4724-4.2-2`](#rfc4724-4.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4724-3-1` | A BGP speaker MUST NOT include more than one instance of the Graceful Restart Capability in the capability advertisement (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestExtractGRCapabilities_CapabilityDecl`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L111). **negative:** no negative test. **{single-polarity}:** ze's GR sender emits exactly one code-64 declaration per peer (internal/component/bgp/plugins/gr/gr.go:703 extractGRCapabilities appends one CapabilityDecl per peer) and the encoder writes a single TLV (internal/core/bgp/capability/capability.go:553 WriteTo); there is no code path that emits two instances, so the more-than-one case cannot be constructed to test negatively |
| `RFC4724-3-2` | The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L457). **negative:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L468) |
| `RFC4724-3-3` | The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L463). **negative:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L477) |
| `RFC4724-3-4` | If more than one instance of the Graceful Restart Capability is carried in the capability advertisement, the receiver of the advertisement MUST ignore all but the last instance of the Graceful Restart Capability. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L62). **negative:** `unit/verify` [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L70) |
| `RFC4724-3-5` | When set (value 1), this bit indicates that the BGP speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker from the speaker before advertising routing information to the speaker. (Section 3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze advertises its Adj-RIB-Out and per-family End-of-RIB immediately on reaching Established (internal/component/bgp/reactor/peer_initial_sync.go:277,334) and has no receive-side mechanism that gates advertisement on a peer's End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and never defers advertisement, so there is no wait state for the R bit to override |
| `RFC4724-4-1` | The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L16). **positive:** `unit/verify` [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L99). **positive:** `unit/verify` [`TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L678). **positive:** `unit/verify` [`TestInitialSyncMarkerWaitsForNoProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L711). **positive:** `unit/verify` [`TestInitialSyncShutsTheQueueGateAndFreesTheRailsWithTheMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L341). **positive:** `unit/verify` [`TestRoutePushingBindingsCountBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L419). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L224). **positive:** `interop/nightly` [`checkNoFamilyEndOfRIB`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1194) |
| `RFC4724-4-2` | It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestGRStateManagerNotificationBypass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L220). **negative:** `unit/verify` [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L44) |
| `RFC4724-4.1-1` | When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's Restarting Speaker path implements only GR signaling -- it writes a restart marker and sets the R bit (internal/component/bgp/grmarker/grmarker.go, internal/component/bgp/reactor/peer.go:574) -- and does not retain its own in-memory Loc-RIB forwarding state across a process restart within the bgp packages; the Loc-RIB is rebuilt from scratch on restart |
| `RFC4724-4.1-2` | When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB and MUST mark them as stale. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** because ze does not retain its own Loc-RIB across a restart (see RFC4724-4.1-1), there is no retained own-forwarding state to mark stale; the stale-marking machinery (internal/component/bgp/plugins/rib/rib_commands.go:817 markStaleCommand) applies to routes received from a restarting peer, not to ze's own routes on ze's restart |
| `RFC4724-4.1-3` | It MUST NOT differentiate between stale and other information during forwarding. (Section 4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this governs forwarding over ze's own retained stale Loc-RIB, which ze does not build on restart (see RFC4724-4.1-1); the generic non-differentiation of level-1 stale in best-path selection (internal/component/bgp/plugins/rib/bestpath.go:308) is the Receiving Speaker path for peer routes, not ze's own routes as a Restarting Speaker |
| `RFC4724-4.1-4` | To re-establish the session with its peer, the Restarting Speaker MUST set the "Restart State" bit in the Graceful Restart Capability of the OPEN message. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L262). **negative:** `unit/verify` [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L497) |
| `RFC4724-4.1-5` | However, it MUST defer route selection for an address family until it either (a) receives the End-of-RIB marker from all its peers (excluding the ones with the "Restart State" bit set in the received capability and excluding the ones that do not advertise the graceful restart capability) or (b) the Selection_Deferral_Timer referred to below has expired. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs best-path selection as updates arrive and has no selection-deferral path keyed on End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and does not gate route selection |
| `RFC4724-4.1-6` | After the BGP speaker performs route selection, the forwarding state of the speaker MUST be updated and any previously marked stale information MUST be removed. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this is the completion step of the deferred-selection cycle ze does not run (see RFC4724-4.1-5); with no own-Loc-RIB stale state (RFC4724-4.1-1) there is no post-selection stale removal on ze's own restart |
| `RFC4724-4.1-7` | Once the initial update is complete for an address family (including the case that there is no routing update to send), the End-of-RIB marker MUST be sent. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L20). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L228) |
| `RFC4724-4.1-8` | To put an upper bound on the amount of time a router defers its route selection, an implementation MUST support a (configurable) timer that imposes this upper bound. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze exposes no Selection_Deferral_Timer configuration -- the GR YANG model (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang) carries restart-time and long-lived-stale-time only, and no code defers selection (see RFC4724-4.1-5) |
| `RFC4724-4.2-1` | In case it does not detect the termination of the old TCP session and still considers the BGP session as being established, it MUST treat the subsequent open connection from the peer as an indication of the termination of the old TCP session and act accordingly (when the Graceful Restart Capability has been received from the peer). (Section 4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze follows plain RFC 4271 Section 6.8 collision detection -- a new inbound connection while the session is Established is rejected with Cease/Connection Collision (internal/component/bgp/reactor/reactor_connection.go:134-136), with no GR-capability branch that treats the new OPEN as terminating the old session |
| `RFC4724-4.2-2` | "Acting accordingly" in this context means that the previous TCP session MUST be closed, and the new one retained. (Section 4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same RFC 4271 collision path closes the NEW connection and keeps the existing Established session (internal/component/bgp/reactor/reactor_connection.go:134-136 rejectConnectionCollisionWithSettings), the opposite of the RFC 4724 Section 4.2 override that closes the previous session and retains the new one |
| `RFC4724-4.2-3` | When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L40). **positive:** `unit/verify` [`TestRFC4724RetentionCoversEveryAdvertisedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L124). **positive:** `unit/verify` [`TestRFC4724SessionDownRetainsAndMarksRoutesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L73). **negative:** `unit/verify` [`TestGRStateManagerNoGRCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L297). **negative:** `unit/verify` [`TestRFC4724SessionDownWithoutCapabilityRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L96) |
| `RFC4724-4.2-4` | To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L272). **negative:** `unit/verify` [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L286) |
| `RFC4724-4.2-5` | The router MUST NOT differentiate between stale and other routing information during forwarding. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L840). **negative:** `unit/verify` [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L808) |
| `RFC4724-4.2-6` | In re-establishing the session, the "Restart State" bit in the Graceful Restart Capability of the OPEN message sent by the Receiving Speaker MUST NOT be set unless the Receiving Speaker has restarted. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L494). **negative:** `unit/verify` [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L265) |
| `RFC4724-4.2-7` | If the session does not get re-established within the "Restart Time" that the peer advertised previously, the Receiving Speaker MUST delete all the stale routes from the peer that it is retaining. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L65). **negative:** `unit/verify` [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L105) |
| `RFC4724-4.2-8` | Once the session is re-established, if the "Forwarding State" bit for a specific address family is not set in the newly received Graceful Restart Capability, or if a specific address family is not included in the newly received Graceful Restart Capability, or if the Graceful Restart Capability is not received in the re-established session at all, then the Receiving Speaker MUST immediately remove all the stale routes from the peer that it is retaining for that address family. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerReconnectFBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L143). **negative:** `unit/verify` [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L102) |
| `RFC4724-4.2-9` | The Receiving Speaker MUST send the End-of-RIB marker once it completes the initial update for an address family (including the case that it has no routes to send) to the peer. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L22). **positive:** `unit/verify` [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L104). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L229) |
| `RFC4724-4.2-10` | The Receiving Speaker MUST replace the stale routes by the routing updates received from the peer. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFamilyRIB_InsertClearsStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/stale_test.go#L147). **negative:** `unit/verify` [`TestFamilyRIB_InsertNewDuringStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/stale_test.go#L179) |
| `RFC4724-4.2-11` | Once the End-of-RIB marker for an address family is received from the peer, it MUST immediately remove any routes from the peer that are still marked as stale for that address family. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerEORPurge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L185). **negative:** `unit/verify` [`TestGRStateManagerEORForNonGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L314) |
| `RFC4724-2-1` | Although the End-of-RIB marker is specified for the purpose of BGP graceful restart, it is noted that the generation of such a marker upon completion of the initial update would be useful for routing convergence in general, and thus the practice is recommended. (Section 2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4-3` | In addition, even if the speaker does not have the ability to preserve its forwarding state for any address family during BGP restart, it is still recommended that the speaker advertise the Graceful Restart Capability to its peer (Section 4) | RECOMMENDED | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4-4` | A BGP speaker MAY advertise the Graceful Restart Capability for an address family to its peer if it has the ability to preserve its forwarding state for the address family when BGP restarts. (Section 4) | MAY | 4 | **positive:** `unit/verify` [`TestRFC4724GRCapabilityListsTheFamiliesOfTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_capability_test.go#L96). **negative:** `unit/verify` [`TestRFC4724GRCapabilityClaimsNoFamilyItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_capability_test.go#L123) |
| `RFC4724-4.2-12` | In the event that it determines that its peer's forwarding state is not viable prior to the re-establishment of the session, the speaker MAY delete all the stale routes from the peer that it is retaining. (Section 4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4.2-13` | To put an upper bound on the amount of time a router retains the stale routes, an implementation MAY support a (configurable) timer that imposes this upper bound. (Section 4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4724-3-5`](#rfc4724-3-5) When set (value 1), this bit indicates that the BGP speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker from the speaker before advertising routing information to the speaker. (Section 3) | no test | no test carries this requirement id; annotated {not-applicable}: ze advertises its Adj-RIB-Out and per-family End-of-RIB immediately on reaching Established (internal/component/bgp/reactor/peer_initial_sync.go:277,334) and has no receive-side mechanism that gates advertisement on a peer's End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and never defers advertisement, so there is no wait state for the R bit to override |
| [`RFC4724-4.1-1`](#rfc4724-4.1-1) When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB (Section 4.1) | {gap}, no test | ze's Restarting Speaker path implements only GR signaling -- it writes a restart marker and sets the R bit (internal/component/bgp/grmarker/grmarker.go, internal/component/bgp/reactor/peer.go:574) -- and does not retain its own in-memory Loc-RIB forwarding state across a process restart within the bgp packages; the Loc-RIB is rebuilt from scratch on restart |
| [`RFC4724-4.1-2`](#rfc4724-4.1-2) When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB and MUST mark them as stale. (Section 4.1) | {gap}, no test | because ze does not retain its own Loc-RIB across a restart (see RFC4724-4.1-1), there is no retained own-forwarding state to mark stale; the stale-marking machinery (internal/component/bgp/plugins/rib/rib_commands.go:817 markStaleCommand) applies to routes received from a restarting peer, not to ze's own routes on ze's restart |
| [`RFC4724-4.1-3`](#rfc4724-4.1-3) It MUST NOT differentiate between stale and other information during forwarding. (Section 4.1) | {gap}, no test | this governs forwarding over ze's own retained stale Loc-RIB, which ze does not build on restart (see RFC4724-4.1-1); the generic non-differentiation of level-1 stale in best-path selection (internal/component/bgp/plugins/rib/bestpath.go:308) is the Receiving Speaker path for peer routes, not ze's own routes as a Restarting Speaker |
| [`RFC4724-4.1-5`](#rfc4724-4.1-5) However, it MUST defer route selection for an address family until it either (a) receives the End-of-RIB marker from all its peers (excluding the ones with the "Restart State" bit set in the received capability and excluding the ones that do not advertise the graceful restart capability) or (b) the Selection_Deferral_Timer referred to below has expired. (Section 4.1) | {gap}, no test | ze runs best-path selection as updates arrive and has no selection-deferral path keyed on End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and does not gate route selection |
| [`RFC4724-4.1-6`](#rfc4724-4.1-6) After the BGP speaker performs route selection, the forwarding state of the speaker MUST be updated and any previously marked stale information MUST be removed. (Section 4.1) | {gap}, no test | this is the completion step of the deferred-selection cycle ze does not run (see RFC4724-4.1-5); with no own-Loc-RIB stale state (RFC4724-4.1-1) there is no post-selection stale removal on ze's own restart |
| [`RFC4724-4.1-8`](#rfc4724-4.1-8) To put an upper bound on the amount of time a router defers its route selection, an implementation MUST support a (configurable) timer that imposes this upper bound. (Section 4.1) | {gap}, no test | ze exposes no Selection_Deferral_Timer configuration -- the GR YANG model (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang) carries restart-time and long-lived-stale-time only, and no code defers selection (see RFC4724-4.1-5) |
| [`RFC4724-4.2-1`](#rfc4724-4.2-1) In case it does not detect the termination of the old TCP session and still considers the BGP session as being established, it MUST treat the subsequent open connection from the peer as an indication of the termination of the old TCP session and act accordingly (when the Graceful Restart Capability has been received from the peer). (Section 4.2) | {gap}, no test | ze follows plain RFC 4271 Section 6.8 collision detection -- a new inbound connection while the session is Established is rejected with Cease/Connection Collision (internal/component/bgp/reactor/reactor_connection.go:134-136), with no GR-capability branch that treats the new OPEN as terminating the old session |
| [`RFC4724-4.2-2`](#rfc4724-4.2-2) "Acting accordingly" in this context means that the previous TCP session MUST be closed, and the new one retained. (Section 4.2) | {gap}, no test | the same RFC 4271 collision path closes the NEW connection and keeps the existing Established session (internal/component/bgp/reactor/reactor_connection.go:134-136 rejectConnectionCollisionWithSettings), the opposite of the RFC 4724 Section 4.2 override that closes the previous session and retains the new one |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4724-3-1`](#rfc4724-3-1)

A BGP speaker MUST NOT include more than one instance of the Graceful Restart Capability in the capability advertisement (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtractGRCapabilities_CapabilityDecl`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L111) | unit/verify | unproven |

### [`RFC4724-3-2`](#rfc4724-3-2)

The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the sentence carries two obligations: zero on send and ignored on receive. TestGracefulRestartEncodeReservedBits proves only the sender half (WriteTo masks); no tagged unit parses a Restart Flags nibble with reserved bits set and asserts they are ignored (parseGracefulRestart does ignore them)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L468) | unit/verify | unproven |
| positive | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L457) | unit/verify | unproven |

### [`RFC4724-3-3`](#rfc4724-3-3)

The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. same split as 3-2 for the Address Family Flags byte: sender zeroing is proven by TestGracefulRestartEncodeReservedBits, the receiver-ignore half is not tested by any tagged unit

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L477) | unit/verify | unproven |
| positive | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L463) | unit/verify | unproven |

### [`RFC4724-3-4`](#rfc4724-3-4)

If more than one instance of the Graceful Restart Capability is carried in the capability advertisement, the receiver of the advertisement MUST ignore all but the last instance of the Graceful Restart Capability. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. two GR instances with distinct restart time and family; Negotiate must keep the second; fails on first-wins or merge. The negative tag is the complement of the positive's assertion on the same input (two hats), not an independent case

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L70) | unit/verify | unproven |
| positive | [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L62) | unit/verify | unproven |

### [`RFC4724-3-5`](#rfc4724-3-5)

When set (value 1), this bit indicates that the BGP speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker from the speaker before advertising routing information to the speaker. (Section 3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-3-5, so no unit is bound to it.

### [`RFC4724-4-1`](#rfc4724-4-1)

The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established. (Section 4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the positives are strong (sendInitialRoutes puts one marker per negotiated family on the wire, including a silent family and a no-MP session). The only negative (TestIsEndOfRIBAnyFamily) tests the EoR detector's format discrimination, a neighbouring rule, so no tagged unit violates this requirement; TestInitialSyncEORWaitsForPeerUpBarrier's no-marker-before-barrier assertion could carry it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L224) | unit/verify | unproven |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L16) | unit/verify | unproven |
| positive | [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L99) | unit/verify | unproven |
| positive | [`TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L678) | unit/verify | unproven |
| positive | [`TestInitialSyncMarkerWaitsForNoProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L711) | unit/verify | unproven |
| positive | [`TestInitialSyncShutsTheQueueGateAndFreesTheRailsWithTheMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L341) | unit/verify | unproven |
| positive | [`TestRoutePushingBindingsCountBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L419) | unit/verify | unproven |
| positive | [`checkNoFamilyEndOfRIB`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1194) | interop/nightly | unproven |

### [`RFC4724-4-2`](#rfc4724-4-2)

It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message. (Section 4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged units call grStateManager.onSessionDown with wasNotification injected as true/false. In production handleStructuredState/handleStateEvent (gr.go) derive wasNotification from reason == "notification", and the only session-down producer (peer_run.go via notifyPeerClosed) sends "session closed" or "connection lost", so the NOTIFICATION branch is unreachable and routes are retained after a NOTIFICATION teardown. The tests stay green against that defect

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L44) | unit/verify | unproven |
| positive | [`TestGRStateManagerNotificationBypass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L220) | unit/verify | unproven |

### [`RFC4724-4.1-1`](#rfc4724-4.1-1)

When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-1, so no unit is bound to it.

### [`RFC4724-4.1-2`](#rfc4724-4.1-2)

When the Restarting Speaker restarts, it MUST retain, if possible, the forwarding state for the BGP routes in the Loc-RIB and MUST mark them as stale. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-2, so no unit is bound to it.

### [`RFC4724-4.1-3`](#rfc4724-4.1-3)

It MUST NOT differentiate between stale and other information during forwarding. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-3, so no unit is bound to it.

### [`RFC4724-4.1-4`](#rfc4724-4.1-4)

To re-establish the session with its peer, the Restarting Speaker MUST set the "Restart State" bit in the Graceful Restart Capability of the OPEN message. (Section 4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestSetRBitOnCapability proves grmarker.SetRBit sets 0x80; nothing tagged proves the OPEN sent inside the restart window carries it (production gate is restartFlagsFor in reactor/peer_gr_flags.go, not peer.go:574 as the tag says). The negative, TestSetRBitTimeGatePattern, re-implements the window check inside the test and cannot fail if the production gate breaks

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L497) | unit/verify | unproven |
| positive | [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L262) | unit/verify | unproven |

### [`RFC4724-4.1-5`](#rfc4724-4.1-5)

However, it MUST defer route selection for an address family until it either (a) receives the End-of-RIB marker from all its peers (excluding the ones with the "Restart State" bit set in the received capability and excluding the ones that do not advertise the graceful restart capability) or (b) the Selection_Deferral_Timer referred to below has expired. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-5, so no unit is bound to it.

### [`RFC4724-4.1-6`](#rfc4724-4.1-6)

After the BGP speaker performs route selection, the forwarding state of the speaker MUST be updated and any previously marked stale information MUST be removed. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-6, so no unit is bound to it.

### [`RFC4724-4.1-7`](#rfc4724-4.1-7)

Once the initial update is complete for an address family (including the case that there is no routing update to send), the End-of-RIB marker MUST be sent. (Section 4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestBuildEOR_IPv4Unicast proves only the marker's encoding, not that it is sent once the initial update completes; the negative tests EoR detection, a neighbouring rule. No tagged unit drives the send

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L228) | unit/verify | unproven |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L20) | unit/verify | unproven |

### [`RFC4724-4.1-8`](#rfc4724-4.1-8)

To put an upper bound on the amount of time a router defers its route selection, an implementation MUST support a (configurable) timer that imposes this upper bound. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.1-8, so no unit is bound to it.

### [`RFC4724-4.2-1`](#rfc4724-4.2-1)

In case it does not detect the termination of the old TCP session and still considers the BGP session as being established, it MUST treat the subsequent open connection from the peer as an indication of the termination of the old TCP session and act accordingly (when the Graceful Restart Capability has been received from the peer). (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.2-1, so no unit is bound to it.

### [`RFC4724-4.2-2`](#rfc4724-4.2-2)

"Acting accordingly" in this context means that the previous TCP session MUST be closed, and the new one retained. (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4724-4.2-2, so no unit is bound to it.

### [`RFC4724-4.2-3`](#rfc4724-4.2-3)

When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC4724SessionDownRetainsAndMarksRoutesStale asserts the production dispatch of retain-routes and mark-stale with the Restart Time; TestRFC4724RetentionCoversEveryAdvertisedFamily pins the family set to the capability's; the no-capability negative asserts nothing is dispatched

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerNoGRCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L297) | unit/verify | unproven |
| negative | [`TestRFC4724SessionDownWithoutCapabilityRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L96) | unit/verify | unproven |
| positive | [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L40) | unit/verify | unproven |
| positive | [`TestRFC4724RetentionCoversEveryAdvertisedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L124) | unit/verify | unproven |
| positive | [`TestRFC4724SessionDownRetainsAndMarksRoutesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L73) | unit/verify | unproven |

### [`RFC4724-4.2-4`](#rfc4724-4.2-4)

To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted. (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged unit asserts grStateManager's staleFamilies set is replaced on a second down; the deletion of previously stale routes is the purge-stale dispatch in gr.go, asserted only in TestRFC4724SessionDownRetainsAndMarksRoutesStale, which is tagged 4.2-3 and runs a first restart, not a consecutive one

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L286) | unit/verify | unproven |
| positive | [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L272) | unit/verify | unproven |

### [`RFC4724-4.2-5`](#rfc4724-4.2-5)

The router MUST NOT differentiate between stale and other routing information during forwarding. (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the positive (level-1 stale with higher LOCAL_PREF beats fresh) proves non-differentiation in selection. The negative is an LLGR level-2 route losing, which is RFC 9494 behaviour and does not violate this requirement, so no genuine negative exists

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L808) | unit/verify | unproven |
| positive | [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L840) | unit/verify | unproven |

### [`RFC4724-4.2-6`](#rfc4724-4.2-6)

In re-establishing the session, the "Restart State" bit in the Graceful Restart Capability of the OPEN message sent by the Receiving Speaker MUST NOT be set unless the Receiving Speaker has restarted. (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the positive, TestSetRBitTimeGatePattern, re-implements the restart-window condition inside the test and never calls the production gate restartFlagsFor (reactor/peer_gr_flags.go), so it cannot fail if that gate sets R on a cold start. Tag prose cites peer.go:574, stale

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L265) | unit/verify | unproven |
| positive | [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/grmarker_test.go#L494) | unit/verify | unproven |

### [`RFC4724-4.2-7`](#rfc4724-4.2-7)

If the session does not get re-established within the "Restart Time" that the peer advertised previously, the Receiving Speaker MUST delete all the stale routes from the peer that it is retaining. (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestGRStateManagerTimerExpiry installs its own expiry callback and asserts it fires; it cannot see whether the production callback (gp.onTimerExpired) deletes the stale routes

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L105) | unit/verify | unproven |
| positive | [`TestGRStateManagerTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L65) | unit/verify | unproven |

### [`RFC4724-4.2-8`](#rfc4724-4.2-8)

Once the session is re-established, if the "Forwarding State" bit for a specific address family is not set in the newly received Graceful Restart Capability, or if a specific address family is not included in the newly received Graceful Restart Capability, or if the Graceful Restart Capability is not received in the re-established session at all, then the Receiving Speaker MUST immediately remove all the stale routes from the peer that it is retaining for that address family. (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestGRStateManagerReconnectFBitZero asserts onSessionReestablished returns the F-bit-clear family; the purge-stale dispatch in gr.go is not asserted. The other two triggers (family absent, no GR capability) have tests (ReconnectMissingFamily, ReconnectNoGR) that are not tagged

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L102) | unit/verify | unproven |
| positive | [`TestGRStateManagerReconnectFBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L143) | unit/verify | unproven |

### [`RFC4724-4.2-9`](#rfc4724-4.2-9)

The Receiving Speaker MUST send the End-of-RIB marker once it completes the initial update for an address family (including the case that it has no routes to send) to the peer. (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the positive TestInitialSyncEORReachesTheSilentFamilyToo proves the send; TestBuildEOR_IPv4Unicast proves encoding only; the negative tests EoR detection, a neighbouring rule

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L229) | unit/verify | unproven |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/eor_test.go#L22) | unit/verify | unproven |
| positive | [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L104) | unit/verify | unproven |

### [`RFC4724-4.2-10`](#rfc4724-4.2-10)

The Receiving Speaker MUST replace the stale routes by the routing updates received from the peer. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Insert of a stale prefix clears StaleLevel to fresh with the new attributes; the negative keeps an unrefreshed stale prefix stale while a new prefix is fresh

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFamilyRIB_InsertNewDuringStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/stale_test.go#L179) | unit/verify | unproven |
| positive | [`TestFamilyRIB_InsertClearsStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/stale_test.go#L147) | unit/verify | unproven |

### [`RFC4724-4.2-11`](#rfc4724-4.2-11)

Once the End-of-RIB marker for an address family is received from the peer, it MUST immediately remove any routes from the peer that are still marked as stale for that address family. (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestGRStateManagerEORPurge asserts onEORReceived returns true per family; the removal is the purge-stale dispatch in handleEOREvent (gr.go), which no tagged unit asserts

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerEORForNonGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L314) | unit/verify | unproven |
| positive | [`TestGRStateManagerEORPurge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L185) | unit/verify | unproven |

### [`RFC4724-4-4`](#rfc4724-4-4)

A BGP speaker MAY advertise the Graceful Restart Capability for an address family to its peer if it has the ability to preserve its forwarding state for the address family when BGP restarts. (Section 4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tests prove the configured families are listed with F clear on a cold start, but not the MAY's condition (the family is listed only when the speaker can preserve its forwarding state for it), so a tuple for a family ze cannot preserve would still pass

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4724GRCapabilityClaimsNoFamilyItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_capability_test.go#L123) | unit/verify | revert, verified |
| positive | [`TestRFC4724GRCapabilityListsTheFamiliesOfTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_capability_test.go#L96) | unit/verify | revert, verified |

## Extraction sign-off

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
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 5 | walked | not stated |
| `4` | not stated | 3 | walked | not stated |
| `4.1` | not stated | 7 | walked | not stated |
| `4.2` | not stated | 11 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A pointer sentence that hands the reader to sections 4.1 and 4.2. It adds no obligation of its own; the procedures it promises are the sites in those two sections. | The following sections detail the procedures that MUST be followed by the Restarting Speaker as well as the Receiving Speaker once the Restarting Speaker restarts. |
| `5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The quoted [BGP-4] sentence that section 5 removes. It sits under 'Replace this text:' and the next site carries the replacement, so this is the text RFC 4724 deletes rather than an obligation it imposes. | In response to an indication that the TCP connection is successfully established (Event 16 or Event 17), the second connection SHALL be tracked until it sends an OPEN message. |
| `5:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The replacement keeps [BGP-4] Section 8.2.2's own collision handling for the case the Graceful Restart Capability was not received; section 5 opens 'The specific state machine modifications to [BGP-4], Section 8.2.2, are as follows.' RFC 4724 adds only the guard. The branch it does add, where the capability WAS received, is recorded as RFC4724-4.2-1 and RFC4724-4.2-2. | with If the Graceful Restart Capability with one or more AFIs/SAFIs has not been received for the session, then in response to an indication that a TCP connection is successfully established (Event 16 or Event 17), the second connection SHALL be tracked until it sends an OPEN message. |

## Superseded

No document obsoletes RFC 4724, so its obligations are stated where they were written.
