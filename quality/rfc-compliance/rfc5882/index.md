# RFC 5882 - Generic Application of Bidirectional Forwarding Detection (BFD)

Partial. Every requirement this repository extracted from RFC 5882, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 1 of 1 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 1 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 1 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 1 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 27 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 1 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 27 |
| Gated MUST-level | 3 |
| Obligations that bind Ze | 1 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5882.md` |
| Requirement shard | `rfc/requirements/rfc5882.md` |
| RFC text | `rfc/full/rfc5882.txt` |

## Enrolment

Enrolled: Generic Application of BFD: three MUST-level requirements. RFC5882-4.4-1 (multiple control protocols wanting a BFD session to the same remote/data-protocol MUST share a single BFD session) is met by the refcounted, path-keyed session registry EnsureSession (internal/component/bfd/engine/engine.go:344), keyed by api.Key{Peer,Local,Interface,VRF,Mode} which deliberately excludes timers (internal/component/bfd/api/events.go:155-156); both polarities: TestBFDSharedSessionSameKey proves a second same-Key request bumps the refcount to 2 on ONE session and the session survives until the last release, TestBFDDistinctSessionsDifferentKey proves two different remotes get two distinct sessions. RFC5882-4.1-1 (establishment allowed under AdminDown) is {not-applicable}: Ze never gates control-protocol establishment on BFD state -- the BFD client attaches only after the adjacency is up (BGP on StateEstablished, OSPF on Full) and is strictly additive, so no AdminDown session can block establishment. RFC5882-10.1.3-1 (OSPF virtual links MUST use RFC 5883 multihop) is {not-applicable}: Ze does not run BFD on OSPF virtual links (the BFD-for-OSPF client keys on per-interface config; a virtual link is a synthetic backbone link, not a configured interface, so it never gets a session). The SHOULD/SHOULD-NOT clauses (single session per path, hysteresis notify, AdminDown/bring-up connectivity semantics, GR fate-sharing, static-route withdrawal, planned-outage AdminDown, authentication) and the MAY clauses are not gated.

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
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC5882-4.4-1`](#rfc5882-4.4-1)

**Annotated instead of tested (2):** [`RFC5882-4.1-1`](#rfc5882-4.1-1), [`RFC5882-10.1.3-1`](#rfc5882-10.1.3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5882-4.1-1` | If local or remote session state is AdminDown, establishment of a control protocol adjacency must be allowed (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze never conditions control-protocol adjacency establishment on BFD session state. The BFD client is attached only AFTER the adjacency is already up -- BGP starts it on StateEstablished (internal/component/bgp/reactor/peer_bfd.go:51-52,61-73) and OSPF opens the session only when a neighbor reaches Full (internal/plugins/ospf/bfd_client.go:124-138, onNeighborFull -> bfdNeighborFull) -- and it is strictly additive: a failure detector, never a bring-up gate (peer_bfd.go:59-60, bfd_client.go:12-13). With no BFD-gated establishment path anywhere in Ze, a session in AdminDown (or any state) cannot block establishment, so this AdminDown carve-out to establishment-blocking has no applicable code path |
| `RFC5882-4.4-1` | If multiple control protocols want a BFD session to the same remote system for the same data protocol, all must share a single BFD session (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestBFDSharedSessionSameKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L18). **negative:** `unit/verify` [`TestBFDDistinctSessionsDifferentKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L72) |
| `RFC5882-10.1.3-1` | OSPF Virtual Links: the multihop mechanism (RFC 5883) must be used (§10.1.3) | MUST | 10.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not run BFD on OSPF virtual links. The BFD-for-OSPF client opens a session only for a neighbor whose interface carries an explicit per-interface BFD config (internal/plugins/ospf/bfd_client.go:136-148, interfaceBFDConfig(snap.Interface) must be present and Enabled) and requests a single-hop session (bfd_client.go:132). A virtual link is a synthetic backbone link keyed by (transit area, neighbor) (internal/plugins/ospf/instance.go:62-67), not a configured interface, so a virtual-link neighbor never matches an interfaceBFDConfig entry and never gets a BFD session. With no OSPF-vlink BFD session to originate, the "must use the RFC 5883 multihop mechanism" requirement has no applicable code path in Ze |
| `RFC5882-2-1` | Only a single BFD session should be established per data protocol path, regardless of number of applications (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.1-1` | If BFD session does not recover within the hysteresis window, the client must be notified (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.2-1` | System should not indicate a connectivity failure to a client if session transitions to AdminDown, provided the client has independent liveness detection (§3.2) | SHOULD NOT | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.2-2` | If a client has no independent liveness detection, system should indicate connectivity failure and assume Down semantics on AdminDown (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.3-1` | BFD state machine transitions during bring-up should not cause connectivity failure notification to clients (§3.3) | SHOULD NOT | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.1-2` | Establishment of control protocol adjacencies should be blocked if both systems are willing to establish BFD but the session cannot be established (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.1-3` | Establishment of a control protocol adjacency should not be blocked if the peer is believed not to support BFD (§4.1) | SHOULD NOT | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2-1` | If BFD session transitions from Up to AdminDown, or Down due to remote AdminDown, clients should not take control protocol action (§4.2) | SHOULD NOT | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2.1-1` | When BFD session transitions from Up to Down, action should be taken in the control protocol to signal lack of connectivity (§4.2.1, §4.2.2.1, §4.2.2.2) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2.1-2` | If control protocol has an explicit path-state mechanism, use it rather than impacting control protocol connectivity (§4.2.1) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.2.1-3` | If no explicit mechanism, emulate a control protocol timeout for the associated neighbor (§4.2.1) | SHOULD | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.1-1` | If BFD is fate-independent of control plane (C bit set both directions), Graceful Restart should be aborted on BFD session failure, and topology change should be signaled (§4.3.1) | SHOULD | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2-1` | If BFD shares fate with control plane (C bit clear), BFD session failures during restart should not abort the restart (§4.3.2) | SHOULD NOT | 4.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2.1-1` | During a planned restart with BFD fate-sharing, BFD session failure should not result in a topology change (§4.3.2.1) | SHOULD NOT | 4.3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2.2-1` | For unplanned restarts without planned-restart signaling, if BFD session times out, topology change should be signaled (§4.3.2.2) | SHOULD | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2.2-2` | The restarting system should not send BFD Control packets until neighbors are likely aware of Graceful Restart (§4.3.2.2) | SHOULD NOT | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-5-1` | If peer is BFD capable and session is not Up, appropriate action should be taken (e.g., withdraw static route) (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-5-2` | If peer does not support BFD, action such as withdrawing a static route should not be taken (§5) | SHOULD NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-8-1` | Before a planned outage, take BFD session into AdminDown state for at least one Detection Time (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-8-2` | Before deconfiguring BFD, put session into AdminDown and maintain for at least one Detection Time (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-10.1.3-2` | BFD authentication should be used for OSPF Virtual Links and EBGP sessions (§10.1.3, §10.2) | SHOULD | 10.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.1-2` | A system may choose not to notify clients of rapid Up/Down/Up transitions if recovery occurs within a reasonable period (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-3.3-2` | A client capable of establishing state prior to BFD session configuration may do so (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5882-4.3.2.2-3` | An implementation may adjust BFD timing parameters prior to a planned restart to extend Detection Time (§4.3.2.2) | MAY | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5882-4.1-1`](#rfc5882-4.1-1) If local or remote session state is AdminDown, establishment of a control protocol adjacency must be allowed (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: Ze never conditions control-protocol adjacency establishment on BFD session state. The BFD client is attached only AFTER the adjacency is already up -- BGP starts it on StateEstablished (internal/component/bgp/reactor/peer_bfd.go:51-52,61-73) and OSPF opens the session only when a neighbor reaches Full (internal/plugins/ospf/bfd_client.go:124-138, onNeighborFull -> bfdNeighborFull) -- and it is strictly additive: a failure detector, never a bring-up gate (peer_bfd.go:59-60, bfd_client.go:12-13). With no BFD-gated establishment path anywhere in Ze, a session in AdminDown (or any state) cannot block establishment, so this AdminDown carve-out to establishment-blocking has no applicable code path |
| [`RFC5882-10.1.3-1`](#rfc5882-10.1.3-1) OSPF Virtual Links: the multihop mechanism (RFC 5883) must be used (§10.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not run BFD on OSPF virtual links. The BFD-for-OSPF client opens a session only for a neighbor whose interface carries an explicit per-interface BFD config (internal/plugins/ospf/bfd_client.go:136-148, interfaceBFDConfig(snap.Interface) must be present and Enabled) and requests a single-hop session (bfd_client.go:132). A virtual link is a synthetic backbone link keyed by (transit area, neighbor) (internal/plugins/ospf/instance.go:62-67), not a configured interface, so a virtual-link neighbor never matches an interfaceBFDConfig entry and never gets a BFD session. With no OSPF-vlink BFD session to originate, the "must use the RFC 5883 multihop mechanism" requirement has no applicable code path in Ze |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5882-4.1-1`](#rfc5882-4.1-1)

If local or remote session state is AdminDown, establishment of a control protocol adjacency must be allowed (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5882-4.1-1, so no unit is bound to it.

### [`RFC5882-4.4-1`](#rfc5882-4.4-1)

If multiple control protocols want a BFD session to the same remote system for the same data protocol, all must share a single BFD session (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBFDDistinctSessionsDifferentKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L72) | unit/verify | unproven |
| positive | [`TestBFDSharedSessionSameKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5882_shared_session_test.go#L18) | unit/verify | unproven |

### [`RFC5882-10.1.3-1`](#rfc5882-10.1.3-1)

OSPF Virtual Links: the multihop mechanism (RFC 5883) must be used (§10.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5882-10.1.3-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 5882, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5882, so its obligations are stated where they were written.
