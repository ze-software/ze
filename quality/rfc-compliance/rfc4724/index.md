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
| Partial proof; remaining gap | 0.0% | 0 of 26 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 50.7% | 36 of 71 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

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
| Audit verdicts | 17 | of 26 gated MUSTs judged | 1 weak, wrong or unimplemented, 2 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 26 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 31 |
| Gated MUST-level | 26 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 8 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 71 |
| Tagged units | 71 |
| Recorded audit verdicts | 17 |
| Discrimination records | 36 |
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
| Annotated (including scoped evidence) | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **26** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (16):** [`RFC4724-3-2`](#rfc4724-3-2), [`RFC4724-3-3`](#rfc4724-3-3), [`RFC4724-3-4`](#rfc4724-3-4), [`RFC4724-4-1`](#rfc4724-4-1), [`RFC4724-4-2`](#rfc4724-4-2), [`RFC4724-4.1-4`](#rfc4724-4.1-4), [`RFC4724-4.1-7`](#rfc4724-4.1-7), [`RFC4724-4.2-3`](#rfc4724-4.2-3), [`RFC4724-4.2-4`](#rfc4724-4.2-4), [`RFC4724-4.2-5`](#rfc4724-4.2-5), [`RFC4724-4.2-6`](#rfc4724-4.2-6), [`RFC4724-4.2-7`](#rfc4724-4.2-7), [`RFC4724-4.2-8`](#rfc4724-4.2-8), [`RFC4724-4.2-9`](#rfc4724-4.2-9), [`RFC4724-4.2-10`](#rfc4724-4.2-10), [`RFC4724-4.2-11`](#rfc4724-4.2-11)

**Annotated (including scoped evidence) (10):** [`RFC4724-3-1`](#rfc4724-3-1), [`RFC4724-3-5`](#rfc4724-3-5), [`RFC4724-4.1-1`](#rfc4724-4.1-1), [`RFC4724-4.1-2`](#rfc4724-4.1-2), [`RFC4724-4.1-3`](#rfc4724-4.1-3), [`RFC4724-4.1-5`](#rfc4724-4.1-5), [`RFC4724-4.1-6`](#rfc4724-4.1-6), [`RFC4724-4.1-8`](#rfc4724-4.1-8), [`RFC4724-4.2-1`](#rfc4724-4.2-1), [`RFC4724-4.2-2`](#rfc4724-4.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4724-3-1` | A BGP speaker MUST NOT include more than one instance of the Graceful Restart Capability in the capability advertisement (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestExtractGRCapabilities_CapabilityDecl`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L111). **negative:** no negative test. **{single-polarity}:** ze's GR sender emits exactly one code-64 declaration per peer (internal/component/bgp/plugins/gr/gr.go:703 extractGRCapabilities appends one CapabilityDecl per peer) and the encoder writes a single TLV (internal/core/bgp/capability/capability.go:553 WriteTo); there is no code path that emits two instances, so the more-than-one case cannot be constructed to test negatively |
| `RFC4724-3-2` | The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L478). **positive:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedRestartFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L53). **negative:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L489). **negative:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedRestartFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L54) |
| `RFC4724-3-3` | The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L484). **positive:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L73). **negative:** `unit/verify` [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L498). **negative:** `unit/verify` [`TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L74) |
| `RFC4724-3-4` | If more than one instance of the Graceful Restart Capability is carried in the capability advertisement, the receiver of the advertisement MUST ignore all but the last instance of the Graceful Restart Capability. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L62). **negative:** `unit/verify` [`TestNegotiateGracefulRestartLastInstance`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L70) |
| `RFC4724-3-5` | When set (value 1), this bit indicates that the BGP speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker from the speaker before advertising routing information to the speaker. (Section 3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze advertises its Adj-RIB-Out and per-family End-of-RIB immediately on reaching Established (internal/component/bgp/reactor/peer_initial_sync.go:277,334) and has no receive-side mechanism that gates advertisement on a peer's End-of-RIB; the only End-of-RIB timer (internal/component/bgp/reactor/session_health.go:114 startEORTimer) raises a health warning and never defers advertisement, so there is no wait state for the R bit to override |
| `RFC4724-4-1` | The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L16). **positive:** `unit/verify` [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L99). **positive:** `unit/verify` [`TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L743). **positive:** `unit/verify` [`TestInitialSyncMarkerWaitsForNoProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L776). **positive:** `unit/verify` [`TestInitialSyncShutsTheQueueGateAndFreesTheRailsWithTheMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L341). **positive:** `unit/verify` [`TestRoutePushingBindingsCountBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L419). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L224). **negative:** `unit/verify` [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L23). **positive:** `interop/nightly` [`checkNoFamilyEndOfRIB`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1191) |
| `RFC4724-4-2` | It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestGRStateManagerNotificationBypass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L219). **negative:** `unit/verify` [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L45) |
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
| `RFC4724-4.2-3` | When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L41). **positive:** `unit/verify` [`TestRFC4724RetentionCoversEveryAdvertisedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L169). **positive:** `unit/verify` [`TestRFC4724SessionDownRetainsAndMarksRoutesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L70). **negative:** `unit/verify` [`TestGRStateManagerNoGRCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L296). **negative:** `unit/verify` [`TestRFC4724SessionDownWithoutCapabilityRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L98) |
| `RFC4724-4.2-4` | To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L271). **positive:** `unit/verify` [`TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L35). **negative:** `unit/verify` [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L285). **negative:** `unit/verify` [`TestRFC4724ConsecutiveRestartDeletesBeforeMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L46) |
| `RFC4724-4.2-5` | The router MUST NOT differentiate between stale and other routing information during forwarding. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L848). **negative:** `unit/verify` [`TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_stale_forwarding_test.go#L27) |
| `RFC4724-4.2-6` | In re-establishing the session, the "Restart State" bit in the Graceful Restart Capability of the OPEN message sent by the Receiving Speaker MUST NOT be set unless the Receiving Speaker has restarted. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestRFC4724OpenOutsideTheRestartWindowClearsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L86). **positive:** `unit/verify` [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L497). **negative:** `unit/verify` [`TestRFC4724OpenInsideTheRestartWindowSetsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L66). **negative:** `unit/verify` [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L266) |
| `RFC4724-4.2-7` | If the session does not get re-established within the "Restart Time" that the peer advertised previously, the Receiving Speaker MUST delete all the stale routes from the peer that it is retaining. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L66). **positive:** `unit/verify` [`TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L96). **positive:** `unit/verify` [`TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L116). **negative:** `unit/verify` [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L104). **negative:** `unit/verify` [`TestGRStateManagerRestartExpiryAfterReconnect`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L427). **negative:** `unit/verify` [`TestGRStateManagerRestartExpiryRejectsOldCycle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L378). **negative:** `unit/verify` [`TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L114) |
| `RFC4724-4.2-8` | Once the session is re-established, if the "Forwarding State" bit for a specific address family is not set in the newly received Graceful Restart Capability, or if a specific address family is not included in the newly received Graceful Restart Capability, or if the Graceful Restart Capability is not received in the re-established session at all, then the Receiving Speaker MUST immediately remove all the stale routes from the peer that it is retaining for that address family. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerReconnectFBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L142). **positive:** `unit/verify` [`TestRFC4724ReestablishedWithoutCapabilityPurgesOnTheStructuredPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_capability_absent_test.go#L31). **positive:** `unit/verify` [`TestRFC4724ReestablishedWithoutForwardingStatePurgesTheFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L140). **negative:** `unit/verify` [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L101). **negative:** `unit/verify` [`TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L180) |
| `RFC4724-4.2-9` | The Receiving Speaker MUST send the End-of-RIB marker once it completes the initial update for an address family (including the case that it has no routes to send) to the peer. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L22). **positive:** `unit/verify` [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L104). **negative:** `unit/verify` [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L229). **negative:** `unit/verify` [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L24) |
| `RFC4724-4.2-10` | The Receiving Speaker MUST replace the stale routes by the routing updates received from the peer. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFamilyRIB_InsertClearsStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L147). **negative:** `unit/verify` [`TestFamilyRIB_InsertNewDuringStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L179) |
| `RFC4724-4.2-11` | Once the End-of-RIB marker for an address family is received from the peer, it MUST immediately remove any routes from the peer that are still marked as stale for that address family. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestGRStateManagerEORPurge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L184). **positive:** `unit/verify` [`TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L51). **negative:** `unit/verify` [`TestGRStateManagerEORForNonGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L313). **negative:** `unit/verify` [`TestRFC4724EndOfRIBLeavesOtherFamiliesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L71) |
| `RFC4724-2-1` | Although the End-of-RIB marker is specified for the purpose of BGP graceful restart, it is noted that the generation of such a marker upon completion of the initial update would be useful for routing convergence in general, and thus the practice is recommended. (Section 2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4-3` | In addition, even if the speaker does not have the ability to preserve its forwarding state for any address family during BGP restart, it is still recommended that the speaker advertise the Graceful Restart Capability to its peer (Section 4) | RECOMMENDED | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4724-4-4` | A BGP speaker MAY advertise the Graceful Restart Capability for an address family to its peer if it has the ability to preserve its forwarding state for the address family when BGP restarts. (Section 4) | MAY | 4 | **positive:** `unit/verify` [`TestRFC4724GRCapabilityKeepsEveryFamilyTheRIBStores`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_rib_families_test.go#L93). **positive:** `unit/verify` [`TestRFC4724GRCapabilityListsTheFamiliesOfTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_gr_capability_test.go#L96). **negative:** `unit/verify` [`TestRFC4724GRCapabilityClaimsNoFamilyItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_gr_capability_test.go#L123). **negative:** `unit/verify` [`TestRFC4724GRCapabilityOmitsAFamilyNoRIBStores`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_rib_families_test.go#L60) |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sender half: TestGracefulRestartEncodeReservedBits (WriteTo masks to R bit and 12-bit time). Receiver half: TestRFC4724ReceiverIgnoresReservedRestartFlags parses the nibble with reserved 0x3 set, R clear and set, and requires the result equal to the clean parse. Judge breaks: R bit read with mask 0x9000, and Restart Time read with mask 0x3FFF, each turned the test red. Only 0x3 used because RFC 8538 assigned the N bit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L489) | unit/verify | unproven |
| negative | [`TestRFC4724ReceiverIgnoresReservedRestartFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L478) | unit/verify | unproven |
| positive | [`TestRFC4724ReceiverIgnoresReservedRestartFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L53) | unit/verify | revert, verified |

### [`RFC4724-3-3`](#rfc4724-3-3)

The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sender half: TestGracefulRestartEncodeReservedBits (only F bit 0x80 per family). Receiver half: TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags sets all seven reserved bits (0x7F) with F clear and set and requires the parse equal to the clean one. Judge break: ForwardingState read as flags != 0 turned it red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L498) | unit/verify | unproven |
| negative | [`TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestGracefulRestartEncodeReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L484) | unit/verify | unproven |
| positive | [`TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4724_reserved_bits_test.go#L73) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/peer_initial_sync_test.go::TestInitialSyncEORReachesTheSilentFamilyToo, internal/component/bgp/reactor/peer_initial_sync_test.go::TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily, internal/component/bgp/reactor/peer_initial_sync_test.go::TestInitialSyncMarkerWaitsForNoProcess, internal/component/bgp/reactor/peer_initial_sync_test.go::TestInitialSyncShutsTheQueueGateAndFreesTheRailsWithTheMarker, internal/component/bgp/reactor/peer_initial_sync_test.go::TestRoutePushingBindingsCountBothRails moved. Positives: sendInitialRoutes puts one End-of-RIB per negotiated family on the wire, including a silent family and a no-MP session. Negative TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes: while a plugin producing the initial update has not acknowledged peer-up, sendInitialRoutes is observed waiting and nothing is on the wire (EORSent 0); after the acknowledgement exactly the IPv4 unicast marker is written. Red on a marker sent ahead of the initial update or not sent after it. TestIsEndOfRIBAnyFamily is a neighbouring detector rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L224) | unit/verify | unproven |
| negative | [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L16) | unit/verify | unproven |
| positive | [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L99) | unit/verify | unproven |
| positive | [`TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L743) | unit/verify | unproven |
| positive | [`TestInitialSyncMarkerWaitsForNoProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L776) | unit/verify | unproven |
| positive | [`TestInitialSyncShutsTheQueueGateAndFreesTheRailsWithTheMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L341) | unit/verify | unproven |
| positive | [`TestRoutePushingBindingsCountBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L419) | unit/verify | unproven |
| positive | [`checkNoFamilyEndOfRIB`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1191) | interop/nightly | unproven |

### [`RFC4724-4-2`](#rfc4724-4-2)

It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message. (Section 4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged units call grStateManager.onSessionDown with wasNotification injected as true/false. In production handleStructuredState/handleStateEvent (gr.go) derive wasNotification from reason == "notification", and the only session-down producer (peer_run.go via notifyPeerClosed) sends "session closed" or "connection lost", so the NOTIFICATION branch is unreachable and routes are retained after a NOTIFICATION teardown. The tests stay green against that defect

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L45) | unit/verify | unproven |
| positive | [`TestGRStateManagerNotificationBypass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L219) | unit/verify | unproven |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c25 judge). rfc4724_restart_state_test.go builds the OPEN through Session.buildOpen with Peer.getPluginCapabilities wired as peer_run.go wires it, a bgp-gr code 64 value with R clear served by the test-only Reactor.pluginCapabilitySeam (nil in production; with it nil getPluginCapabilities behaves as at HEAD), and parses the sent capability: inside the restart window R is set with Restart Time and the IPv4 unicast tuple unchanged (positive); cold start and passed deadline leave R clear (negative, the bit is not blanket). Overlay probes: SetRBit removed reds only the positive, window forced true reds only the negative. Records: restartFlagsFor revert (+), getPluginCapabilities revert (-). The grmarker units stay as supplementary. Re-judged 2026-10-02 (BGP c26 judge): the grmarker supplementary units' tag prose now states what each body asserts (TestSetRBitOnCapability: SetRBit sets 0x80 and keeps Restart Time 120; TestSetRBitTimeGatePattern: SetRBit edits a copy, the caller's caps keep R=0) and names the production gate Peer.getPluginCapabilities -> restartFlagsFor (peer_gr_flags.go), proven by the reactor units above; bodies unchanged, read against the prose and found accurate. Both grmarker units carry SetRBit-revert records, observed red. Verdict rests on the reactor units, unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L500) | unit/verify | revert, verified |
| negative | [`TestRFC4724OpenOutsideTheRestartWindowClearsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L262) | unit/verify | revert, verified |
| positive | [`TestRFC4724OpenInsideTheRestartWindowSetsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L65) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. New unit TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete drives peer.sendInitialRoutes on an established peer and reads the socket. Positive: empty table -> the wire is exactly the IPv4 unicast EoR (the RFC's "including the case that there is no routing update to send"); with default-originate -> [UPDATE 0.0.0.0/0, EoR], EORSent 1. Negative: no EoR precedes the route and exactly one follows it (a marker before completion violates "Once the initial update is complete"). Observed-red records +/- (revert peer_initial_sync.go::sendEORFamilies); both polarities share one break, which reddens the unit as a whole rather than the ordering clause alone. Message-level EoR encoding units stay as supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L228) | unit/verify | unproven |
| negative | [`TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_initial_update_eor_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L20) | unit/verify | unproven |
| positive | [`TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_initial_update_eor_test.go#L42) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4724 §4.2: "When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information." Source rejudgment of all five tagged carriers: internal/component/bgp/plugins/gr/gr_state_test.go::TestGRStateManagerRouteRetention checks activation, TestGRStateManagerNoGRCapability checks its absence; those alone are insufficient. internal/component/bgp/plugins/gr/rfc4724_retention_test.go::TestRFC4724SessionDownRetainsAndMarksRoutesStale now uses real_rib_test.go::newGRWithRealRIB and asserts received IPv4 retained at stale level 1, with an IPv6 family control retained at level 1 only when advertised; TestRFC4724SessionDownWithoutCapabilityRetainsNothing requires a pre-existing received route to disappear after DOWN without code 64. TestRFC4724RetentionCoversEveryAdvertisedFamily pins exactly IPv4+IPv6 in the state family set and the armed restart timer. Producers: gr.go::handleStateEvent -> gr_state.go::onSessionDownDeferred -> gr.go::retainPeerFamilies, then rib_commands.go::retainRoutes/markStaleCommand and rib.go::handleState. These assertions discriminate removal, missing stale marking and peer-wide overretention, rather than merely inspecting dispatched command text. Three positive tags, two negative tags; advertised versus absent capability/family are separate input cases. Both retention and stale marking clauses are checked at actual RIB storage; this says nothing about restarting-speaker Loc-RIB preservation or collision handling, whose existing gaps remain. No runtime or observed-red renewal was performed by this judge. Post-lint independent source rejudgment: both real-RIB tagged callers now omit only the invariant testPeer argument to received/down; those helpers still put testPeer in the event and verify the UPDATE entered actual received storage. The positive still asserts exact stale level 1, IPv6 retention iff advertised, and deletion of unadvertised IPv6; the no-GR negative requires actual route absence. Re-read all five carriers and handleStateEvent -> retainPeerFamilies/onSessionDownDeferred. newGRWithRealRIB reports mux.Close errors without skipping cancellation, plugin close or worker join. This adds cleanup failure visibility, not an alternative acceptable protocol outcome. Verdict remains enforced; both changed real-RIB tags currently have no discrimination record and owe first native observed-red records, not invented hashes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerNoGRCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L296) | unit/verify | unproven |
| negative | [`TestRFC4724SessionDownWithoutCapabilityRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestGRStateManagerRouteRetention`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L41) | unit/verify | unproven |
| positive | [`TestRFC4724RetentionCoversEveryAdvertisedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L169) | unit/verify | unproven |
| positive | [`TestRFC4724SessionDownRetainsAndMarksRoutesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L70) | unit/verify | revert, verified |

### [`RFC4724-4.2-4`](#rfc4724-4.2-4)

To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4724 §4.2: "To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted." Read every tagged carrier: internal/component/bgp/plugins/gr/gr_state_test.go::TestGRConsecutiveRestartClearsPriorStale compares two-family then one-family state and a first-restart control; state-only evidence is supplemented by internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go::TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes and TestRFC4724ConsecutiveRestartDeletesBeforeMarking. Their rfc4724SecondRestart helper drives two actual GR DOWN cycles with the registered RIB engine, verifies both routes stale after the first, reestablishes and refreshes only 198.51.101.0/24, then requires unrefreshed 198.51.100.0/24 gone and refreshed 198.51.101.0/24 retained at stale level 1. gr.go::handleStateEvent purges before retainPeerFamilies and mark-stale; rib_commands.go::purgeStaleCommand removes old stale storage while markStaleCommand marks the new generation. Purging after marking or omitting purge violates different exact assertions. Two positive and two negative tags, including both tags on the state test; the real-RIB functions share one scenario and duplicate the two assertions, so they are not two independent experiments, but the old-stale and refreshed routes are distinct positive/control inputs rather than one assertion with two polarities. Restricted to conventional GR: RFC9494-4.2-7/-9 consecutive LLGR gaps are unchanged. No runtime or observed-red renewal was performed by this judge. Post-lint independent source rejudgment: the tagged assertion functions are unchanged by the argument simplification; their untagged rfc4724SecondRestart and realGRRIB helpers are affected. The helper still stores two routes, drops the session, proves both stale, reestablishes, refreshes only the second, and drops again. Both tagged functions require old-stale deletion and refreshed-route retention at level 1; these are distinct input-route outcomes despite duplicated functions. handleStateEvent still purges before retain/mark. Removed arguments were testPeer constants; the same received and DOWN events reach the registered RIB. Checked mux.Close cannot weaken this result. Preserve enforced and the pre-lint planned renewals for both existing records; no LLGR consecutive-restart claim is made.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L285) | unit/verify | unproven |
| negative | [`TestRFC4724ConsecutiveRestartDeletesBeforeMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestGRConsecutiveRestartClearsPriorStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L271) | unit/verify | unproven |
| positive | [`TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_consecutive_test.go#L35) | unit/verify | revert, verified |

### [`RFC4724-4.2-5`](#rfc4724-4.2-5)

The router MUST NOT differentiate between stale and other routing information during forwarding. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c25 judge). Positive TestComparePair_GRStaleCompetesNormally: a level-1 stale route with the higher LOCAL_PREF wins. New genuine negative (R1(b)) TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone drives SelectBest toward both violations in both argument orders: a fresh route with the higher LOCAL_PREF beats the stale one (staleness does not favour) and with all attributes equal the stale route from the lower address wins the tie-break (staleness costs nothing). Overlay probes: favouring level 1 reds only the negative; lowering the depreference threshold to 1 reds the positive and the tie subtest. Record: comparePair revert (-). The old negative tag on TestComparePair_LLGRStale proved RFC 9494 depreference and was removed (D-15). Proof is at the RIB best path the forwarding plane is handed; sysrib and fib-kernel hold no GR-stale distinction.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4724_stale_forwarding_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L848) | unit/verify | unproven |

### [`RFC4724-4.2-6`](#rfc4724-4.2-6)

In re-establishing the session, the "Restart State" bit in the Graceful Restart Capability of the OPEN message sent by the Receiving Speaker MUST NOT be set unless the Receiving Speaker has restarted. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c25 judge). Positive TestRFC4724OpenOutsideTheRestartWindowClearsRestartState: the OPEN built through the production getter on a cold start and after the deadline carries R clear; the overlay probe forcing the window true turns both subtests red. Negative TestRFC4724OpenInsideTheRestartWindowSetsRestartState pins the 'unless restarted' exception (R set inside the window), so the positive cannot pass by a getter that never sets R. Records: getPluginCapabilities revert (+), restartFlagsFor revert (-). Same seam and same supplementary grmarker units as RFC4724-4.1-4. Re-judged 2026-10-02 (BGP c26 judge): the grmarker supplementary units' tag prose now states what each body asserts (TestSetRBitOnCapability: SetRBit sets 0x80 and keeps Restart Time 120; TestSetRBitTimeGatePattern: SetRBit edits a copy, the caller's caps keep R=0) and names the production gate Peer.getPluginCapabilities -> restartFlagsFor (peer_gr_flags.go), proven by the reactor units above; bodies unchanged, read against the prose and found accurate. Both grmarker units carry SetRBit-revert records, observed red. Verdict rests on the reactor units, unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSetRBitOnCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L266) | unit/verify | revert, verified |
| negative | [`TestRFC4724OpenInsideTheRestartWindowSetsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestSetRBitTimeGatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/grmarker/rfc4724_grmarker_test.go#L497) | unit/verify | revert, verified |
| positive | [`TestRFC4724OpenOutsideTheRestartWindowClearsRestartState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_restart_state_test.go#L86) | unit/verify | revert, verified |

### [`RFC4724-4.2-7`](#rfc4724-4.2-7)

If the session does not get re-established within the "Restart Time" that the peer advertised previously, the Receiving Speaker MUST delete all the stale routes from the peer that it is retaining. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent supplemental source rejudgment, 2026-10-06. RFC4724 Section 4.2: "If the session does not get re-established within the "Restart Time" that the peer advertised previously, the Receiving Speaker MUST delete all the stale routes from the peer that it is retaining." Read all seven tagged units: gr_state_test.go TestGRStateManagerTimerExpiry, TestGRStateManagerReconnectWithFBit, TestGRStateManagerRestartExpiryRejectsOldCycle, TestGRStateManagerRestartExpiryAfterReconnect; rfc4724_dispatch_test.go TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes and TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes; rfc4724_retention_test.go TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking. Explicit callback units prove owner/cancellation decisions, not elapsed time; the actual one-second timer dispatch and two-second timely-reconnect control prove complementary timing behavior. Zero-time test exercises both JSON state handler and structured EventKindState/SessionStateDown dispatch; it requires exact purge/retain/mark/release order, an actually populated registered RIB becoming empty, no active GR and no resurrection after RIB DOWN. real_rib_test.go consumes commands through registered bgp-rib RunEngine and checks verdicts and received route inventory, not just recorded strings. Traced onSessionDownDeferred -> completion after both handlers dispatch stale marking -> startRestartTimer anchored to original deadline -> handleTimerExpired owner/deadline guards -> wireStateCallbacks/onTimerExpired/releaseRoutes -> registered request bgp rib release-routes -> RIBManager.releaseRoutes, removing retained inventory and retention metadata and reconciling best path. Ordinary no-LLGR expiry is isolated; no absent LLGR feature is inferred. The moved zero-time unit has no current diff against HEAD; this is preexisting stale judgment debt, with history showing the integrated checkpoint and earlier delayed-DOWN repair, not a current mechanical edit. Prior enforced verdict retained. No tests, timers, mutations or gates were executed by this judge; no new source defect found.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L104) | unit/verify | unproven |
| negative | [`TestGRStateManagerRestartExpiryAfterReconnect`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L427) | unit/verify | revert, verified |
| negative | [`TestGRStateManagerRestartExpiryRejectsOldCycle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L378) | unit/verify | revert, verified |
| negative | [`TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestGRStateManagerTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L116) | unit/verify | revert, verified |

### [`RFC4724-4.2-8`](#rfc4724-4.2-8)

Once the session is re-established, if the "Forwarding State" bit for a specific address family is not set in the newly received Graceful Restart Capability, or if a specific address family is not included in the newly received Graceful Restart Capability, or if the Graceful Restart Capability is not received in the re-established session at all, then the Receiving Speaker MUST immediately remove all the stale routes from the peer that it is retaining for that address family. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c25 judge) after the D-8 fix. Producer read: gr.go forgetGRCapability (deletes peerCaps and peerLLGRCaps, RFC 4724 4.2 and RFC 9494 4.5 quoted) is called from handleOpenEvent and handleStructuredOpen only when the received OPEN has no code 64; an OPEN carrying 64 still stores the new capability, so GR state is not dropped. Opens are subscribed 'direction received' and the server filters by direction (events.go PeerScopedProcs), so Ze's own OPEN never reaches it. peerCaps is read only by gr.go's two state handlers. All three triggers are now proven at the production dispatch: F clear and family omitted (OPEN through handleEvent) purge exactly that family; capability absent purges both on the JSON path (OPEN with only cap 65) and the structured path (only cap 65, no optional parameters) through a real down/up cycle. Negative: F set for both purges nothing. Judge overlay probe removing the peerCaps delete turned exactly the three absent-capability subtests red. Records: forgetGRCapability revert (both positives), onSessionReestablished revert (negative). Not proven here: the JSON path's test hex omits the 2-byte capability header that format.JSONEncoder.Open's fallback includes for code 64; production runs the structured path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerReconnectWithFBit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L101) | unit/verify | unproven |
| negative | [`TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L180) | unit/verify | revert, verified |
| positive | [`TestGRStateManagerReconnectFBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L142) | unit/verify | unproven |
| positive | [`TestRFC4724ReestablishedWithoutCapabilityPurgesOnTheStructuredPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_capability_absent_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestRFC4724ReestablishedWithoutForwardingStatePurgesTheFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L140) | unit/verify | revert, verified |

### [`RFC4724-4.2-9`](#rfc4724-4.2-9)

The Receiving Speaker MUST send the End-of-RIB marker once it completes the initial update for an address family (including the case that it has no routes to send) to the peer. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/peer_initial_sync_test.go::TestInitialSyncEORReachesTheSilentFamilyToo moved. Positive TestInitialSyncEORReachesTheSilentFamilyToo proves the marker is sent including for a family with no routes. Negative TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes: no End-of-RIB before the initial update for IPv4 unicast completes (plugin barrier held, sendInitialRoutes observed waiting, wire empty), then exactly that marker once it does. TestBuildEOR_IPv4Unicast proves encoding only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsEndOfRIBAnyFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L229) | unit/verify | unproven |
| negative | [`TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4724_reactor_b_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestBuildEOR_IPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4724_eor_test.go#L22) | unit/verify | unproven |
| positive | [`TestInitialSyncEORReachesTheSilentFamilyToo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L104) | unit/verify | unproven |

### [`RFC4724-4.2-10`](#rfc4724-4.2-10)

The Receiving Speaker MUST replace the stale routes by the routing updates received from the peer. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Insert of a stale prefix clears StaleLevel to fresh with the new attributes; the negative keeps an unrefreshed stale prefix stale while a new prefix is fresh

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFamilyRIB_InsertNewDuringStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L179) | unit/verify | unproven |
| positive | [`TestFamilyRIB_InsertClearsStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4724_stale_test.go#L147) | unit/verify | unproven |

### [`RFC4724-4.2-11`](#rfc4724-4.2-11)

Once the End-of-RIB marker for an address family is received from the peer, it MUST immediately remove any routes from the peer that are still marked as stale for that address family. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c24 judge). The production removal is now asserted: TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes (production callbacks, session-down sequence consumed) sends an IPv4 unicast End-of-RIB and asserts exactly one dispatch, purge-stale <peer> ipv4/unicast, before handleEOREvent returns; TestRFC4724EndOfRIBLeavesOtherFamiliesStale asserts no ipv6 or peer-wide purge and nothing for an End-of-RIB of a family never retained. Judge overlay probe (handleEOREvent purging regardless of onEORReceived) turned only the negative red; the author's probe (purge-stale without the family) turned both red. Rib side: TestRIBPurgeStaleFamilyCommand. HEAD gr_state_test units stay supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGRStateManagerEORForNonGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L313) | unit/verify | unproven |
| negative | [`TestRFC4724EndOfRIBLeavesOtherFamiliesStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestGRStateManagerEORPurge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L184) | unit/verify | unproven |
| positive | [`TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go#L51) | unit/verify | revert, verified |

### [`RFC4724-4-4`](#rfc4724-4-4)

A BGP speaker MAY advertise the Graceful Restart Capability for an address family to its peer if it has the ability to preserve its forwarding state for the address family when BGP restarts. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c29 judge), owner rulings 7 and 8(a). parseGRCapValue now lists a family only when nlrisplit.Supported (the RIB plugin, a hard Dependency of bgp-gr resolved by ResolveDependencies at autoload, can store and re-send it). Negative TestRFC4724GRCapabilityOmitsAFamilyNoRIBStores registers ipv4/rib-less (SAFI 241, no splitter, preconditions asserted) and pins 007800010100 for the session list and 0078 (capability still sent, no tuple) for a configured list naming only it, each with the Warn carrying peer and family; HEAD negative ClaimsNoFamilyItDoesNotCarry keeps the uncarried-family refusal. Positives: TestRFC4724GRCapabilityKeepsEveryFamilyTheRIBStores (ipv4/flow + ipv4/unicast, 00780001850000010100, no Warn) and ListsTheFamiliesOfTheSession (exact two-tuple payload). Four observed-red records (revert parseGRCapValue); author probe inverting the condition reds all four, and the pre-fix run reds only the negative, so the pair discriminates the condition itself.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4724GRCapabilityClaimsNoFamilyItDoesNotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_gr_capability_test.go#L123) | unit/verify | revert, verified |
| negative | [`TestRFC4724GRCapabilityOmitsAFamilyNoRIBStores`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_rib_families_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC4724GRCapabilityListsTheFamiliesOfTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_gr_capability_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestRFC4724GRCapabilityKeepsEveryFamilyTheRIBStores`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_rib_families_test.go#L93) | unit/verify | revert, verified |

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
