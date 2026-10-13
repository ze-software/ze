# RFC 5882 - Generic Application of Bidirectional Forwarding Detection (BFD)

Partial. Every requirement this repository extracted from RFC 5882, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 3 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 3 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 30 of 30 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 32 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 3 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 66.7% | 2 of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 3 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 3 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Partial proof; remaining gap | 33.3% | 1 of 3 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Audit verdicts | 6 | of 3 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 3 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | bad | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
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
| Requirements | 32 |
| Gated MUST-level | 3 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 30 |
| Tagged units | 30 |
| Recorded audit verdicts | 6 |
| Discrimination records | 30 |
| Summary | `rfc/short/rfc5882.md` |
| Requirement shard | `rfc/requirements/rfc5882.md` |
| RFC text | `rfc/full/rfc5882.txt` |

## Enrolment

Enrolled: Generic Application of BFD: three MUST-level requirements. RFC5882-4.4-1 (multiple control protocols wanting a BFD session to the same remote/data-protocol MUST share a single BFD session) is met for every configuration in which the clients reach one session, and PARTIALLY met in the two configurations named below, which is what docs/features/rfc-status.md publishes. The registry half is complete: EnsureSession (internal/component/bfd/engine/engine.go:344) is refcounted and path-keyed on api.Key{Peer,Local,Interface,VRF,Mode}, deliberately excluding timers (internal/component/bfd/api/events.go:155-156), and both polarities are proven by TestBFDSharedSessionSameKey (a second same-Key request bumps the refcount to 2 on ONE session, which survives until the last release) and TestBFDDistinctSessionsDifferentKey (two different remotes get two distinct sessions). The key half has two steps. api.SessionRequest.Canonical completes what a client left out from the link table, and clears a multi-hop interface; it never derives a multi-hop local address, so a multi-hop session is keyed on peer, VRF and mode, plus the local address only where a client pinned one (RFC 5883 Section 4.1 makes a pinned address part of the identity). Then engine.Loop.EnsureSession joins a request that left a field unset to the ONE live session it cannot be told apart from (sharedEntryLocked, sharesSession), in either arrival order, which closes the multi-hop cases (no route, an egress interface with no or several addresses, a non-default VRF, no interface backend) and the single-hop cases for a global peer (two links on one subnet, a peer on no connected prefix, no interface backend). TestMultiHopClientsInAVRFShareOneSession proves it through applyPinned and pluginService.EnsureSession, and TestMultiHopClientsToTwoRemoteSystemsGetTwoSessions proves a different remote system still gets its own session. Two configurations still give two sessions, listed under "Multiple Control Protocols (Section 4.4)" on this page: an IPv6 link-local peer whose client names no interface (RFC 4007 Section 6: the address does not name one system), and a client that left a field unset while two or more live sessions to that peer differ in it (it could mean either, so it is not merged by a guess). Each closes when the client names the interface or the local address. RFC5882-4.1-1 (establishment allowed under AdminDown) is {not-applicable}: Ze never gates control-protocol establishment on BFD state -- the BFD client attaches only after the adjacency is up (BGP on StateEstablished, OSPF on Full) and is strictly additive, so no AdminDown session can block establishment. RFC5882-10.1.3-1 (OSPF virtual links MUST use RFC 5883 multihop) is {not-applicable}: Ze does not run BFD on OSPF virtual links (the BFD-for-OSPF client keys on per-interface config; a virtual link is a synthetic backbone link, not a configured interface, so it never gets a session). RFC5882-4.2-1 (no control protocol action on Up to AdminDown, or on Up to Down caused by the remote system's AdminDown) is MET, at the service boundary and at both clients, BGP and OSPF. api.StateChange.RemoteAdminDown carries the neighbor's AdminDown, which RFC 5880 Section 6.8.6 otherwise erases into a local Down with the same diagnostic a peer-signaled Down produces, and both polarities are proven by TestRFC5882RemoteAdminDownIsDistinguished and TestRFC5882NeighborDownIsNotRemoteAdminDown. Those two check what the event SAYS; Peer.runBFDSubscriber (internal/component/bgp/reactor/peer_bfd.go) then reads the field and raises no FSM event for that case, which is the "control protocol action" Section 4.2 names, and TestBFDRemoteAdminDownDoesNotTeardown checks what the client DOES with a control in the same body: the same local state and diagnostic with the neighbor not AdminDown is a path failure and still drops the session. The recording of state and time is deliberately left to run, because the OPEN rail reads it and bfdStrictHolds must still see a session that is not Up. Section 3.2 conditions the obligation on the client having "independent means of liveness detection", and BGP's hold timer is one. The OSPF client, engine.runBFDSubscriber (internal/plugins/ospf/bfd_client.go), takes no action on a local AdminDown or on a Down with RemoteAdminDown set: it neither declares the neighbor down nor releases the session, and TestOSPFBFDAdminDownTakesNoAction and TestOSPFBFDRemoteAdminDownTakesNoAction (internal/plugins/ospf/rfc5882_bfd_client_test.go) check it, the second with a control in the same body (the same Down without the flag still declares the neighbor down). OSPF's RouterDeadInterval is its independent liveness detection. The other SHOULD/SHOULD-NOT clauses (single session per path, hysteresis notify, bring-up connectivity semantics, GR fate-sharing, static-route withdrawal, planned-outage AdminDown, authentication) and the MAY clauses are not gated.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

BFD integration model used by BGP and static next-hop tracking.

**What the ledger says remains:**

Same BFD partial status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 1 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (3):** [`RFC5882-4.1-1`](#rfc5882-4.1-1), [`RFC5882-4.4-1`](#rfc5882-4.4-1), [`RFC5882-10.1.3-1`](#rfc5882-10.1.3-1)

**Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) (1):** [`RFC5882-4.4-1`](#rfc5882-4.4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5882-4.1-1` | If the session state on either the local or remote system (if known) is AdminDown, BFD has been administratively disabled, and the establishment of a control protocol adjacency MUST be allowed. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze never conditions control-protocol adjacency establishment on BFD session state. The BFD client is attached only AFTER the adjacency is already up -- BGP starts it on StateEstablished (internal/component/bgp/reactor/peer_bfd.go:51-52,61-73) and OSPF opens the session only when a neighbor reaches Full (internal/plugins/ospf/bfd_client.go:124-138, onNeighborFull -> bfdNeighborFull) -- and it is strictly additive: a failure detector, never a bring-up gate (peer_bfd.go:59-60, bfd_client.go:12-13). With no BFD-gated establishment path anywhere in Ze, a session in AdminDown (or any state) cannot block establishment, so this AdminDown carve-out to establishment-blocking has no applicable code path |
| `RFC5882-4.4-1` | If multiple control protocols wish to establish BFD sessions with the same remote system for the same data protocol, all MUST share a single BFD session. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestBFDSharedSessionSameKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L18). **positive:** `unit/verify` [`TestCanonicalCollapsesEveryClientShapeOntoOneKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/api/rfc5882_session_identity_test.go#L59). **positive:** `unit/verify` [`TestCanonicalCollapsesMultiHopShapesOntoOneKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/api/rfc5882_session_identity_test.go#L150). **positive:** `unit/verify` [`TestMultiHopClientsInAVRFShareOneSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_vrf_shared_key_test.go#L39). **positive:** `unit/verify` [`TestOSPFNeighborRequestReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5882_shared_key_test.go#L16). **positive:** `unit/verify` [`TestPinnedSessionReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_session_identity_test.go#L19). **positive:** `unit/verify` [`TestSharedSessionWidensOnRelease`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L112). **positive:** `unit/verify` [`TestStrictPeerRequestReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L206). **negative:** `unit/verify` [`TestBFDDistinctSessionsDifferentKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L73). **negative:** `unit/verify` [`TestMultiHopClientsToTwoRemoteSystemsGetTwoSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_vrf_shared_key_test.go#L73). **{partial}:** tested "all MUST share a single BFD session"; gap "with the same remote system"; internal/component/bfd/engine/shared_session.go::sharedEntryLocked is the boundary: Tested covers the BGP request builder, the OSPF request builder and the pinned-session parser reaching one canonical key for one neighbor, two requests on one key sharing one session at refcount 2 that lives until the last release, a multi-hop BGP request with no local address joining the pinned session to the same peer in a VRF at refcount 2, a session opened by an unpinned multi-hop client answering for the unpinned key again after a pinned client leaves so that a client pinned to another local address joins it, and a request to a different remote system getting its own session; Gap is the two configurations in which Ze cannot tell from the request that two clients name the same remote system, so they still get two sessions: an IPv6 link-local peer whose client names no interface (RFC 4007 Section 6: the address alone does not name one system), and a client that left a field unset while two or more live sessions to that peer differ in that field (it could mean either, so it is not merged by a guess); each closes when the client names the interface or the local address. **Scoped evidence:** Tests and tag-claim records apply only to Tested; zero whole-requirement credit. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Re-judge 2026-10-12 (independent auditor, not the author of the {partial} marker) after the marker was added. RFC 5882 Section 4.4 re-read in rfc/full/rfc5882.txt: 'If multiple control protocols wish to establish BFD sessions with the same remote system for the same data protocol, all MUST share a single BFD session.' Verdict partial. Tested selector: "all MUST share a single BFD session". Gap selector: "with the same remote system". Producer: internal/component/bfd/engine/shared_session.go::sharedEntryLocked (with sharesSession, which it calls). What the tagged bodies check, each with a current verified revert record: TestBFDSharedSessionSameKey (one key, two EnsureSession calls: exactly 1 session at refcount 2, refcount 1 after one release, 0 sessions after the last); TestBFDDistinctSessionsDifferentKey (negative: two peers, 2 sessions each at refcount 1); TestCanonicalCollapsesEveryClientShapeOntoOneKey, TestCanonicalCollapsesMultiHopShapesOntoOneKey, TestPinnedSessionReachesTheSharedKey, TestStrictPeerRequestReachesTheSharedKey, TestOSPFNeighborRequestReachesTheSharedKey (the pinned parser, bfdRequestFor and bfdRequestForNeighbor reach one exact key, asserted by equality, so they share through the exact-key registry); TestMultiHopClientsInAVRFShareOneSession (pinned multi-hop entry with a local address in VRF red, then a BGP request with none through pluginService.EnsureSession: 1 session at refcount 2); TestMultiHopClientsToTwoRemoteSystemsGetTwoSessions (negative: a different peer that matches on every other field gets its own session); TestSharedSessionWidensOnRelease (after a pinned client joins and leaves, entry.joined is the unpinned key again and a client pinned elsewhere gets the same session, 1 session at refcount 2; with the pinned client still attached the other pinned client gets its own, RFC 5883 Section 4.1). Q1 the assertions are session counts and refcounts, which is what the RFC says, not what the code does. Q2 every unit has an observed-red revert record (./le rfc discriminate stem rfc5882: none stale, none unproven). Q3 the engine tests use bare loops and Snapshot, nothing else can cut the count. Q4 not the whole MUST: the two configurations the Gap names still give two sessions by design, read at the producer: sharesSession requires equal interfaces for an IPv6 link-local peer, and sharedEntryLocked answers no session when a request with an unset field matches two or more live sessions (sharedMatch.only). Marker corrected in this re-judge: the earlier Tested reason said the join holds 'in either arrival order' and for 'a request that left a field unset' in general; the tagged bodies check only the multi-hop local-address case, pinned entry first, and no tagged unit drives a single-hop request with an unset interface or local address through sharedEntryLocked, so the reason now names exactly the shapes the bodies check. Not covered by the Gap and not judged here: two clients asking for different hop modes to one peer address get two sessions (sharesSession compares Mode); under the partial rules that obligation stays unproven, not met. |
| `RFC5882-10.1.3-1` | If it is desired to use BFD for failure detection of OSPF Virtual Links, the mechanism described in [BFD-MULTI] MUST be used, since OSPF Virtual Links may traverse an arbitrary number of hops. (§10.1.3) | MUST | 10.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not run BFD on OSPF virtual links. The BFD-for-OSPF client opens a session only for a neighbor whose interface carries an explicit per-interface BFD config (internal/plugins/ospf/bfd_client.go:136-148, interfaceBFDConfig(snap.Interface) must be present and Enabled) and requests a single-hop session (bfd_client.go:132). A virtual link is a synthetic backbone link keyed by (transit area, neighbor) (internal/plugins/ospf/instance.go:62-67), not a configured interface, so a virtual-link neighbor never matches an interfaceBFDConfig entry and never gets a BFD session. With no OSPF-vlink BFD session to originate, the "must use the RFC 5883 multihop mechanism" requirement has no applicable code path in Ze |
| `RFC5882-2-1` | An implementation SHOULD establish only a single BFD session per data protocol path, regardless of the number of applications that wish to utilize it. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.1-1` | If the BFD session does not return to Up state within that time frame, the clients SHOULD be notified that a session failure has occurred. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.2-1` | a system SHOULD NOT indicate a connectivity failure to a client if either the local session state or the remote session state (if known) transitions to AdminDown, so long as that client has independent means of liveness detection (typically, control protocols). (§3.2) | SHOULD NOT | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.2-2` | If a client does not have any independent means of liveness detection, a system SHOULD indicate a connectivity failure to a client, and assume the semantics of Down state, if either the local or remote session state transitions to AdminDown. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.3-1` | The BFD state machine transitions that occur in the process of bringing up a BFD session in such situations SHOULD NOT cause a connectivity failure notification to the clients. (§3.3) | SHOULD NOT | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.1-2` | the establishment of control protocol adjacencies SHOULD be blocked if both systems are willing to establish a BFD session but a BFD session cannot be established. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.1-3` | If it is believed that the neighboring system does not support BFD, the establishment of a control protocol adjacency SHOULD NOT be blocked. (§4.1) | SHOULD NOT | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2-1` | If a BFD session transitions from Up state to AdminDown, or the session transitions from Up to Down because the remote system is indicating that the session is in state AdminDown, clients SHOULD NOT take any control protocol action. (§4.2) | SHOULD NOT | 4.2 | **positive:** `unit/verify` [`TestBFDClientAdminDownDoesNotTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_peer_bfd_test.go#L313). **positive:** `unit/verify` [`TestBFDRemoteAdminDownDoesNotTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_peer_bfd_test.go#L477). **positive:** `unit/verify` [`TestOSPFBFDAdminDownTakesNoAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5882_bfd_client_test.go#L424). **positive:** `unit/verify` [`TestOSPFBFDRemoteAdminDownTakesNoAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5882_bfd_client_test.go#L441). **positive:** `unit/verify` [`TestRFC5882RemoteAdminDownIsDistinguished`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_admindown_test.go#L70). **negative:** `unit/verify` [`TestRFC5882NeighborDownIsNotRemoteAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_admindown_test.go#L97) |
| `RFC5882-4.2.1-1` | when a BFD session transitions from Up to Down, action SHOULD be taken in the control protocol to signal the lack of connectivity for the path over which BFD is running. (§4.2.1) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2.1-2` | If the control protocol has an explicit mechanism for announcing path state, a system SHOULD use that mechanism rather than impacting the connectivity of the control protocol, particularly if the control protocol operates out-of-band from the failed data protocol. (§4.2.1) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2.1-3` | if such a mechanism is not available, a control protocol timeout SHOULD be emulated for the associated neighbor. (§4.2.1) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2.1-4` | A control protocol that is tightly bound to a single failing data protocol SHOULD take action to ensure that data traffic is no longer directed to the failing path. (§4.2.1) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2.2.1-1` | when a BFD session transitions from Up to Down, action SHOULD be taken in the control protocol to signal the lack of connectivity for the path in the topology corresponding to the BFD session. (§4.2.2.1) | SHOULD | 4.2.2.1 | **positive:** `unit/verify` [`TestBFDClient_TeardownOnDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_peer_bfd_test.go#L252). **negative:** no negative test. **{single-polarity}:** the sentence obliges Ze to act on an Up to Down transition, and the non-conformant outcome is Ze doing nothing, which is no input a peer can send for Ze to refuse. The no-action boundary, an Up to AdminDown transition, is RFC5882-4.2-1 and carries its own tests. The client that carries several data protocols is MP-BGP, and the BGP client ends the session on BFD Down (internal/component/bgp/reactor/session_bfd_strict.go, bfdTeardown) |
| `RFC5882-4.2.2.1-2` | If this cannot be signaled otherwise, a control protocol timeout SHOULD be emulated for the associated neighbor. (§4.2.2.1) | SHOULD | 4.2.2.1 | **positive:** `unit/verify` [`TestBFDClient_TeardownOnDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_peer_bfd_test.go#L257). **negative:** no negative test. **{single-polarity}:** the obligation is an action Ze takes on BFD Down, and its failure is inaction, which no peer input can present for refusal. BGP has no mechanism to announce one path's state short of the session, so Ze emulates the Hold Timer expiry: NOTIFICATION, then Idle, with the RFC 9384 Cease subcode 10 as the code (bfdTeardown) |
| `RFC5882-4.2.2.2-1` | when a BFD session transitions from Up to Down, action SHOULD be taken in the control protocol to signal the lack of connectivity for the path in the topology over which BFD is running. (§4.2.2.2) | SHOULD | 4.2.2.2 | **positive:** `unit/verify` [`TestBFDClient_TeardownOnDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_peer_bfd_test.go#L261). **negative:** no negative test. **{single-polarity}:** the obligation is an action on an Up to Down transition, and its failure is inaction, which no input can present for refusal. Ze ends the whole BGP session, which signals the loss in the topology BFD runs over and in every other family the session carries. §4.2.2.2 states no obligation to spare the other topologies ("Generally, this can be done without impacting the connectivity of other topologies" is descriptive), and §10.2 asks that the EBGP session "should be torn down in accordance with Section 3.2" |
| `RFC5882-4.3.1-1` | If BFD is implemented in the forwarding plane and does not share fate with the control plane on either system (the "C" bit is set in the BFD Control packets in both directions), control protocol restarts should not affect the BFD session. In this case, a BFD session failure implies that data can no longer be forwarded, so any Graceful Restart in progress at the time of the BFD session failure SHOULD be aborted in order to avoid black holes, and a topology change SHOULD be signaled in the control protocol. (§4.3.1) | SHOULD | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2-1` | If BFD shares fate with the control plane on either system (the "C" bit is clear in either direction), a BFD session failure cannot be disentangled from other events taking place in the control plane. In many cases, the BFD session will fail as a side effect of the restart taking place. As such, it would be best to avoid aborting any Graceful Restart taking place, if possible (since otherwise BFD and Graceful Restart cannot coexist). (§4.3.2) | SHOULD NOT | 4.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2.1-1` | Some control protocols can signal a planned restart prior to the restart taking place. In this case, if a BFD session failure occurs during the restart, such a planned restart SHOULD NOT be aborted and the session failure SHOULD NOT result in a topology change being signaled in the control protocol. (§4.3.2.1) | SHOULD NOT | 4.3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2.2-1` | Control protocols that cannot signal a planned restart depend on the recently restarted system to signal the Graceful Restart prior to the control protocol adjacency timeout. In most cases, whether the restart is planned or unplanned, it is likely that the BFD session will time out prior to the onset of Graceful Restart, in which case a topology change SHOULD be signaled in the control protocol as specified in Section 3.2. (§4.3.2.2) | SHOULD | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2.2-2` | The restarting system SHOULD NOT send any BFD Control packets until there is a high likelihood that its neighbors know a Graceful Restart is taking place (§4.3.2.2) | SHOULD NOT | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-5-1` | If it is known, or presumed, that the remote system is BFD capable and the BFD session is not in Up state, appropriate action SHOULD be taken (such as withdrawing a static route). (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-5-2` | If it is known, or presumed, that the remote system does not support BFD, action such as withdrawing a static route SHOULD NOT be taken. (§5) | SHOULD NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-8-1` | If a planned outage is to take place on a path over which BFD is run, it is preferable to take down the BFD session by going into AdminDown state prior to the outage. The system asserting AdminDown SHOULD do so for at least one Detection Time in order to ensure that the remote system is aware of it. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-8-2` | the system on which BFD is being deconfigured SHOULD put the session into AdminDown state and maintain this state for a Detection Time to ensure that the remote system is aware of it. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-10.1.3-2` | since OSPF Virtual Links may traverse an arbitrary number of hops. BFD authentication SHOULD be used and is strongly encouraged. (§10.1.3) | SHOULD | 10.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-10.2-1` | BFD authentication SHOULD be used and is strongly encouraged. (§10.2) | SHOULD | 10.2 | **positive:** `unit/verify` [`TestRFC5882AuthJoinIdenticalAuthShares`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_auth_join_test.go#L235). **positive:** `unit/verify` [`TestRFC5882AuthenticatedOrIBGPProfileDoesNotWarn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_bfd_auth_warn_test.go#L88). **positive:** `unit/verify` [`TestRFC5882BGPPeerAuthProfileSignsItsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_bgp_auth_test.go#L165). **positive:** `unit/verify` [`TestRFC5882EBGPPeerNamesAuthenticatedProfile`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_bfd_auth_test.go#L25). **positive:** `unit/verify` [`TestRFC5882IBGPPeerWithoutProfileDoesNotWarn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_bfd_auth_warn_test.go#L142). **negative:** `unit/verify` [`TestRFC5882AuthJoinDifferentAuthRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_auth_join_test.go#L182). **negative:** `unit/verify` [`TestRFC5882BGPPeerAuthProfileRefusesMismatchedNeighbor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_bgp_auth_test.go#L207). **negative:** `unit/verify` [`TestRFC5882BGPPeerAuthProfileRefusesUnauthenticatedNeighbor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_bgp_auth_test.go#L227). **negative:** `unit/verify` [`TestRFC5882BGPPeerProfileWithoutAuthRunsUnauthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_bgp_auth_test.go#L246). **negative:** `unit/verify` [`TestRFC5882EBGPPeerWithoutProfileWarns`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_bfd_auth_warn_test.go#L115). **negative:** `unit/verify` [`TestRFC5882EBGPUnauthenticatedProfileWarns`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_bfd_auth_warn_test.go#L64) |
| `RFC5882-3.1-2` | a system MAY choose not to notify clients if a BFD session transitions from Up to Down state, and returns to Up state, if it does so within a reasonable period of time (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.3-2` | A client that is capable of establishing its state prior to the configuration or restarting of a BFD session MAY do so if appropriate. (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2.2-3` | if the restart is in fact planned, an implementation MAY adjust the BFD session timing parameters prior to restarting in such a way that the Detection Time in each direction is longer than the restart period of the control protocol (§4.3.2.2) | MAY | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5882-4.1-1`](#rfc5882-4.1-1) If the session state on either the local or remote system (if known) is AdminDown, BFD has been administratively disabled, and the establishment of a control protocol adjacency MUST be allowed. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: Ze never conditions control-protocol adjacency establishment on BFD session state. The BFD client is attached only AFTER the adjacency is already up -- BGP starts it on StateEstablished (internal/component/bgp/reactor/peer_bfd.go:51-52,61-73) and OSPF opens the session only when a neighbor reaches Full (internal/plugins/ospf/bfd_client.go:124-138, onNeighborFull -> bfdNeighborFull) -- and it is strictly additive: a failure detector, never a bring-up gate (peer_bfd.go:59-60, bfd_client.go:12-13). With no BFD-gated establishment path anywhere in Ze, a session in AdminDown (or any state) cannot block establishment, so this AdminDown carve-out to establishment-blocking has no applicable code path |
| [`RFC5882-4.4-1`](#rfc5882-4.4-1) If multiple control protocols wish to establish BFD sessions with the same remote system for the same data protocol, all MUST share a single BFD session. (§4.4) | {partial}, Partial proof; remaining gap | tested "all MUST share a single BFD session"; gap "with the same remote system"; internal/component/bfd/engine/shared_session.go::sharedEntryLocked is the boundary: Tested covers the BGP request builder, the OSPF request builder and the pinned-session parser reaching one canonical key for one neighbor, two requests on one key sharing one session at refcount 2 that lives until the last release, a multi-hop BGP request with no local address joining the pinned session to the same peer in a VRF at refcount 2, a session opened by an unpinned multi-hop client answering for the unpinned key again after a pinned client leaves so that a client pinned to another local address joins it, and a request to a different remote system getting its own session; Gap is the two configurations in which Ze cannot tell from the request that two clients name the same remote system, so they still get two sessions: an IPv6 link-local peer whose client names no interface (RFC 4007 Section 6: the address alone does not name one system), and a client that left a field unset while two or more live sessions to that peer differ in that field (it could mean either, so it is not merged by a guess); each closes when the client names the interface or the local address |
| [`RFC5882-10.1.3-1`](#rfc5882-10.1.3-1) If it is desired to use BFD for failure detection of OSPF Virtual Links, the mechanism described in [BFD-MULTI] MUST be used, since OSPF Virtual Links may traverse an arbitrary number of hops. (§10.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not run BFD on OSPF virtual links. The BFD-for-OSPF client opens a session only for a neighbor whose interface carries an explicit per-interface BFD config (internal/plugins/ospf/bfd_client.go:136-148, interfaceBFDConfig(snap.Interface) must be present and Enabled) and requests a single-hop session (bfd_client.go:132). A virtual link is a synthetic backbone link keyed by (transit area, neighbor) (internal/plugins/ospf/instance.go:62-67), not a configured interface, so a virtual-link neighbor never matches an interfaceBFDConfig entry and never gets a BFD session. With no OSPF-vlink BFD session to originate, the "must use the RFC 5883 multihop mechanism" requirement has no applicable code path in Ze |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5882-4.1-1`](#rfc5882-4.1-1)

If the session state on either the local or remote system (if known) is AdminDown, BFD has been administratively disabled, and the establishment of a control protocol adjacency MUST be allowed. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5882-4.1-1, so no unit is bound to it.

### [`RFC5882-4.4-1`](#rfc5882-4.4-1)

If multiple control protocols wish to establish BFD sessions with the same remote system for the same data protocol, all MUST share a single BFD session. (§4.4)

Scoped tag-claim records only: tested "all MUST share a single BFD session"; gap "with the same remote system"; internal/component/bfd/engine/shared_session.go::sharedEntryLocked is the boundary: Tested covers the BGP request builder, the OSPF request builder and the pinned-session parser reaching one canonical key for one neighbor, two requests on one key sharing one session at refcount 2 that lives until the last release, a multi-hop BGP request with no local address joining the pinned session to the same peer in a VRF at refcount 2, a session opened by an unpinned multi-hop client answering for the unpinned key again after a pinned client leaves so that a client pinned to another local address joins it, and a request to a different remote system getting its own session; Gap is the two configurations in which Ze cannot tell from the request that two clients name the same remote system, so they still get two sessions: an IPv6 link-local peer whose client names no interface (RFC 4007 Section 6: the address alone does not name one system), and a client that left a field unset while two or more live sessions to that peer differ in that field (it could mean either, so it is not merged by a guess); each closes when the client names the interface or the local address; zero whole-requirement credit.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Re-judge 2026-10-12 (independent auditor, not the author of the {partial} marker) after the marker was added. RFC 5882 Section 4.4 re-read in rfc/full/rfc5882.txt: 'If multiple control protocols wish to establish BFD sessions with the same remote system for the same data protocol, all MUST share a single BFD session.' Verdict partial. Tested selector: "all MUST share a single BFD session". Gap selector: "with the same remote system". Producer: internal/component/bfd/engine/shared_session.go::sharedEntryLocked (with sharesSession, which it calls). What the tagged bodies check, each with a current verified revert record: TestBFDSharedSessionSameKey (one key, two EnsureSession calls: exactly 1 session at refcount 2, refcount 1 after one release, 0 sessions after the last); TestBFDDistinctSessionsDifferentKey (negative: two peers, 2 sessions each at refcount 1); TestCanonicalCollapsesEveryClientShapeOntoOneKey, TestCanonicalCollapsesMultiHopShapesOntoOneKey, TestPinnedSessionReachesTheSharedKey, TestStrictPeerRequestReachesTheSharedKey, TestOSPFNeighborRequestReachesTheSharedKey (the pinned parser, bfdRequestFor and bfdRequestForNeighbor reach one exact key, asserted by equality, so they share through the exact-key registry); TestMultiHopClientsInAVRFShareOneSession (pinned multi-hop entry with a local address in VRF red, then a BGP request with none through pluginService.EnsureSession: 1 session at refcount 2); TestMultiHopClientsToTwoRemoteSystemsGetTwoSessions (negative: a different peer that matches on every other field gets its own session); TestSharedSessionWidensOnRelease (after a pinned client joins and leaves, entry.joined is the unpinned key again and a client pinned elsewhere gets the same session, 1 session at refcount 2; with the pinned client still attached the other pinned client gets its own, RFC 5883 Section 4.1). Q1 the assertions are session counts and refcounts, which is what the RFC says, not what the code does. Q2 every unit has an observed-red revert record (./le rfc discriminate stem rfc5882: none stale, none unproven). Q3 the engine tests use bare loops and Snapshot, nothing else can cut the count. Q4 not the whole MUST: the two configurations the Gap names still give two sessions by design, read at the producer: sharesSession requires equal interfaces for an IPv6 link-local peer, and sharedEntryLocked answers no session when a request with an unset field matches two or more live sessions (sharedMatch.only). Marker corrected in this re-judge: the earlier Tested reason said the join holds 'in either arrival order' and for 'a request that left a field unset' in general; the tagged bodies check only the multi-hop local-address case, pinned entry first, and no tagged unit drives a single-hop request with an unset interface or local address through sharedEntryLocked, so the reason now names exactly the shapes the bodies check. Not covered by the Gap and not judged here: two clients asking for different hop modes to one peer address get two sessions (sharesSession compares Mode); under the partial rules that obligation stays unproven, not met.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBFDDistinctSessionsDifferentKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L73) | unit/verify | revert, verified |
| negative | [`TestMultiHopClientsToTwoRemoteSystemsGetTwoSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_vrf_shared_key_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestCanonicalCollapsesEveryClientShapeOntoOneKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/api/rfc5882_session_identity_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestCanonicalCollapsesMultiHopShapesOntoOneKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/api/rfc5882_session_identity_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestBFDSharedSessionSameKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L18) | unit/verify | revert, verified |
| positive | [`TestSharedSessionWidensOnRelease`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestPinnedSessionReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_session_identity_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestMultiHopClientsInAVRFShareOneSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_vrf_shared_key_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestStrictPeerRequestReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestOSPFNeighborRequestReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5882_shared_key_test.go#L16) | unit/verify | revert, verified |

### [`RFC5882-10.1.3-1`](#rfc5882-10.1.3-1)

If it is desired to use BFD for failure detection of OSPF Virtual Links, the mechanism described in [BFD-MULTI] MUST be used, since OSPF Virtual Links may traverse an arbitrary number of hops. (§10.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5882-10.1.3-1, so no unit is bound to it.

### [`RFC5882-4.2-1`](#rfc5882-4.2-1)

If a BFD session transitions from Up state to AdminDown, or the session transitions from Up to Down because the remote system is indicating that the session is in state AdminDown, clients SHOULD NOT take any control protocol action. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-12 (independent auditor). SHIFTED by 6e063fb498 (tests: give ten assert-nothing tests a real assertion), whose hunks sit in untagged sibling tests and the import block. The tagged unit is byte-identical (gate-verified) and no helper it calls changed. Verdict stands. Judge rejudge 2026-09-30 (bfd AdminDown, OSPF client). D-8 fixed: engine.runBFDSubscriber (internal/plugins/ospf/bfd_client.go, shared by OSPFv2 and v3) no longer declares the neighbor down on StateAdminDown or on a Down with RemoteAdminDown; the 4.2 quote sits above the statement. + TestOSPFBFDAdminDownTakesNoAction and + TestOSPFBFDRemoteAdminDownTakesNoAction (Up, change, Up; later Up read, neighbor not down, no release, session_down_total 0; arm 2 carries an in-body control: the same Down without the flag still declares the neighbor down). TestOSPFBFDAdminDownTreatedAsDown, which asserted the defect and carried no tag, replaced (D-15). Judge overlays: the old condition reds both new tests; removing only the RemoteAdminDown guard reds only arm 2. BGP arms and engine +/- unchanged (see previous verdict). Every client with independent liveness (BGP hold timer, OSPF RouterDeadInterval) now meets the SHOULD; static routes have none, so treating AdminDown as Down is correct under Section 3.2. Records: revert ospf runBFDSubscriber observed red for both units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5882NeighborDownIsNotRemoteAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_admindown_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestRFC5882RemoteAdminDownIsDistinguished`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_admindown_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestBFDClientAdminDownDoesNotTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_peer_bfd_test.go#L313) | unit/verify | revert, verified |
| positive | [`TestBFDRemoteAdminDownDoesNotTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_peer_bfd_test.go#L477) | unit/verify | revert, verified |
| positive | [`TestOSPFBFDAdminDownTakesNoAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5882_bfd_client_test.go#L424) | unit/verify | revert, verified |
| positive | [`TestOSPFBFDRemoteAdminDownTakesNoAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5882_bfd_client_test.go#L441) | unit/verify | revert, verified |

### [`RFC5882-4.2.2.1-1`](#rfc5882-4.2.2.1-1)

when a BFD session transitions from Up to Down, action SHOULD be taken in the control protocol to signal the lack of connectivity for the path in the topology corresponding to the BFD session. (§4.2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-12 (independent auditor). SHIFTED by 6e063fb498 (tests: give ten assert-nothing tests a real assertion), whose hunks sit in untagged sibling tests and the import block. The tagged unit is byte-identical (gate-verified) and no helper it calls changed. Verdict stands. Judge rejudge 2026-09-30 (last five). + reactor TestBFDClient_TeardownOnDown: a non-initial BFD Down StateChange (DiagControlDetectExpired) on an Established peer puts Cease / BFD Down on the wire and moves the session to Idle; asserts the wire, not the opQueue. Producer Peer.runBFDSubscriber -> Session.bfdTeardown (Established arm). MP-BGP is the multi-data-protocol client; ending the session considers the path failed for every family, which is the shared-topology action. {single-polarity: positive} accepted: the obligation is to act on Up->Down and its violation is inaction, which no input presents for refusal; the only no-action boundaries are Up->AdminDown / remote AdminDown (RFC5882-4.2-1, enforced on its own tags, a neighbour row) and the Initial snapshot (not a transition), neither violates this row; no negative existed at HEAD. Record: bfdTeardown revert observed red; author's targeted overlay (Established arm off) red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBFDClient_TeardownOnDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_peer_bfd_test.go#L252) | unit/verify | revert, verified |

### [`RFC5882-4.2.2.1-2`](#rfc5882-4.2.2.1-2)

If this cannot be signaled otherwise, a control protocol timeout SHOULD be emulated for the associated neighbor. (§4.2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-12 (independent auditor). SHIFTED by 6e063fb498 (tests: give ten assert-nothing tests a real assertion), whose hunks sit in untagged sibling tests and the import block. The tagged unit is byte-identical (gate-verified) and no helper it calls changed. Verdict stands. Judge rejudge 2026-09-30 (last five). + TestBFDClient_TeardownOnDown: BGP has no per-path announcement short of the session, so Ze emulates the hold-timer outcome: NOTIFICATION then Idle, with RFC 9384 Cease subcode 10 as the code (msg[19]/msg[20] asserted, state polled to Idle). {single-polarity: positive} accepted: action-on-event SHOULD, failure is inaction, no refusal path; the AdminDown no-action case is RFC5882-4.2-1's. Record: bfdTeardown revert observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBFDClient_TeardownOnDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_peer_bfd_test.go#L257) | unit/verify | revert, verified |

### [`RFC5882-4.2.2.2-1`](#rfc5882-4.2.2.2-1)

when a BFD session transitions from Up to Down, action SHOULD be taken in the control protocol to signal the lack of connectivity for the path in the topology over which BFD is running. (§4.2.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-12 (independent auditor). SHIFTED by 6e063fb498 (tests: give ten assert-nothing tests a real assertion), whose hunks sit in untagged sibling tests and the import block. The tagged unit is byte-identical (gate-verified) and no helper it calls changed. Verdict stands. Judge rejudge 2026-09-30 (last five). Earlier weak note said Ze acts on the whole session rather than one topology. Re-read §4.2.2.2: the SHOULD is 'action SHOULD be taken in the control protocol to signal the lack of connectivity for the path in the topology over which BFD is running'; the next sentence 'Generally, this can be done without impacting the connectivity of other topologies' carries no keyword and states feasibility, not an obligation to spare the others; §10.2 asks the EBGP session 'should be torn down in accordance with Section 3.2'. Ending the BGP session signals the loss in the BFD topology, so the author's reading holds. + TestBFDClient_TeardownOnDown (Cease/BFD Down on the wire, Idle). {single-polarity: positive} accepted for the same reason as 4.2.2.1-1. Record: bfdTeardown revert observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBFDClient_TeardownOnDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_peer_bfd_test.go#L261) | unit/verify | revert, verified |

### [`RFC5882-10.2-1`](#rfc5882-10.2-1)

BFD authentication SHOULD be used and is strongly encouraged. (§10.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-10-11 (independent re-judge after ef70c19e79 changed TestRFC5882AuthJoinDifferentAuthRefused; judge authored none of it). Row quotes Section 10.2 verbatim (rfc5882.txt 'BFD authentication SHOULD be used and is strongly encouraged.'). Ze's part: let an EBGP-advised BFD session run authenticated, hold it to that, and name the unauthenticated case. Unchanged units re-read against the 2026-09-30 judgement: + SignsItsSession (A bit, Auth Type 4, Len 28, Key ID 7, Up), - MismatchedNeighbor and - UnauthenticatedNeighbor (never Up, no fallback), - WithoutAuthRunsUnauthenticated; reactor + NamesAuthenticatedProfile and the four commit Warn units. Changed unit - AuthJoinDifferentAuthRefused now runs seven cases on both join arms (exact key, shared join): auth->none, none->auth, key id, secret, Auth Type alone (keyed-md5 vs keyed-sha1), keyed-sha1 then meticulous-keyed-sha1, and the Meticulous flag alone (resolved 'secure' auth with Meticulous inverted, sent under no profile because authTypeFromEnum ties each meticulous enum to its own wire Auth Type). Each asserts errors.Is ErrAuthMismatch, peer named, one session, first session's next Control packet keeps its auth or none. The earlier caveat (Type and Meticulous comparisons in engine sameAuth unexercised) is closed: judge re-ran go test -overlay with each comparison deleted; the Type mutant reddens only different_auth_type on both arms, the Meticulous mutant only different_meticulous_flag on both arms; clean run green (-race). Discrimination record for the negative (revert of joinAuthCheck) re-written at unit-sha 15bc152837a4dfa6; it proves the refusal path, the overlay mutants prove the field comparisons. + AuthJoinIdenticalAuthShares unchanged (same auth under two names; none twice). Remaining limit: the Meticulous-alone case is a request no profile can produce, so it guards the comparison rather than a configured path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5882AuthJoinDifferentAuthRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_auth_join_test.go#L182) | unit/verify | revert, verified |
| negative | [`TestRFC5882BGPPeerAuthProfileRefusesMismatchedNeighbor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_bgp_auth_test.go#L207) | unit/verify | revert, verified |
| negative | [`TestRFC5882BGPPeerAuthProfileRefusesUnauthenticatedNeighbor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_bgp_auth_test.go#L227) | unit/verify | revert, verified |
| negative | [`TestRFC5882BGPPeerProfileWithoutAuthRunsUnauthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_bgp_auth_test.go#L246) | unit/verify | revert, verified |
| negative | [`TestRFC5882EBGPPeerWithoutProfileWarns`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_bfd_auth_warn_test.go#L115) | unit/verify | revert, verified |
| negative | [`TestRFC5882EBGPUnauthenticatedProfileWarns`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_bfd_auth_warn_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFC5882AuthJoinIdenticalAuthShares`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_auth_join_test.go#L235) | unit/verify | revert, verified |
| positive | [`TestRFC5882BGPPeerAuthProfileSignsItsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5882_bgp_auth_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestRFC5882EBGPPeerNamesAuthenticatedProfile`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_bfd_auth_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC5882AuthenticatedOrIBGPProfileDoesNotWarn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_bfd_auth_warn_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC5882IBGPPeerWithoutProfileDoesNotWarn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5882_bfd_auth_warn_test.go#L142) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5882.txt |
| Source fingerprint | 010c8613f697b6a9 |
| Record | rfc/extraction/rfc5882.json |
| Mapped sentences | 3 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 0 | walked | not stated |
| `4.2.2` | not stated | 0 | walked | not stated |
| `4.2.2.1` | not stated | 0 | walked | not stated |
| `4.2.2.2` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.3.1` | not stated | 0 | walked | not stated |
| `4.3.2` | not stated | 0 | walked | not stated |
| `4.3.2.1` | not stated | 0 | walked | not stated |
| `4.3.2.2` | not stated | 0 | walked | not stated |
| `4.4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.1.1` | not stated | 0 | walked | not stated |
| `10.1.2` | not stated | 0 | walked | not stated |
| `10.1.3` | not stated | 1 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |
| `10.3` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 5882 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 5882, so its obligations are stated where they were written.
