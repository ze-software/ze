# RFC 7947 - Internet Exchange BGP Route Server

BGP Route Server. Every requirement this repository extracted from RFC 7947, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 3 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 3 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 3 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 3 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 7 | 7 of 22 tagged units, 0 escaped and 5 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 7 | of 3 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 3 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 3 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 3 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 3 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 1 | of 3 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 3 gated MUSTs: they add up to 3. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP Route Server |
| Enrolment | Enrolled |
| Requirements | 8 |
| Gated MUST-level | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 22 |
| Tagged units | 22 |
| Recorded audit verdicts | 7 |
| Discrimination records | 12 |
| Summary | `rfc/short/rfc7947.md` |
| Requirement shard | `rfc/requirements/rfc7947.md` |
| RFC text | `rfc/full/rfc7947.txt` |

## Enrolment

Enrolled: BGP Route Server: two MUST-level transparency requirements over the reactor-native RS forwarding path (internal/component/bgp/reactor/forward_rs.go), plus the advisory-level RFC7947-x-1, which the gate does not require but which is proven anyway. RFC7947-x-1 (SHOULD NOT prepend own AS for RS clients -- Section 2.2.2.1 calls it "a recommendation rather than a requirement", so it is not MUST-level and the gate does not require it) has both polarities: TestReactorForwardRSTransparent proves an RS client's forwarded body is byte-identical (no prepend), TestReactorForwardRSEBGPPrepend proves a non-RS eBGP peer DOES get the local AS prepended, so the no-prepend is confined to RS clients. Ze deviates from RFC7947-x-1 under one gate only: an RS client is sent its route server's AS prepended when, and only when, the route itself carries the route server's large-community prepend request naming that client or every client (`<rs-asn>:101..103:<asn>`, wireu.CommunityPolicy.RSPrepend, called from both forward rails). Section 2.2.2.1 states the recommendation exists "solely to provide backwards compatibility with legacy route server client implementations", and a route whose sender asks for the prepend has asked for the modification (owner decision, spec-bcp194-1-communities). Without the request the AS_PATH is untouched, which is what TestReactorForwardRSTransparent proves; TestRSPrependReachesWire proves the gated prepend on both rails. RFC7947-x-2 (MUST NOT rewrite NEXT_HOP) is {single-polarity: positive}: the byte-identical transparency test proves it, and it has no natural negative, NEXT_HOP transparency not being RS-specific. RFC7947-x-3 is NOT MUST-level and the gate does not require it: Section 2.2.3 reads "this attribute SHOULD be propagated to other route server clients, and the route server SHOULD NOT modify its value", so the row is [SHOULD] and the earlier MUST here misquoted it. The same transparency test proves Ze meets it anyway: medPropagationAllowedTo (internal/component/bgp/reactor/forward_med.go) never applies the automatic RFC 4271 Section 5.1.4 strip to a route server client. An operator's own `del { med; }` policy still removes the metric upstream of that predicate (test/plugin/med-removal-configured.ci, against an rs-client receiver), which is Section 5.1.4's required removal mechanism rather than an exception to this recommendation. RFC7947-x-4 (per-client import/export policy applied on each redistribution) has both polarities: TestReactorForwardRSFallback proves an export-filtered client is routed to the per-client policy path, TestReactorForwardRSBasic proves an unfiltered client is forwarded directly. RFC7947-x-5 (SHOULD perform one or more BGP Decision Processes) is not MUST-level and the gate does not require it, but it is carried in both polarities: TestRFC7947RouteServerWorseReannouncementYieldsOtherSourceRoute tags the positive (a client without ADD-PATH ends on another source's better route when the source of its route re-announces a worse one) and TestRFC7947RouteServerWorseLaterRouteDoesNotDisplace the negative (a worse route from a second source sends that client nothing). Two more positive carriers cover the configuration the decision takes into account: TestRSAnnouncementAddPathClientGetsEveryPath (a client with ADD-PATH send receives both sources' paths, each under its own Path Identifier, and owes no election, the multiple-path case of Section 2.3.2.2) and TestRSOwnerWithdrawalPolicyDeniedSkipsToNextSurvivor (a survivor the client's export policy denies is stepped past to the next permitted one, the per-client policy of Section 2.3.2.1).

## What the public ledger says

**Area:** BGP Route Server

**Proof:** 2 of 3 MUST proven

**Evidence:** unit 2

**Unmet:** RFC7947-2.1-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated (including scoped evidence) | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC7947-2.2.2.2-1`](#rfc7947-2.2.2.2-1)

**Annotated (including scoped evidence) (2):** [`RFC7947-x-2`](#rfc7947-x-2), [`RFC7947-2.1-1`](#rfc7947-2.1-1)

## Figures

- **MUST bar**: Ze proven 2, Ze correctness gap 1
- **Overview bar**: Ze MUST proven 2, SHOULD proven 3, SHOULD not decided 2, Ze correctness gap 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 3 |
| MUST rows kept (the MUST bar) | 3 |
| Ze proven | 2 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 1 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD proven | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 2 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc7947.json](../../../data/rfc/rfc7947.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7947-x-1` | the route server SHOULD NOT prepend its own AS number to the AS_PATH segment nor modify the AS_PATH segment in any other way. (§2.2.2.1) | SHOULD NOT | 2.2.2.1 - Route Server AS_PATH Management | **positive:** `unit/verify` [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_forward_rs_test.go#L451). **positive:** `unit/verify` [`TestRelayStoredRouteRSClientPreservesASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_reactor_api_relay_test.go#L223). **negative:** `unit/verify` [`TestReactorForwardRSEBGPPrepend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_forward_rs_test.go#L329). **negative:** `unit/verify` [`TestRelayStoredRoutePlainEBGPPrependsLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_reactor_api_relay_test.go#L259). **positive:** `functional/verify` [`bgp-rs-relay-aspath-transparency.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/bgp-rs-relay-aspath-transparency.ci#L4). **negative:** `functional/verify` [`bgp-rs-relay-aspath-transparency.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/bgp-rs-relay-aspath-transparency.ci#L8). **positive:** `interop/nightly` [`checkRouteServerASPath`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1028) |
| `RFC7947-x-2` | As the route server does not participate in the actual routing of traffic, the NEXT_HOP attribute MUST be passed unmodified to the route server clients, similar to the "third-party" next-hop feature described in Section 5.1.3. of [RFC4271]. (§2.2.1) | MUST | 2.2.1 - NEXT_HOP Attribute | **positive:** `unit/verify` [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_forward_rs_test.go#L456). **negative:** no negative test. **{single-polarity}:** NEXT_HOP transparency is not RS-specific -- every forwarded route preserves it by default (nhModeNone, internal/component/bgp/reactor/peer_forward_facts.go:147), so unlike x-1's AS-path prepend there is no "confined" negative where a comparable non-RS peer rewrites NEXT_HOP; the only rewrite is an explicit per-peer next-hop-self/explicit override, which exercises the override feature rather than the RS-transparency MUST-NOT. The positive is proven byte-identical in TestReactorForwardRSTransparent |
| `RFC7947-x-3` | Contrary to Section 5.1.4 of [RFC4271], if applied to an NLRI UPDATE sent to a route server, this attribute SHOULD be propagated to other route server clients, and the route server SHOULD NOT modify its value. (§2.2.3) | SHOULD | 2.2.3 - MULTI_EXIT_DISC Attribute | **positive:** `unit/verify` [`TestForwardKeepsMEDForRouteServerClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L373). **positive:** `unit/verify` [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_forward_rs_test.go#L458). **positive:** `functional/verify` [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L11). **negative:** `functional/verify` [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L15) |
| `RFC7947-x-4` | The route server SHOULD forward UPDATE messages from its Loc-RIB or Loc-RIBs to its clients as determined by local policy. (§2.1) | SHOULD | 2.1 - Client UPDATE Messages | **positive:** `unit/verify` [`TestRFC7947ClientPolicyControlsWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_consumer_test.go#L86). **negative:** `unit/verify` [`TestRFC7947ClientPolicyControlsWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_consumer_test.go#L87) |
| `RFC7947-x-5` | The route server SHOULD perform one or more BGP Decision Processes to select routes for subsequent advertisement to its clients, taking into account possible configuration to provide multiple Network Layer Reachability Information (NLRI) paths to a particular client as described in Section 2.3.2.2 or multiple Loc-RIBs as described in Section 2.3.2.1. (§2.1) | SHOULD | 2.1 - Client UPDATE Messages | **positive:** `unit/verify` [`TestRFC7947RouteServerWorseReannouncementYieldsOtherSourceRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_rs_decision_test.go#L28). **positive:** `unit/verify` [`TestRSAnnouncementAddPathClientGetsEveryPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_rs_announcement_election_test.go#L628). **positive:** `unit/verify` [`TestRSOwnerWithdrawalPolicyDeniedSkipsToNextSurvivor`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_rs_withdrawal_replacement_test.go#L501). **negative:** `unit/verify` [`TestRFC7947RouteServerWorseLaterRouteDoesNotDisplace`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_rs_decision_test.go#L86) |
| `RFC7947-2.2-1` | Therefore, contrary to what is specified in Section 5 of [RFC4271], route servers SHOULD NOT by default (unless explicitly configured) update well-known BGP attributes received from route server clients before redistributing them to their other route server clients. Optional recognized and unrecognized BGP attributes, whether transitive or non-transitive, SHOULD NOT be updated by the route server (unless enforced by local IXP operator configuration) and SHOULD be passed on to other route server clients. (§2.2) | SHOULD NOT | 2.2 - Attribute Transparency | **positive:** `unit/verify` [`TestRFC7947AllAttributesReachClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_consumer_test.go#L33). **negative:** `unit/verify` [`TestRouteServerTransparencyStopsAtOrdinaryPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_consumer_test.go#L55) |
| `RFC7947-2.2.2.2-1` | In contrast to what is recommended in Section 6.3 of [RFC4271], route server clients need to be able to accept UPDATE messages where the leftmost AS in the AS_PATH attribute is not equal to the AS number of the route server that sent the UPDATE message. If the route server client BGP system has implemented a check for this, the BGP implementation MUST allow this check to be disabled and SHOULD allow the check to be disabled on a per-peer basis. (§2.2.2.2) | MUST | 2.2.2.2 - Route Server client AS_PATH Management | **positive:** `unit/verify` [`TestLoopIngressAcceptsNonAdjacentLeftmostAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L345). **negative:** `unit/verify` [`TestLoopIngressRejectsLocalASFromRouteServer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L363) |
| `RFC7947-2.1-1` | A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In. (§2.1) | MUST | 2.1 - Client UPDATE Messages | **positive:** no positive test. **negative:** no negative test. **{gap}:** an UPDATE an ingress filter denies is never stored: internal/component/bgp/reactor/reactor_notify.go::notifyMessageReceiver returns at the ordered ingress step loop's non-accept branch, ahead of the Adj-RIB-In cache add, so a denied UPDATE is absent from show bgp adj-rib-in, and LoopIngress denies with no operator configuration; the fix (store first, mark the entry filtered) is owned by plan/release/fixes/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7947-2.1-1`](#rfc7947-2.1-1) A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In. (§2.1) | {gap}, no test | an UPDATE an ingress filter denies is never stored: internal/component/bgp/reactor/reactor_notify.go::notifyMessageReceiver returns at the ordered ingress step loop's non-accept branch, ahead of the Adj-RIB-In cache add, so a denied UPDATE is absent from show bgp adj-rib-in, and LoopIngress denies with no operator configuration; the fix (store first, mark the entry filtered) is owned by plan/release/fixes/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7947-x-1`](#rfc7947-x-1)

the route server SHOULD NOT prepend its own AS number to the AS_PATH segment nor modify the AS_PATH segment in any other way. (§2.2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7947 Section 2.2.2.1: "As a route server does not participate in the process of forwarding data between client routers, and because modification of the AS_PATH attribute could affect the route server client BGP Decision Process, the route server SHOULD NOT prepend its own AS number to the AS_PATH segment nor modify the AS_PATH segment in any other way." Re-judged 2026-10-31 by an independent auditor after c8a7d38c3f changed TestReactorForwardRSEBGPPrepend and f8f7628bdf changed bgp-rs-relay-aspath-transparency.ci. Both changes are the same RFC 8212 Section 3 fixture repair: the unit's non-RS EBGP destination gains an explicit filterless FinalAccept ExportPolicy (and makeRSPeer gives its source the same import/export pair), and the .ci gains a bgp-level filter { import { final accept; } export { final accept; } }. Without them reactorForwardRSSection skips the destination at filterapi.Policy.AcceptsAtEnd (an unconfigured policy returns !external), so the tests dispatched nothing; the policies carry no refs, so hasActiveFilter stays false and the fast path, and the !facts.rsClient prepend branch it reaches, are unchanged. No assertion moved: the unit still requires one dispatch, non-empty rawBodies and a body longer than the input, and the .ci still pins the exact wire bytes, conn=2 (rs-client true) byte-identical with AS_SEQUENCE[65001] and conn=3 (rs-client false) AS_SEQUENCE[65000,65001]. The unit's growth-only assertion remains weaker than an exact path oracle, as recorded before; the exact pair is supplied by the .ci and by TestRelayStoredRouteRSClientPreservesASPath / TestRelayStoredRoutePlainEBGPPrependsLocalAS, with TestReactorForwardRSTransparent proving byte identity for an RS client and checkRouteServerASPath the interop side, all unchanged. Discrimination record re-taken in c8a7d38c3f over the changed unit and verified by ./le rfc discriminate: mutant forward_rs.go::reactorForwardRSSection line 633 '!facts.rsClient' -> false reddens it. Both carriers re-run green (go test -run TestReactorForwardRSEBGPPrepend|TestReactorForwardRSTransparent; ./le test bgp plugin bgp-rs-relay-aspath-transparency). Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRSEBGPPrepend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_forward_rs_test.go#L329) | unit/verify | mutant, verified |
| negative | [`TestRelayStoredRoutePlainEBGPPrependsLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_reactor_api_relay_test.go#L259) | unit/verify | unproven |
| negative | [`bgp-rs-relay-aspath-transparency.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/bgp-rs-relay-aspath-transparency.ci#L8) | functional/verify | unproven |
| positive | [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_forward_rs_test.go#L451) | unit/verify | unproven |
| positive | [`TestRelayStoredRouteRSClientPreservesASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_reactor_api_relay_test.go#L223) | unit/verify | unproven |
| positive | [`checkRouteServerASPath`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1028) | interop/nightly | unproven |
| positive | [`bgp-rs-relay-aspath-transparency.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/bgp-rs-relay-aspath-transparency.ci#L4) | functional/verify | unproven |

### [`RFC7947-x-2`](#rfc7947-x-2)

As the route server does not participate in the actual routing of traffic, the NEXT_HOP attribute MUST be passed unmodified to the route server clients, similar to the "third-party" next-hop feature described in Section 5.1.3. of [RFC4271]. (§2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7947 Section 2.2.1: "As the route server does not participate in the actual routing of traffic, the NEXT_HOP attribute MUST be passed unmodified to the route server clients, similar to the "third-party" next-hop feature described in Section 5.1.3. of [RFC4271]." The sole current carrier TestReactorForwardRSTransparent forwards a body with NEXT_HOP 10.0.0.254 and reachable NLRI to an RS client and requires complete byte identity. A dropped/changed next-hop fails, and nonempty output excludes the no-forward false positive. Existing single-polarity annotation remains justified for the default transparent path: ordinary forwarding also preserves next-hop unless an explicit policy requests a rewrite; changing that policy would test another feature, not create this rule's confining negative. peerForwardFacts/buildForwardFacts and forwarding producer path were read. One positive cover, zero negative, with the existing annotation rather than a fabricated pair. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_forward_rs_test.go#L456) | unit/verify | mutant, verified |

### [`RFC7947-x-3`](#rfc7947-x-3)

Contrary to Section 5.1.4 of [RFC4271], if applied to an NLRI UPDATE sent to a route server, this attribute SHOULD be propagated to other route server clients, and the route server SHOULD NOT modify its value. (§2.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc7947): "Contrary to Section 5.1.4 of [RFC4271], if applied to an NLRI UPDATE sent to a route server, this attribute SHOULD be propagated to other route server clients, and the route server SHOULD NOT modify its value." The only edit to test/plugin/med-not-propagated-across-as.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L15) | functional/verify | unproven |
| positive | [`TestForwardKeepsMEDForRouteServerClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L373) | unit/verify | unproven |
| positive | [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_forward_rs_test.go#L458) | unit/verify | unproven |
| positive | [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L11) | functional/verify | unproven |

### [`RFC7947-x-4`](#rfc7947-x-4)

The route server SHOULD forward UPDATE messages from its Loc-RIB or Loc-RIBs to its clients as determined by local policy. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of faf7c5c380. RFC 7947 Section 2.1: "The route server SHOULD forward UPDATE messages from its Loc-RIB or Loc-RIBs to its clients as determined by local policy." faf7c5c380 (filter chain model moved into internal/component/policy) changed one line in the tagged unit internal/component/bgp/reactor/rfc7947_consumer_test.go::TestRFC7947ClientPolicyControlsWire: mustPolicy(t, []filterapi.FilterRef{{Name: "client-policy"}}, filterapi.FinalAccept) became mustPolicy(t, []routepolicy.Ref{{Name: "client-policy"}}, routepolicy.FinalAccept). mustPolicy still calls filterapi.NewPolicy with the same single unbound ref and the explicit accept final (NewPolicy now wraps policy.NewChain; an unbound ref stays external as the zero inProcess field did), so the policy the fixture installs, every input and every assertion are unchanged. No other commit touched the file since the verdict was stamped. be95c10053 changed sibling units in the same file (acceptingLiveSettings), not this one. The unit still requires no policy-agnostic send from reactorForwardRS, exactly one export policy call per client, one UPDATE per client with the exact NLRI and ORIGIN for the accepting client and only the exact withdrawal for the rejecting one, with the accepting client swapped between rounds. Verdict unchanged: enforced. Ran: go test -tags ze_bgp -run over the six faf7c5c380-changed reactor units, PASS on 2026-10-31. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7947ClientPolicyControlsWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_consumer_test.go#L87) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7947ClientPolicyControlsWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_consumer_test.go#L86) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7947-x-5`](#rfc7947-x-5)

The route server SHOULD perform one or more BGP Decision Processes to select routes for subsequent advertisement to its clients, taking into account possible configuration to provide multiple Network Layer Reachability Information (NLRI) paths to a particular client as described in Section 2.3.2.2 or multiple Loc-RIBs as described in Section 2.3.2.1. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc7947): "The route server SHOULD perform one or more BGP Decision Processes to select routes for subsequent advertisement to its clients, taking into account possible configuration to provide multiple Network Layer Reachability Information (NLRI) paths to a particular client as described in Section 2.3.2.2 or multiple Loc-RIBs as described in Section 2.3.2.1." be95c10053 wraps the settings of TestRSAnnouncementAddPathClientGetsEveryPath (and the shared fixture builders) in acceptingPolicies (FinalAccept import and export, no refs) because every peer is EBGP. An accepting policy removes no path, so every path the ADD-PATH client must receive is still selected by the route server's own election. Assertions unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7947RouteServerWorseLaterRouteDoesNotDisplace`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_rs_decision_test.go#L86) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRSAnnouncementAddPathClientGetsEveryPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_rs_announcement_election_test.go#L628) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7947RouteServerWorseReannouncementYieldsOtherSourceRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_rs_decision_test.go#L28) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRSOwnerWithdrawalPolicyDeniedSkipsToNextSurvivor`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_rs_withdrawal_replacement_test.go#L501) | unit/verify | mutant, verified |

### [`RFC7947-2.2-1`](#rfc7947-2.2-1)

Therefore, contrary to what is specified in Section 5 of [RFC4271], route servers SHOULD NOT by default (unless explicitly configured) update well-known BGP attributes received from route server clients before redistributing them to their other route server clients. Optional recognized and unrecognized BGP attributes, whether transitive or non-transitive, SHOULD NOT be updated by the route server (unless enforced by local IXP operator configuration) and SHOULD be passed on to other route server clients. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc7947): "Therefore, contrary to what is specified in Section 5 of [RFC4271], route servers SHOULD NOT by default (unless explicitly configured) update well-known BGP attributes received from route server clients before redistributing them to their other route server clients. Optional recognized and unrecognized BGP attributes, whether transitive or non-transitive, SHOULD NOT be updated by the route server (unless enforced by local IXP operator configuration) and SHOULD be passed on to other route server clients." be95c10053 replaced lowLiveSettings with acceptingLiveSettings for source, client and ordinary peer in TestRFC7947AllAttributesReachClient and TestRouteServerTransparencyStopsAtOrdinaryPeer (FinalAccept import and export, no refs); faf7c5c380 only renamed filterapi to routepolicy elsewhere in the file. An accepting policy rewrites no attribute, so the client's byte-identical attributes, the unknown non-transitive attribute kept from the client, and the Partial bit set on the unknown transitive toward the ordinary peer still come from the route-server transparency producer. Assertions unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRouteServerTransparencyStopsAtOrdinaryPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_consumer_test.go#L55) | unit/verify | revert, verified |
| positive | [`TestRFC7947AllAttributesReachClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7947_consumer_test.go#L33) | unit/verify | revert, verified |

### [`RFC7947-2.2.2.2-1`](#rfc7947-2.2.2.2-1)

In contrast to what is recommended in Section 6.3 of [RFC4271], route server clients need to be able to accept UPDATE messages where the leftmost AS in the AS_PATH attribute is not equal to the AS number of the route server that sent the UPDATE message. If the route server client BGP system has implemented a check for this, the BGP implementation MUST allow this check to be disabled and SHOULD allow the check to be disabled on a per-peer basis. (§2.2.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The client must accept a leftmost AS that is not the RS AS, and MUST allow any such check to be disabled. TestLoopIngressAcceptsNonAdjacentLeftmostAS asserts acceptance of [65002 65003] from RS peer AS 65010, so any leftmost-AS equality check goes red. TestLoopIngressRejectsLocalASFromRouteServer shows the filter reads AS_PATH and rejects a local-AS loop on the same session. ze has no such check, so the disable clause holds by absence. Note: check.log reports both discrimination records for these units as producer-changed (loop.go moved), and they need re-recording.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLoopIngressRejectsLocalASFromRouteServer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L363) | unit/verify | revert, verified |
| positive | [`TestLoopIngressAcceptsNonAdjacentLeftmostAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L345) | unit/verify | revert, verified |

### [`RFC7947-2.1-1`](#rfc7947-2.1-1)

A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7947-2.1-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc7947 |
| Signed off | 2026-09-01 |
| Register | rfc2119 |
| Source | rfc/full/rfc7947.txt |
| Source fingerprint | 6d8434457eff580c |
| Record | rfc/extraction/rfc7947.json |
| Mapped sentences | 3 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. The Abstract restates section 1 and directs no speaker. |
| `1` | Introduction to Multilateral Interconnection | 0 | walked | Introduction to Multilateral Interconnection. Indicative prose: why dense bilateral peering scales badly at an IXP, what a route server is, that it forwards no traffic and is therefore not a router, that it is similar in function to a route reflector but runs eBGP, and that route server functionality is not mandatory in BGP implementations. The last paragraph fixes the term 'route server' against 'route collector'. No sentence directs a speaker. |
| `1.1` | Notational Conventions | 0 | walked | Notational Conventions. The RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Technical Considerations for Route Server Implementations | 0 | walked | Technical Considerations for Route Server Implementations. One paragraph saying a route server brokers reachability with BGP, that its behavior differs from a speaker strictly compliant with RFC 4271, and that the differences follow. No directive. |
| `2.1` | Client UPDATE Messages | 1 | walked | Client UPDATE Messages. Carries the document's only MUST that binds the route server itself, site 2.1:1 below. Ze does not meet it: notifyMessageReceiver (internal/component/bgp/reactor/reactor_notify.go) returns false at the ordered ingress step loop's non-accept branch, ahead of the recentUpdates cache add, ahead of reactorForwardRS and ahead of the plugin dispatch that feeds handleReceivedStructured (internal/component/bgp/plugins/adj_rib_in/rib.go), so a denied UPDATE from a route server client never reaches the Adj-RIB-In. RFC7947-2.1-1 carries it with {gap}, and the site below maps to that row; the fix is homed by owner ruling of 2026-08-30 in plan/release/fixes/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md. Section 2.1's remaining directives carry no MUST-level keyword, so the site scan cannot see them: the MAY that lets a configured policy filter omit an UPDATE from the Loc-RIB, the SHOULD to perform one or more BGP Decision Processes, and the SHOULD to forward UPDATE messages from the Loc-RIB to the clients as determined by local policy. RFC7947-x-4 is read from that last SHOULD and from the MAY beside it, which is why it is declared unsourced here rather than mapped from a site; note that the summary states it at [MUST] while this section states it at SHOULD. |
| `2.2` | Attribute Transparency | 0 | walked | Attribute Transparency. Three advisory sentences and no MUST-level keyword, so the site scan sees nothing here: contrary to RFC 4271 Section 5, route servers SHOULD NOT by default update well-known BGP attributes received from clients before redistributing them; optional recognized and unrecognized attributes SHOULD NOT be updated unless enforced by local IXP operator configuration; and those attributes SHOULD be passed on to the other clients. rfc/short/rfc7947.md carries the AS_PATH and MULTI_EXIT_DISC instances of this rule as RFC7947-x-1 and RFC7947-x-3. The general well-known and optional attribute recommendation has no id of its own; Ze meets it by relaying an RS client's received bytes unchanged (TestReactorForwardRSTransparent, internal/component/bgp/reactor/rfc7947_forward_rs_test.go). |
| `2.2.1` | NEXT_HOP Attribute | 1 | walked | NEXT_HOP Attribute. Defines the attribute, then states the transparency MUST mapped below to RFC7947-x-2. |
| `2.2.2` | AS_PATH Attribute | 0 | walked | AS_PATH Attribute. One sentence saying what AS_PATH identifies. No directive: the two AS_PATH rules are stated in 2.2.2.1 and 2.2.2.2. |
| `2.2.2.1` | Route Server AS_PATH Management | 0 | walked | Route Server AS_PATH Management. The route server SHOULD NOT prepend its own AS number nor modify the AS_PATH segment in any other way, which rfc/short/rfc7947.md carries as RFC7947-x-1 at [SHOULD NOT]. The section says outright that this 'is a recommendation rather than a requirement', existing solely for backwards compatibility with legacy clients that do not yet support Section 2.2.2.2, so no MUST-level keyword appears and the site scan sees nothing here. |
| `2.2.2.2` | Route Server client AS_PATH Management | 1 | walked | Route Server client AS_PATH Management. States that route server clients need to accept UPDATEs whose leftmost AS is not the sending route server's AS, then puts a conditional MUST and a conditional SHOULD on the client's own BGP implementation. That MUST is site 2.2.2.2:1 below, excluded as binding a role Ze does not implement. |
| `2.2.3` | MULTI_EXIT_DISC Attribute | 0 | walked | MULTI_EXIT_DISC Attribute. Defines the attribute, then states, contrary to RFC 4271 Section 5.1.4, that a metric applied to an NLRI UPDATE sent to a route server SHOULD be propagated to the other clients and that the route server SHOULD NOT modify its value. rfc/short/rfc7947.md carries both halves as RFC7947-x-3 at [SHOULD]. No MUST-level keyword, so no site. |
| `2.2.4` | Communities Attributes | 0 | walked | Communities Attributes. Transitive and non-transitive Communities and Extended Communities applied to an UPDATE sent to a route server SHOULD NOT be modified, processed, or removed except as defined by local policy, and a Communities attribute intended for processing by the route server itself MAY be modified or removed. Both are advisory, so the site scan sees nothing here, and neither has an id in rfc/short/rfc7947.md. Ze meets the recommendation the same way it meets section 2.2, by relaying the client's received bytes unchanged to the other RS clients. |
| `2.3` | Per-Client Policy Control in Multilateral Interconnection | 0 | walked | Per-Client Policy Control in Multilateral Interconnection. Introduces per-client path distribution control and names its consequence, 'path hiding'. Its last paragraph states that neither this section nor its subsections form part of the normative specification, so nothing under 2.3 can bind a speaker. |
| `2.3.1` | Path Hiding on a Route Server | 0 | walked | Path Hiding on a Route Server. Figure 1 and a worked example of how a single best path plus a per-client outbound filter hides an alternative path. Descriptive, and inside the non-normative scope its parent declares. |
| `2.3.2` | Mitigation of Path Hiding | 0 | walked | Mitigation of Path Hiding. One sentence saying several approaches exist. Non-normative by its parent's declaration. |
| `2.3.2.1` | Multiple Route Server RIBs | 0 | walked | Multiple Route Server RIBs. Describes per-client Loc-RIBs with filtering between the Adj-RIB-In and each per-client Loc-RIB, and the delta optimization. RFC7947-x-5 reads its 'multi-RIB per client' mitigation from here. Non-normative by its parent's declaration. |
| `2.3.2.2` | Advertising Multiple Paths | 0 | walked | Advertising Multiple Paths. Describes how sending more than one path per prefix would remove path hiding, and announces the two methods below. Non-normative by its parent's declaration. |
| `2.3.2.2.1` | Diverse BGP Path Approach | 0 | walked | Diverse BGP Path Approach. Describes RFC 6774: one BGP session per distributed path, bounded by the number of sessions, and not guaranteed to remove path hiding. Non-normative by its parent's declaration. |
| `2.3.2.2.2` | BGP ADD-PATH Approach | 0 | walked | BGP ADD-PATH Approach. Describes RFC 7911 and the send-only mode a route server 'should enforce' toward its clients. Section 2.3 explicitly states: "Neither Section 2.3 nor its subsections form part of the normative specification of this document; they are included for information purposes only." No normative permission or replacement requirement is extracted from this subsection. |
| `2.3.3` | Implementation Suggestions | 0 | walked | Implementation Suggestions. Authors 'may wish to consider' one of the methods in 2.3.2, and operational recommendations are deferred to RFC 7948. Non-normative by its parent's declaration. |
| `3` | Security Considerations | 0 | walked | Security Considerations. Three statements, none a MUST-level keyword: route server operators should be aware that path hiding can be used to block third-party announcements, the AS_PATH check of Section 2.2.2 SHOULD NOT be disabled by IXP participants unless it is needed to bring up a route server session, and operators should consider the practices in RFC 7454. The second binds the IXP participant configuring its own router, which is the same role site 2.2.2.2:1 is excluded against. |
| `4` | References | 0 | skipped (references) | References. A heading over 4.1 and 4.2. |
| `4.1` | Normative References: RFC 1997, RFC 2119, RFC 4271, RFC 4360 | 0 | skipped (references) | Normative References: RFC 1997, RFC 2119, RFC 4271, RFC 4360. |
| `4.2` | not stated | 0 | skipped (references) | Informative References: RFC 1863, RFC 4223, RFC 4456, RFC 6774, RFC 7454, RFC 7911, RFC 7948. The derived section list ends here, so the Acknowledgments and Authors' Addresses that follow in the source are read under this heading; neither states an obligation. |

### Excluded sentences

The walk over RFC 7947 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 7947, so its obligations are stated where they were written.
