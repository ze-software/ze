# RFC 7947 - Internet Exchange BGP Route Server

Supported. Every requirement this repository extracted from RFC 7947, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 66.7% | 2 of 3 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 33.3% | 1 of 3 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 3 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 11.1% | 2 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 3 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 8 |
| Gated MUST-level | 3 |
| Obligations that bind Ze | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 0 |
| Discrimination records | 2 |
| Summary | `rfc/short/rfc7947.md` |
| Requirement shard | `rfc/requirements/rfc7947.md` |
| RFC text | `rfc/full/rfc7947.txt` |

## Enrolment

Enrolled: BGP Route Server: two MUST-level transparency requirements over the reactor-native RS forwarding path (internal/component/bgp/reactor/forward_rs.go), plus the advisory-level RFC7947-x-1, which the gate does not require but which is proven anyway. RFC7947-x-1 (SHOULD NOT prepend own AS for RS clients -- Section 2.2.2.1 calls it "a recommendation rather than a requirement", so it is not MUST-level and the gate does not require it) has both polarities: TestReactorForwardRSTransparent proves an RS client's forwarded body is byte-identical (no prepend), TestReactorForwardRSEBGPPrepend proves a non-RS eBGP peer DOES get the local AS prepended, so the no-prepend is confined to RS clients. RFC7947-x-2 (MUST NOT rewrite NEXT_HOP) is {single-polarity: positive}: the byte-identical transparency test proves it, and it has no natural negative, NEXT_HOP transparency not being RS-specific. RFC7947-x-3 is NOT MUST-level and the gate does not require it: Section 2.2.3 reads "this attribute SHOULD be propagated to other route server clients, and the route server SHOULD NOT modify its value", so the row is [SHOULD] and the earlier MUST here misquoted it. The same transparency test proves Ze meets it anyway: medPropagationAllowedTo (internal/component/bgp/reactor/forward_med.go) never applies the automatic RFC 4271 Section 5.1.4 strip to a route server client. An operator's own `del { med; }` policy still removes the metric upstream of that predicate (test/plugin/med-removal-configured.ci, against an rs-client receiver), which is Section 5.1.4's required removal mechanism rather than an exception to this recommendation. RFC7947-x-4 (per-client import/export policy applied on each redistribution) has both polarities: TestReactorForwardRSFallback proves an export-filtered client is routed to the per-client policy path, TestReactorForwardRSBasic proves an unfiltered client is forwarded directly. The x-5 SHOULD (path-hiding mitigation) and x-6 MAY (add-path) are not gated.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

- Transparent RS forwarding for RS clients by default: no AS_PATH prepend, no NEXT_HOP rewrite, MED preserved (verbatim wire forwarding)
- per-client import/export policy applied on redistribution. Sections 2.2.2.1 and 2.2.3 state the AS_PATH and MULTI_EXIT_DISC rules as recommendations, so an operator policy may override each. On the client side of the same document, Section 2.2.2.2 ([`RFC7947-2.2.2.2-1`](#rfc7947-2.2.2.2-1)) asks that a leftmost-AS check be possible to disable
- ze runs no ingress leftmost-AS check at all, so a route ze receives from a route server whose leftmost AS_PATH AS is another client's is accepted in every configuration (`LoopIngress`, [`internal/component/bgp/reactor/filter/loop.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop.go)). The walk that bounds this row against the RFC's own text is [`rfc/extraction/rfc7947.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc7947.json).


**What the ledger says remains**

Optional per-peer next-hop override and path-hiding mitigation (add-path) are operator-configured. A modify policy with `as-path-prepend` or `del { med; }` also changes what an RS client receives. Section 2.1 requires a route server to accept every UPDATE it receives from a client for inclusion in its Adj-RIB-In, and ze does not: `notifyMessageReceiver` ([`internal/component/bgp/reactor/reactor_notify.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_notify.go)) returns at the ordered ingress step loop's non-accept branch, ahead of the cache add, ahead of `reactorForwardRS` and ahead of the plugin dispatch, so an UPDATE a filter denied is absent from `show bgp adj-rib-in`. `LoopIngress` denies with no operator configuration, so the path is reachable by default. Thomas ruled on 2026-08-30 that ze stores such an UPDATE first and marks the entry filtered; that work is homed in [`plan/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md`](https://github.com/ze-software/ze/blob/main/plan/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md), which reserves the id RFC7947-2.1-1, and site 2.1:1 of the extraction sign-off records the relocation.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC7947-x-4`](#rfc7947-x-4), [`RFC7947-2.2.2.2-1`](#rfc7947-2.2.2.2-1)

**Annotated instead of tested (1):** [`RFC7947-x-2`](#rfc7947-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7947-x-1` | Route server SHOULD NOT prepend its own AS to AS_PATH nor modify it in any other way, per RFC 7947 Section 2.2.2.1 (Key Requirements) | SHOULD NOT | x | **positive:** `unit/verify` [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L440). **positive:** `unit/verify` [`TestRelayStoredRouteRSClientPreservesASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_relay_test.go#L222). **negative:** `unit/verify` [`TestReactorForwardRSEBGPPrepend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L318). **negative:** `unit/verify` [`TestRelayStoredRoutePlainEBGPPrependsLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_relay_test.go#L258). **positive:** `functional/verify` [`bgp-rs-relay-aspath-transparency.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/bgp-rs-relay-aspath-transparency.ci#L4). **negative:** `functional/verify` [`bgp-rs-relay-aspath-transparency.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/bgp-rs-relay-aspath-transparency.ci#L8). **positive:** `interop/nightly` [`checkRouteServerASPath`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L900) |
| `RFC7947-x-2` | Route server must not rewrite NEXT_HOP (Key Requirements) | MUST NOT | x | **positive:** `unit/verify` [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L445). **negative:** no negative test. **{single-polarity}:** NEXT_HOP transparency is not RS-specific -- every forwarded route preserves it by default (nhModeNone, internal/component/bgp/reactor/peer_forward_facts.go:147), so unlike x-1's AS-path prepend there is no "confined" negative where a comparable non-RS peer rewrites NEXT_HOP; the only rewrite is an explicit per-peer next-hop-self/explicit override, which exercises the override feature rather than the RS-transparency MUST-NOT. The positive is proven byte-identical in TestReactorForwardRSTransparent |
| `RFC7947-x-3` | MULTI_EXIT_DISC applied to an NLRI UPDATE sent to a route server SHOULD be propagated to other route server clients, and the route server SHOULD NOT modify its value, per RFC 7947 Section 2.2.3 (Key Requirements) | SHOULD | x | **positive:** `unit/verify` [`TestForwardKeepsMEDForRouteServerClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L364). **positive:** `unit/verify` [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L447). **positive:** `functional/verify` [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L11). **negative:** `functional/verify` [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L15) |
| `RFC7947-x-4` | Per-client import/export policy must be applied on each redistribution (Key Requirements) | MUST | x | **positive:** `unit/verify` [`TestReactorForwardRSFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L241). **negative:** `unit/verify` [`TestReactorForwardRSBasic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L153) |
| `RFC7947-x-5` | Route server should mitigate path hiding (e.g., multi-RIB per client or add-path) (Key Requirements) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7947-x-6` | Route server may use ADD-PATH (RFC 7911) to distribute multiple paths to clients (Key Requirements) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7947-2.2-1` | A route server SHOULD NOT by default update the well-known or optional BGP attributes it receives from a route server client, and SHOULD pass them on unchanged to its other clients (S2.2) | SHOULD NOT | 2.2 - Attribute Transparency | **positive:** `unit/verify` [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L451). **negative:** `unit/verify` [`TestReactorForwardRSEBGPPrepend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L322) |
| `RFC7947-2.2.2.2-1` | A route server client BGP implementation that has implemented the RFC 4271 Section 6.3 leftmost-AS check MUST allow that check to be disabled, so an UPDATE whose leftmost AS in AS_PATH is not the AS of the route server that sent it is accepted (S2.2.2.2) | MUST | 2.2.2.2 - Route Server client AS_PATH Management | **positive:** `unit/verify` [`TestLoopIngressAcceptsNonAdjacentLeftmostAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L345). **negative:** `unit/verify` [`TestLoopIngressRejectsLocalASFromRouteServer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L363) |

## Gaps and untested MUSTs

RFC 7947 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7947-x-1`](#rfc7947-x-1)

Route server SHOULD NOT prepend its own AS to AS_PATH nor modify it in any other way, per RFC 7947 Section 2.2.2.1 (Key Requirements)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRSEBGPPrepend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L318) | unit/verify | unproven |
| negative | [`TestRelayStoredRoutePlainEBGPPrependsLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_relay_test.go#L258) | unit/verify | unproven |
| negative | [`bgp-rs-relay-aspath-transparency.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/bgp-rs-relay-aspath-transparency.ci#L8) | functional/verify | unproven |
| positive | [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L440) | unit/verify | unproven |
| positive | [`TestRelayStoredRouteRSClientPreservesASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_relay_test.go#L222) | unit/verify | unproven |
| positive | [`checkRouteServerASPath`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L900) | interop/nightly | unproven |
| positive | [`bgp-rs-relay-aspath-transparency.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/bgp-rs-relay-aspath-transparency.ci#L4) | functional/verify | unproven |

### [`RFC7947-x-2`](#rfc7947-x-2)

Route server must not rewrite NEXT_HOP (Key Requirements)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L445) | unit/verify | unproven |

### [`RFC7947-x-3`](#rfc7947-x-3)

MULTI_EXIT_DISC applied to an NLRI UPDATE sent to a route server SHOULD be propagated to other route server clients, and the route server SHOULD NOT modify its value, per RFC 7947 Section 2.2.3 (Key Requirements)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L15) | functional/verify | unproven |
| positive | [`TestForwardKeepsMEDForRouteServerClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L364) | unit/verify | unproven |
| positive | [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L447) | unit/verify | unproven |
| positive | [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L11) | functional/verify | unproven |

### [`RFC7947-x-4`](#rfc7947-x-4)

Per-client import/export policy must be applied on each redistribution (Key Requirements)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRSBasic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L153) | unit/verify | unproven |
| positive | [`TestReactorForwardRSFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L241) | unit/verify | unproven |

### [`RFC7947-2.2-1`](#rfc7947-2.2-1)

A route server SHOULD NOT by default update the well-known or optional BGP attributes it receives from a route server client, and SHOULD pass them on unchanged to its other clients (S2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRSEBGPPrepend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L322) | unit/verify | unproven |
| positive | [`TestReactorForwardRSTransparent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs_test.go#L451) | unit/verify | unproven |

### [`RFC7947-2.2.2.2-1`](#rfc7947-2.2.2.2-1)

A route server client BGP implementation that has implemented the RFC 4271 Section 6.3 leftmost-AS check MUST allow that check to be disabled, so an UPDATE whose leftmost AS in AS_PATH is not the AS of the route server that sent it is accepted (S2.2.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLoopIngressRejectsLocalASFromRouteServer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L363) | unit/verify | mutant, verified |
| positive | [`TestLoopIngressAcceptsNonAdjacentLeftmostAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L345) | unit/verify | mutant, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc7947 |
| Signed off | 2026-09-01 |
| Register | rfc2119 |
| Source | rfc/full/rfc7947.txt |
| Source fingerprint | 6d8434457eff580c |
| Record | rfc/extraction/rfc7947.json |
| Mapped sentences | 2 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 1 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. The Abstract restates section 1 and directs no speaker. |
| `1` | Introduction to Multilateral Interconnection | 0 | walked | Introduction to Multilateral Interconnection. Indicative prose: why dense bilateral peering scales badly at an IXP, what a route server is, that it forwards no traffic and is therefore not a router, that it is similar in function to a route reflector but runs eBGP, and that route server functionality is not mandatory in BGP implementations. The last paragraph fixes the term 'route server' against 'route collector'. No sentence directs a speaker. |
| `1.1` | Notational Conventions | 0 | walked | Notational Conventions. The RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Technical Considerations for Route Server Implementations | 0 | walked | Technical Considerations for Route Server Implementations. One paragraph saying a route server brokers reachability with BGP, that its behavior differs from a speaker strictly compliant with RFC 4271, and that the differences follow. No directive. |
| `2.1` | Client UPDATE Messages | 1 | walked | Client UPDATE Messages. Carries the document's only MUST that binds the route server itself, site 2.1:1 below. Ze does not meet it: notifyMessageReceiver (internal/component/bgp/reactor/reactor_notify.go) returns false at the ordered ingress step loop's non-accept branch, ahead of the recentUpdates cache add, ahead of reactorForwardRS and ahead of the plugin dispatch that feeds handleReceivedStructured (internal/component/bgp/plugins/adj_rib_in/rib.go), so a denied UPDATE from a route server client never reaches the Adj-RIB-In. The obligation is homed by owner ruling of 2026-08-30 in plan/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md, which reserves the id RFC7947-2.1-1 for it, and the site below is the relocation. Section 2.1's remaining directives carry no MUST-level keyword, so the site scan cannot see them: the MAY that lets a configured policy filter omit an UPDATE from the Loc-RIB, the SHOULD to perform one or more BGP Decision Processes, and the SHOULD to forward UPDATE messages from the Loc-RIB to the clients as determined by local policy. RFC7947-x-4 is read from that last SHOULD and from the MAY beside it, which is why it is declared unsourced here rather than mapped from a site; note that the summary states it at [MUST] while this section states it at SHOULD. |
| `2.2` | Attribute Transparency | 0 | walked | Attribute Transparency. Three advisory sentences and no MUST-level keyword, so the site scan sees nothing here: contrary to RFC 4271 Section 5, route servers SHOULD NOT by default update well-known BGP attributes received from clients before redistributing them; optional recognized and unrecognized attributes SHOULD NOT be updated unless enforced by local IXP operator configuration; and those attributes SHOULD be passed on to the other clients. rfc/short/rfc7947.md carries the AS_PATH and MULTI_EXIT_DISC instances of this rule as RFC7947-x-1 and RFC7947-x-3. The general well-known and optional attribute recommendation has no id of its own; Ze meets it by relaying an RS client's received bytes unchanged (TestReactorForwardRSTransparent, internal/component/bgp/reactor/forward_rs_test.go). |
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
| `2.3.2.2.2` | BGP ADD-PATH Approach | 0 | walked | BGP ADD-PATH Approach. Describes RFC 7911 and the send-only mode a route server 'should enforce' toward its clients. The keyword is lower case, so under section 1.1 it carries no RFC 2119 level and the site scan sees nothing. RFC7947-x-6 reads its [MAY] from this subsection. Non-normative by its parent's declaration. |
| `2.3.3` | Implementation Suggestions | 0 | walked | Implementation Suggestions. Authors 'may wish to consider' one of the methods in 2.3.2, and operational recommendations are deferred to RFC 7948. Non-normative by its parent's declaration. |
| `3` | Security Considerations | 0 | walked | Security Considerations. Three statements, none a MUST-level keyword: route server operators should be aware that path hiding can be used to block third-party announcements, the AS_PATH check of Section 2.2.2 SHOULD NOT be disabled by IXP participants unless it is needed to bring up a route server session, and operators should consider the practices in RFC 7454. The second binds the IXP participant configuring its own router, which is the same role site 2.2.2.2:1 is excluded against. |
| `4` | References | 0 | skipped (references) | References. A heading over 4.1 and 4.2. |
| `4.1` | Normative References: RFC 1997, RFC 2119, RFC 4271, RFC 4360 | 0 | skipped (references) | Normative References: RFC 1997, RFC 2119, RFC 4271, RFC 4360. |
| `4.2` | not stated | 0 | skipped (references) | Informative References: RFC 1863, RFC 4223, RFC 4456, RFC 6774, RFC 7454, RFC 7911, RFC 7948. The derived section list ends here, so the Acknowledgments and Authors' Addresses that follow in the source are read under this heading; neither states an obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.1:1` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation IS owed and Ze does not produce it, so this is not a dismissal. notifyMessageReceiver (internal/component/bgp/reactor/reactor_notify.go) walks the ordered ingress steps and returns false on the first step that answers accept false, with the comment 'Route rejected by filter; don't cache or dispatch.'. That return precedes the recentUpdates cache add, the reactorForwardRS fast path and the plugin dispatch, so nothing reaches handleReceivedStructured (internal/component/bgp/plugins/adj_rib_in/rib.go) and the Adj-RIB-In holds the post-policy population where this sentence requires the pre-policy one. The path is reachable by default: LoopIngress (internal/component/bgp/reactor/filter/loop.go) denies on the local AS in AS_PATH with no operator configuration at all. Thomas ruled on 2026-08-30 that Ze inserts into the Adj-RIB-In first for a route-server client peer, marking the stored entry filtered, and homed that work in plan/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md, which reserves the id RFC7947-2.1-1 for it and carries fifteen acceptance criteria, a new event type and a new rpc.EventKind ordinal. The id is deliberately absent from rfc/short/rfc7947.md until that spec lands it. (relocated to plan/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md as RFC7947-2.1-1) | A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In. |

## Superseded

No document obsoletes RFC 7947, so its obligations are stated where they were written.
