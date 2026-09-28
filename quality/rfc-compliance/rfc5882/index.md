# RFC 5882 - Generic Application of Bidirectional Forwarding Detection (BFD)

Partial. Every requirement this repository extracted from RFC 5882, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 33.3% | 1 of 3 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 3 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 77.8% | 7 of 9 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 28 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 3 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 66.7% | 2 of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 3 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 3 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 2 | of 3 gated MUSTs judged | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 3 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 28 |
| Gated MUST-level | 3 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 9 |
| Tagged units | 9 |
| Recorded audit verdicts | 2 |
| Discrimination records | 7 |
| Summary | `rfc/short/rfc5882.md` |
| Requirement shard | `rfc/requirements/rfc5882.md` |
| RFC text | `rfc/full/rfc5882.txt` |

## Enrolment

Enrolled: Generic Application of BFD: three MUST-level requirements. RFC5882-4.4-1 (multiple control protocols wanting a BFD session to the same remote/data-protocol MUST share a single BFD session) is met for every configuration in which the clients reach one key, and PARTIALLY met otherwise, which is what docs/features/rfc-status.md publishes. The registry half is complete: EnsureSession (internal/component/bfd/engine/engine.go:344) is refcounted and path-keyed on api.Key{Peer,Local,Interface,VRF,Mode}, deliberately excluding timers (internal/component/bfd/api/events.go:155-156), and both polarities are proven by TestBFDSharedSessionSameKey (a second same-Key request bumps the refcount to 2 on ONE session, which survives until the last release) and TestBFDDistinctSessionsDifferentKey (two different remotes get two distinct sessions). The key half, api.SessionRequest.Canonical, completes what a client left out so OSPF, BGP and a pinned session reach the same key, and it REFUSES to derive where the answer is ambiguous: seven configurations, listed under "Multiple Control Protocols (Section 4.4)" on this page, leave two clients for one remote system with two sessions rather than one (and, until the interface-less first-packet lookup landed, left the single-hop four with two sessions NEITHER of which could be selected for a packet carrying no discriminator) (an IPv6 link-local peer, two links on one subnet, an off-link peer, no route, an egress interface with no or several addresses of the peer family, multi-hop in a non-default VRF, and any mode with no interface backend loaded). Each closes when the operator names the local address on both sides. RFC5882-4.1-1 (establishment allowed under AdminDown) is {not-applicable}: Ze never gates control-protocol establishment on BFD state -- the BFD client attaches only after the adjacency is up (BGP on StateEstablished, OSPF on Full) and is strictly additive, so no AdminDown session can block establishment. RFC5882-10.1.3-1 (OSPF virtual links MUST use RFC 5883 multihop) is {not-applicable}: Ze does not run BFD on OSPF virtual links (the BFD-for-OSPF client keys on per-interface config; a virtual link is a synthetic backbone link, not a configured interface, so it never gets a session). RFC5882-4.2-1 (no control protocol action on Up to AdminDown, or on Up to Down caused by the remote system's AdminDown) is MET, at the service boundary and at the client. api.StateChange.RemoteAdminDown carries the neighbor's AdminDown, which RFC 5880 Section 6.8.6 otherwise erases into a local Down with the same diagnostic a peer-signaled Down produces, and both polarities are proven by TestRFC5882RemoteAdminDownIsDistinguished and TestRFC5882NeighborDownIsNotRemoteAdminDown. Those two check what the event SAYS; Peer.runBFDSubscriber (internal/component/bgp/reactor/peer_bfd.go) then reads the field and raises no FSM event for that case, which is the "control protocol action" Section 4.2 names, and TestBFDRemoteAdminDownDoesNotTeardown checks what the client DOES with a control in the same body: the same local state and diagnostic with the neighbor not AdminDown is a path failure and still drops the session. The recording of state and time is deliberately left to run, because the OPEN rail reads it and bfdStrictHolds must still see a session that is not Up. Section 3.2 conditions the obligation on the client having "independent means of liveness detection", and BGP's hold timer is one. The other SHOULD/SHOULD-NOT clauses (single session per path, hysteresis notify, bring-up connectivity semantics, GR fate-sharing, static-route withdrawal, planned-outage AdminDown, authentication) and the MAY clauses are not gated.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

BFD integration model used by BGP and static next-hop tracking.

**What the ledger says remains:**

Same BFD partial status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC5882-4.4-1`](#rfc5882-4.4-1)

**Annotated instead of tested (2):** [`RFC5882-4.1-1`](#rfc5882-4.1-1), [`RFC5882-10.1.3-1`](#rfc5882-10.1.3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5882-4.1-1` | If the session state on either the local or remote system (if known) is AdminDown, BFD has been administratively disabled, and the establishment of a control protocol adjacency MUST be allowed. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze never conditions control-protocol adjacency establishment on BFD session state. The BFD client is attached only AFTER the adjacency is already up -- BGP starts it on StateEstablished (internal/component/bgp/reactor/peer_bfd.go:51-52,61-73) and OSPF opens the session only when a neighbor reaches Full (internal/plugins/ospf/bfd_client.go:124-138, onNeighborFull -> bfdNeighborFull) -- and it is strictly additive: a failure detector, never a bring-up gate (peer_bfd.go:59-60, bfd_client.go:12-13). With no BFD-gated establishment path anywhere in Ze, a session in AdminDown (or any state) cannot block establishment, so this AdminDown carve-out to establishment-blocking has no applicable code path |
| `RFC5882-4.4-1` | If multiple control protocols wish to establish BFD sessions with the same remote system for the same data protocol, all MUST share a single BFD session. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestBFDSharedSessionSameKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L18). **positive:** `unit/verify` [`TestCanonicalCollapsesEveryClientShapeOntoOneKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/api/session_identity_test.go#L59). **positive:** `unit/verify` [`TestCanonicalCollapsesMultiHopShapesOntoOneKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/api/session_identity_test.go#L150). **positive:** `unit/verify` [`TestOSPFNeighborRequestReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5882_shared_key_test.go#L16). **positive:** `unit/verify` [`TestPinnedSessionReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session_identity_test.go#L19). **positive:** `unit/verify` [`TestStrictPeerRequestReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L206). **negative:** `unit/verify` [`TestBFDDistinctSessionsDifferentKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L73) |
| `RFC5882-10.1.3-1` | If it is desired to use BFD for failure detection of OSPF Virtual Links, the mechanism described in [BFD-MULTI] MUST be used, since OSPF Virtual Links may traverse an arbitrary number of hops. (§10.1.3) | MUST | 10.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not run BFD on OSPF virtual links. The BFD-for-OSPF client opens a session only for a neighbor whose interface carries an explicit per-interface BFD config (internal/plugins/ospf/bfd_client.go:136-148, interfaceBFDConfig(snap.Interface) must be present and Enabled) and requests a single-hop session (bfd_client.go:132). A virtual link is a synthetic backbone link keyed by (transit area, neighbor) (internal/plugins/ospf/instance.go:62-67), not a configured interface, so a virtual-link neighbor never matches an interfaceBFDConfig entry and never gets a BFD session. With no OSPF-vlink BFD session to originate, the "must use the RFC 5883 multihop mechanism" requirement has no applicable code path in Ze |
| `RFC5882-2-1` | An implementation SHOULD establish only a single BFD session per data protocol path, regardless of the number of applications that wish to utilize it. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.1-1` | If the BFD session does not return to Up state within that time frame, the clients SHOULD be notified that a session failure has occurred. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.2-1` | a system SHOULD NOT indicate a connectivity failure to a client if either the local session state or the remote session state (if known) transitions to AdminDown, so long as that client has independent means of liveness detection (typically, control protocols). (§3.2) | SHOULD NOT | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.2-2` | If a client does not have any independent means of liveness detection, a system SHOULD indicate a connectivity failure to a client, and assume the semantics of Down state, if either the local or remote session state transitions to AdminDown. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.3-1` | The BFD state machine transitions that occur in the process of bringing up a BFD session in such situations SHOULD NOT cause a connectivity failure notification to the clients. (§3.3) | SHOULD NOT | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.1-2` | the establishment of control protocol adjacencies SHOULD be blocked if both systems are willing to establish a BFD session but a BFD session cannot be established. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.1-3` | If it is believed that the neighboring system does not support BFD, the establishment of a control protocol adjacency SHOULD NOT be blocked. (§4.1) | SHOULD NOT | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2-1` | If a BFD session transitions from Up state to AdminDown, or the session transitions from Up to Down because the remote system is indicating that the session is in state AdminDown, clients SHOULD NOT take any control protocol action. (§4.2) | SHOULD NOT | 4.2 | **positive:** `unit/verify` [`TestRFC5882RemoteAdminDownIsDistinguished`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_admindown_test.go#L70). **negative:** `unit/verify` [`TestRFC5882NeighborDownIsNotRemoteAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_admindown_test.go#L97) |
| `RFC5882-4.2.1-1` | when a BFD session transitions from Up to Down, action SHOULD be taken in the control protocol to signal the lack of connectivity for the path over which BFD is running. (§4.2.1) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2.1-2` | If the control protocol has an explicit mechanism for announcing path state, a system SHOULD use that mechanism rather than impacting the connectivity of the control protocol, particularly if the control protocol operates out-of-band from the failed data protocol. (§4.2.1) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2.1-3` | if such a mechanism is not available, a control protocol timeout SHOULD be emulated for the associated neighbor. (§4.2.1) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2.1-4` | A control protocol that is tightly bound to a single failing data protocol SHOULD take action to ensure that data traffic is no longer directed to the failing path. (§4.2.1) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
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
| `RFC5882-3.1-2` | a system MAY choose not to notify clients if a BFD session transitions from Up to Down state, and returns to Up state, if it does so within a reasonable period of time (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.3-2` | A client that is capable of establishing its state prior to the configuration or restarting of a BFD session MAY do so if appropriate. (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2.2-3` | if the restart is in fact planned, an implementation MAY adjust the BFD session timing parameters prior to restarting in such a way that the Detection Time in each direction is longer than the restart period of the control protocol (§4.3.2.2) | MAY | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5882-4.1-1`](#rfc5882-4.1-1) If the session state on either the local or remote system (if known) is AdminDown, BFD has been administratively disabled, and the establishment of a control protocol adjacency MUST be allowed. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: Ze never conditions control-protocol adjacency establishment on BFD session state. The BFD client is attached only AFTER the adjacency is already up -- BGP starts it on StateEstablished (internal/component/bgp/reactor/peer_bfd.go:51-52,61-73) and OSPF opens the session only when a neighbor reaches Full (internal/plugins/ospf/bfd_client.go:124-138, onNeighborFull -> bfdNeighborFull) -- and it is strictly additive: a failure detector, never a bring-up gate (peer_bfd.go:59-60, bfd_client.go:12-13). With no BFD-gated establishment path anywhere in Ze, a session in AdminDown (or any state) cannot block establishment, so this AdminDown carve-out to establishment-blocking has no applicable code path |
| [`RFC5882-10.1.3-1`](#rfc5882-10.1.3-1) If it is desired to use BFD for failure detection of OSPF Virtual Links, the mechanism described in [BFD-MULTI] MUST be used, since OSPF Virtual Links may traverse an arbitrary number of hops. (§10.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not run BFD on OSPF virtual links. The BFD-for-OSPF client opens a session only for a neighbor whose interface carries an explicit per-interface BFD config (internal/plugins/ospf/bfd_client.go:136-148, interfaceBFDConfig(snap.Interface) must be present and Enabled) and requests a single-hop session (bfd_client.go:132). A virtual link is a synthetic backbone link keyed by (transit area, neighbor) (internal/plugins/ospf/instance.go:62-67), not a configured interface, so a virtual-link neighbor never matches an interfaceBFDConfig entry and never gets a BFD session. With no OSPF-vlink BFD session to originate, the "must use the RFC 5883 multihop mechanism" requirement has no applicable code path in Ze |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5882-4.1-1`](#rfc5882-4.1-1)

If the session state on either the local or remote system (if known) is AdminDown, BFD has been administratively disabled, and the establishment of a control protocol adjacency MUST be allowed. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5882-4.1-1, so no unit is bound to it.

### [`RFC5882-4.4-1`](#rfc5882-4.4-1)

If multiple control protocols wish to establish BFD sessions with the same remote system for the same data protocol, all MUST share a single BFD session. (§4.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden behaviour: two control protocols wanting a session to the same remote system for the same data protocol end up on two BFD sessions. Red assertions: TestBFDSharedSessionSameKey len(snap)!=1 and Refcount!=2 (same request twice); TestCanonicalCollapsesEveryClientShapeOntoOneKey, TestCanonicalCollapsesMultiHopShapesOntoOneKey, TestStrictPeerRequestReachesTheSharedKey, TestOSPFNeighborRequestReachesTheSharedKey and TestPinnedSessionReachesTheSharedKey assert BGP/OSPF/pinned request shapes reduce to one api.Key when the link table resolves one link. Negative TestBFDDistinctSessionsDifferentKey pins that different remotes are not aliased. Not the whole MUST: in the configurations api.SessionRequest.Canonical refuses to derive (IPv6 link-local peer, two links on one subnet, off-link peer, no route, egress with zero or several same-family addresses, multi-hop in a non-default VRF, no interface backend) two clients for one remote get two sessions, no tagged unit goes red on that, and untagged TestCanonicalRefusesToGuessAnAmbiguousLink asserts the two-key behaviour. The summary's Enrolment reason already records this as PARTIALLY met.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBFDDistinctSessionsDifferentKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L73) | unit/verify | unproven |
| positive | [`TestCanonicalCollapsesEveryClientShapeOntoOneKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/api/session_identity_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestCanonicalCollapsesMultiHopShapesOntoOneKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/api/session_identity_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestBFDSharedSessionSameKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L18) | unit/verify | unproven |
| positive | [`TestPinnedSessionReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session_identity_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestStrictPeerRequestReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestOSPFNeighborRequestReachesTheSharedKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5882_shared_key_test.go#L16) | unit/verify | revert, verified |

### [`RFC5882-10.1.3-1`](#rfc5882-10.1.3-1)

If it is desired to use BFD for failure detection of OSPF Virtual Links, the mechanism described in [BFD-MULTI] MUST be used, since OSPF Virtual Links may traverse an arbitrary number of hops. (§10.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5882-10.1.3-1, so no unit is bound to it.

### [`RFC5882-4.2-1`](#rfc5882-4.2-1)

If a BFD session transitions from Up state to AdminDown, or the session transitions from Up to Down because the remote system is indicating that the session is in state AdminDown, clients SHOULD NOT take any control protocol action. (§4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The sentence forbids a client taking control protocol action on (a) its own session going Up to AdminDown and (b) Up to Down caused by the remote's AdminDown. The tagged units TestRFC5882RemoteAdminDownIsDistinguished and TestRFC5882NeighborDownIsNotRemoteAdminDown (internal/component/bfd/engine/rfc5882_admindown_test.go) assert only what the BFD service PUBLISHES for clause (b) (State Down, RemoteAdminDown true vs false, same Diag); no tagged assertion goes red if a client tears down the BGP/OSPF adjacency on either clause, and clause (a) (local AdminDown) has no tagged assertion at all. The client-side behaviour is asserted by untagged TestBFDClientAdminDownDoesNotTeardown and TestBFDRemoteAdminDownDoesNotTeardown (internal/component/bgp/reactor/peer_bfd_test.go); tagging them would close this.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5882NeighborDownIsNotRemoteAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_admindown_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestRFC5882RemoteAdminDownIsDistinguished`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_admindown_test.go#L70) | unit/verify | revert, verified |

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
