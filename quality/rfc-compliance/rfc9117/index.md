# RFC 9117 - Revised Validation Procedure for BGP Flow Specifications

Partial. Every requirement this repository extracted from RFC 9117, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 2 of 2 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 2 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 2 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 2 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 2 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 8 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 2 | of 2 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 2 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 2 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 2 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 2 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 2 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 2 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 5 |
| Gated MUST-level | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 2 |
| Discrimination records | 8 |
| Summary | `rfc/short/rfc9117.md` |
| Requirement shard | `rfc/requirements/rfc9117.md` |
| RFC text | `rfc/full/rfc9117.txt` |

## Enrolment

Enrolled: Revised FlowSpec validation: two MUST-level requirements, both met by flowSpecAuthorized (internal/component/bgp/plugins/rib/rib_flowspec_validation.go) on the received-UPDATE rail. RFC9117-4.1-1 redefines step (b) of RFC 8955 Section 6 (the originator match, or an empty or confederation-only AS_PATH); RFC9117-4.2-1 replaces the RFC 8955 neighboring-AS rule with a comparison against the left-most AS of the best-match unicast route. Both are proven in both polarities by TestFlowSpecAuthorizationFromReceivedUpdates. Enrolled 2026-10-02 so the redefined rule has its own row: RFC8955-6-2, which quoted the replaced sentence, is retired under D-10 (rfc/corrections/rfc8955.md).

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Received IPv4 and IPv6 FlowSpec (SAFI 133/134) rules are feasible through the revised step (b): an originator match with the covering unicast route, or an empty or confederation-only AS_PATH
- an eBGP rule's left-most AS must equal the covering unicast route's left-most AS, so FlowSpec crosses a route server that does not prepend its AS. Condition (b.2) is always enabled.


**What the ledger says remains:**

No configuration disables condition (b.2) and no policy permits a non-empty AS_PATH (the two Section 4.1 MAYs). No discriminating test yet proves that the covering unicast route compared is the longest-prefix (best-match) one.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **2** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC9117-4.1-1`](#rfc9117-4.1-1), [`RFC9117-4.2-1`](#rfc9117-4.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9117-4.1-1` | b) One of the following conditions MUST hold true: \| \| 1. The originator of the Flow Specification matches the \| originator of the best-match unicast route for the \| destination prefix embedded in the Flow Specification (this \| is the unicast route with the longest possible prefix \| length covering the destination prefix embedded in the Flow \| Specification). \| \| 2. The AS_PATH attribute of the Flow Specification is empty or \| contains only an AS_CONFED_SEQUENCE segment [RFC5065]. (§4.1) | MUST | 4.1 - Revision of Route Feasibility | **positive:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L128). **positive:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L20). **negative:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L129). **negative:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L21) |
| `RFC9117-4.2-1` | BGP Flow Specification implementations MUST enforce that the AS \| in the left-most position of the AS_PATH attribute of a Flow \| Specification route received via the External Border Gateway \| Protocol (eBGP) matches the AS in the left-most position of the \| AS_PATH attribute of the best-match unicast route for the \| destination prefix embedded in the Flow Specification NLRI. (§4.2) | MUST | 4.2 - Revision of AS_PATH Validation | **positive:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L130). **positive:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L22). **negative:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L131). **negative:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L23) |
| `RFC9117-4.1-2` | This condition SHOULD be enabled by default. (§4.1) | SHOULD | 4.1 - Revision of Route Feasibility | **positive:** no positive test. **negative:** no negative test |
| `RFC9117-4.1-3` | This condition MAY be disabled by explicit \| configuration on a BGP speaker. (§4.1) | MAY | 4.1 - Revision of Route Feasibility | **positive:** no positive test. **negative:** no negative test |
| `RFC9117-4.1-4` | As an extension to this rule, a given non-empty AS_PATH \| (besides AS_CONFED_SEQUENCE segments) MAY be permitted \| by policy. (§4.1) | MAY | 4.1 - Revision of Route Feasibility | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 9117 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9117-4.1-1`](#rfc9117-4.1-1)

b) One of the following conditions MUST hold true: | | 1. The originator of the Flow Specification matches the | originator of the best-match unicast route for the | destination prefix embedded in the Flow Specification (this | is the unicast route with the longest possible prefix | length covering the destination prefix embedded in the Flow | Specification). | | 2. The AS_PATH attribute of the Flow Specification is empty or | contains only an AS_CONFED_SEQUENCE segment [RFC5065]. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC9117 Section 4.1: 'b) One of the following conditions MUST hold true: 1. The originator of the Flow Specification matches the originator of the best-match unicast route for the destination prefix embedded in the Flow Specification (this is the unicast route with the longest possible prefix length covering the destination prefix embedded in the Flow Specification). 2. The AS_PATH attribute of the Flow Specification is empty or contains only an AS_CONFED_SEQUENCE segment [RFC5065].' Section 1 extends the considerations to AS_CONFED_SET. Both polarities of TestFlowSpecAuthorizationFromReceivedUpdates and TestRFC9117AuthorizationUsesLongestCoveringRoute assert current eligibility, retained presence, candidates and exact zero-or-one selected nonwithdraw NLRI/action events. Matching ORIGINATOR_ID, empty internal path and confed-only paths succeed; differing transports despite equal router IDs and unmatched ORIGINATOR_ID fail. The internal /8-/16 pair has ordinary nonempty AS_SEQUENCE and changes only originator, proving longest-cover selection without local-domain or external-AS masking. flowSpecOriginator and flowSpecASPath feed flowSpecAuthorized via handleReceivedStructured/reconcileFlowSpecs; candidate and FlowSpecChanged consumers observe its decision. All four audit questions and both polarities pass by independent source reading. Stored panic records prove producer reachability, not semantic mutation. Current execution and independent live-peer/VPP acceptance remain pending.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L21) | unit/verify | revert, verified |
| negative | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L129) | unit/verify | revert, verified |
| positive | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L128) | unit/verify | revert, verified |

### [`RFC9117-4.2-1`](#rfc9117-4.2-1)

BGP Flow Specification implementations MUST enforce that the AS | in the left-most position of the AS_PATH attribute of a Flow | Specification route received via the External Border Gateway | Protocol (eBGP) matches the AS in the left-most position of the | AS_PATH attribute of the best-match unicast route for the | destination prefix embedded in the Flow Specification NLRI. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC9117 Section 4.2: 'BGP Flow Specification implementations MUST enforce that the AS in the left-most position of the AS_PATH attribute of a Flow Specification route received via the External Border Gateway Protocol (eBGP) matches the AS in the left-most position of the AS_PATH attribute of the best-match unicast route for the destination prefix embedded in the Flow Specification NLRI.' The explanation defines this as the AS last added to AS_SEQUENCE and excludes same-Local-Domain validation from the new rule's scope. Both polarities of TestFlowSpecAuthorizationFromReceivedUpdates and TestRFC9117AuthorizationUsesLongestCoveringRoute were read. PeerAS65100 with matching path AS65001 succeeds, so obsolete peer-AS equality cannot pass; changed first AS and AS_SET-only path yield retained but ineligible routes with no candidates/events. The external /8-/16 cases keep the /16 originator and external peer constant while varying first AS65002 versus AS65001, requiring exactly one matching NLRI/action install versus zero. flowSpecASPath and flowSpecAuthorized compare first AS against the longest-cover route, with received-RIB eligibility, candidate and event consumers enforcing the result. All four questions pass and the polarities are genuine isolated cases. Native panic records show reachability only; no semantic mutation or test execution was observed in this audit, and independent live-peer/VPP acceptance remains pending.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L23) | unit/verify | revert, verified |
| negative | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L131) | unit/verify | revert, verified |
| positive | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L130) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (BGP verdict-fix author c35) |
| Signed off | 2026-10-02 |
| Register | rfc2119 |
| Source | rfc/full/rfc9117.txt |
| Source fingerprint | 76653fb7d70944f4 |
| Record | rfc/extraction/rfc9117.json |
| Mapped sentences | 2 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates Sections 4.1 and 4.2: originator matching is relaxed for rules originated inside the same AS, and AS_PATH validation is revised so eBGP rules can be validated through a route server. No sentence directs a speaker beyond those sections. |
| `1` | Introduction | 0 | walked | Introduction. Indicative: what RFC 8955 FlowSpec is for, that its validation procedure requires the originator of the rule to match the originator of the best-match unicast route, the centralized route controller inside the Local Domain that this check defeats (Figure 1), and the route server case. No capitalised keyword; the obligations it motivates are stated in Section 4. |
| `2` | not stated | 0 | walked | Definitions of Terms Used in This Memo: Local Domain, eBGP and iBGP (an eBGP session inside one confederation counts as iBGP). Definitional only. The paragraph also holds the RFC 2119 and RFC 8174 key-words boilerplate, which binds no speaker and is excluded from the site inventory. |
| `3` | Motivation | 0 | walked | Motivation. Indicative: why step (b) of RFC 8955 Section 6 assumes the rule follows the path of the longest-match unicast route (Figure 2), and where that assumption fails. No keyword. |
| `4` | Revised Validation Procedure | 0 | walked | Revised Validation Procedure. A heading with no body text; its two subsections carry the sites. |
| `4.1` | Revision of Route Feasibility | 1 | walked | Revision of Route Feasibility. One MUST-level site, 4.1:1, the redefined step (b), mapped to RFC9117-4.1-1. The SHOULD (condition b.2 enabled by default) and the two MAYs (disable b.2 by configuration; permit a non-empty AS_PATH by policy) are rows RFC9117-4.1-2, 4.1-3 and 4.1-4, not gated. The Explanation paragraphs are indicative. |
| `4.2` | Revision of AS_PATH Validation | 2 | walked | Revision of AS_PATH Validation. Site 4.2:1 quotes the RFC 8955 Section 6 sentence this section replaces; site 4.2:2 is the replacement rule, mapped to RFC9117-4.2-1. The Explanation paragraphs are indicative, including the remark that the original rule's enforcement remains optional per RFC 4271 Section 6.3. |
| `5` | Topology Considerations | 0 | walked | Topology Considerations. Indicative: congruent topology between unicast and FlowSpec routes, and how condition (b.2) supports non-congruent topologies inside the Local Domain. The lowercase 'should be designed' is advice to network designers, not to a speaker. |
| `6` | IANA Considerations: no IANA actions | 0 | walked | IANA Considerations: no IANA actions. |
| `7` | Security Considerations | 0 | walked | Security Considerations. The capitalised OPTIONAL describes what Section 4.2 did to the RFC 4271 Section 6.3 rule. The SHOULD and SHOULD NOT advise enforcing that optional rule only when configuration indicates the peer is not a route server; both are recommendations about an optional rule, below the MUST-level the inventory counts. No MUST-level keyword. |
| `8` | References heading | 0 | skipped (references) | References heading. |
| `8.1` | Normative References | 0 | skipped (references) | Normative References. |
| `8.2` | Informative References | 0 | skipped (references) | Informative References. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A quotation: the section opens 'Section 6 of [RFC8955] states:' and cites this sentence in order to replace it. This document does not impose it; the replacement is site 4.2:2. | \| BGP implementations MUST also enforce that the AS_PATH \| attribute of a route received via the External Border Gateway \| Protocol (eBGP) contains the neighboring AS in the left-most \| position of the AS_PATH attribute. |

## Superseded

No document obsoletes RFC 9117, so its obligations are stated where they were written.
