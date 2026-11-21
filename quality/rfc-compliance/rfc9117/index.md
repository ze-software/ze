# RFC 9117 - Revised Validation Procedure for BGP Flow Specifications

FlowSpec validation. Every requirement listed for RFC 9117, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 2 | of 2 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 2 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 2 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 2 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 2 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 8 | 8 of 8 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 2 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 2 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 2 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 2 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 2 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 2 | of 5 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 2 mandatory requirements: they add up to 2. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | FlowSpec validation |
| Automatic checks | Included |
| Requirements | 5 |
| Mandatory requirements (MUST level) | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tests linked to requirements | 8 |
| Recorded audit verdicts | 2 |
| Fault test records | 8 |
| Summary | `rfc/short/rfc9117.md` |
| Requirement table | `rfc/requirements/rfc9117.md` |
| RFC text | `rfc/full/rfc9117.txt` |

## Automatic checks

Included in automatic checks: Revised FlowSpec validation: two MUST-level requirements, both met by flowSpecAuthorized (internal/component/bgp/plugins/rib/rib_flowspec_validation.go) on the received-UPDATE rail. RFC9117-4.1-1 redefines step (b) of RFC 8955 Section 6 (the originator match, or an empty or confederation-only AS_PATH); RFC9117-4.2-1 replaces the RFC 8955 neighboring-AS rule with a comparison against the left-most AS of the best-match unicast route. Both are proven in both polarities by TestFlowSpecAuthorizationFromReceivedUpdates. Enrolled 2026-10-02 so the redefined rule has its own row: RFC8955-6-2, which quoted the replaced sentence, is retired under D-10 (rfc/corrections/rfc8955.md).

## Published support status

**Area:** FlowSpec validation

**Test verification:** 2 of 2 MUST proven

**Evidence:** unit 2

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **2** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (2):** [`RFC9117-4.1-1`](#rfc9117-4.1-1), [`RFC9117-4.2-1`](#rfc9117-4.2-1)

## Figures

- **MUST bar**: Ze verified 2
- **Overview bar**: Ze MUST verified 2, Not assessed 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 2 |
| MUST rows kept (the MUST bar) | 2 |
| Ze verified | 2 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9117.json](../../../data/rfc/rfc9117.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9117-4.1-1` | b) One of the following conditions MUST hold true: \| \| 1. The originator of the Flow Specification matches the \| originator of the best-match unicast route for the \| destination prefix embedded in the Flow Specification (this \| is the unicast route with the longest possible prefix \| length covering the destination prefix embedded in the Flow \| Specification). \| \| 2. The AS_PATH attribute of the Flow Specification is empty or \| contains only an AS_CONFED_SEQUENCE segment [RFC5065]. (§4.1) | MUST | 4.1 - Revision of Route Feasibility | **positive:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L229). **positive:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L20). **negative:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L230). **negative:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L21) |
| `RFC9117-4.2-1` | BGP Flow Specification implementations MUST enforce that the AS \| in the left-most position of the AS_PATH attribute of a Flow \| Specification route received via the External Border Gateway \| Protocol (eBGP) matches the AS in the left-most position of the \| AS_PATH attribute of the best-match unicast route for the \| destination prefix embedded in the Flow Specification NLRI. (§4.2) | MUST | 4.2 - Revision of AS_PATH Validation | **positive:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L231). **positive:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L22). **negative:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L232). **negative:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L23) |
| `RFC9117-4.1-2` | This condition SHOULD be enabled by default. (§4.1) | SHOULD | 4.1 - Revision of Route Feasibility | **positive:** no positive test. **negative:** no negative test |
| `RFC9117-4.1-3` | This condition MAY be disabled by explicit \| configuration on a BGP speaker. (§4.1) | MAY | 4.1 - Revision of Route Feasibility | **positive:** no positive test. **negative:** no negative test |
| `RFC9117-4.1-4` | As an extension to this rule, a given non-empty AS_PATH \| (besides AS_CONFED_SEQUENCE segments) MAY be permitted \| by policy. (§4.1) | MAY | 4.1 - Revision of Route Feasibility | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 9117 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9117-4.1-1`](#rfc9117-4.1-1)

b) One of the following conditions MUST hold true: | | 1. The originator of the Flow Specification matches the | originator of the best-match unicast route for the | destination prefix embedded in the Flow Specification (this | is the unicast route with the longest possible prefix | length covering the destination prefix embedded in the Flow | Specification). | | 2. The AS_PATH attribute of the Flow Specification is empty or | contains only an AS_CONFED_SEQUENCE segment [RFC5065]. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/rib/rfc9117_best_match_test.go::TestRFC9117AuthorizationUsesLongestCoveringRoute, internal/component/bgp/plugins/rib/rfc9117_best_match_test.go::TestRFC9117AuthorizationUsesLongestCoveringRoute#2, internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go::TestFlowSpecAuthorizationFromReceivedUpdates, internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go::TestFlowSpecAuthorizationFromReceivedUpdates#2 moved. RFC9117 Section 4.1: 'b) One of the following conditions MUST hold true: 1. The originator of the Flow Specification matches the originator of the best-match unicast route for the destination prefix embedded in the Flow Specification (this is the unicast route with the longest possible prefix length covering the destination prefix embedded in the Flow Specification). 2. The AS_PATH attribute of the Flow Specification is empty or contains only an AS_CONFED_SEQUENCE segment [RFC5065].' Section 1 extends the considerations to AS_CONFED_SET. Both polarities of TestFlowSpecAuthorizationFromReceivedUpdates and TestRFC9117AuthorizationUsesLongestCoveringRoute assert current eligibility, retained presence, candidates and exact zero-or-one selected nonwithdraw NLRI/action events. Matching ORIGINATOR_ID, empty internal path and confed-only paths succeed; differing transports despite equal router IDs and unmatched ORIGINATOR_ID fail. The internal /8-/16 pair has ordinary nonempty AS_SEQUENCE and changes only originator, proving longest-cover selection without local-domain or external-AS masking. flowSpecOriginator and flowSpecASPath feed flowSpecAuthorized via handleReceivedStructured/reconcileFlowSpecs; candidate and FlowSpecChanged consumers observe its decision. All four audit questions and both polarities pass by independent source reading. Stored panic records prove producer reachability, not semantic mutation. Current execution and independent live-peer/VPP acceptance remain pending.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L21) | unit/verify | mutant, verified |
| negative | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L230) | unit/verify | mutant, verified |
| positive | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L20) | unit/verify | mutant, verified |
| positive | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L229) | unit/verify | mutant, verified |

### [`RFC9117-4.2-1`](#rfc9117-4.2-1)

BGP Flow Specification implementations MUST enforce that the AS | in the left-most position of the AS_PATH attribute of a Flow | Specification route received via the External Border Gateway | Protocol (eBGP) matches the AS in the left-most position of the | AS_PATH attribute of the best-match unicast route for the | destination prefix embedded in the Flow Specification NLRI. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/rib/rfc9117_best_match_test.go::TestRFC9117AuthorizationUsesLongestCoveringRoute, internal/component/bgp/plugins/rib/rfc9117_best_match_test.go::TestRFC9117AuthorizationUsesLongestCoveringRoute#2, internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go::TestFlowSpecAuthorizationFromReceivedUpdates, internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go::TestFlowSpecAuthorizationFromReceivedUpdates#2 moved. RFC9117 Section 4.2: 'BGP Flow Specification implementations MUST enforce that the AS in the left-most position of the AS_PATH attribute of a Flow Specification route received via the External Border Gateway Protocol (eBGP) matches the AS in the left-most position of the AS_PATH attribute of the best-match unicast route for the destination prefix embedded in the Flow Specification NLRI.' The explanation defines this as the AS last added to AS_SEQUENCE and excludes same-Local-Domain validation from the new rule's scope. Both polarities of TestFlowSpecAuthorizationFromReceivedUpdates and TestRFC9117AuthorizationUsesLongestCoveringRoute were read. PeerAS65100 with matching path AS65001 succeeds, so obsolete peer-AS equality cannot pass; changed first AS and AS_SET-only path yield retained but ineligible routes with no candidates/events. The external /8-/16 cases keep the /16 originator and external peer constant while varying first AS65002 versus AS65001, requiring exactly one matching NLRI/action install versus zero. flowSpecASPath and flowSpecAuthorized compare first AS against the longest-cover route, with received-RIB eligibility, candidate and event consumers enforcing the result. All four questions pass and the polarities are genuine isolated cases. Native panic records show reachability only; no semantic mutation or test execution was observed in this audit, and independent live-peer/VPP acceptance remains pending.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L23) | unit/verify | mutant, verified |
| negative | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L232) | unit/verify | mutant, verified |
| positive | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L22) | unit/verify | mutant, verified |
| positive | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L231) | unit/verify | mutant, verified |

## Requirement list review

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
| `1` | Introduction | 0 | reviewed | Introduction. Indicative: what RFC 8955 FlowSpec is for, that its validation procedure requires the originator of the rule to match the originator of the best-match unicast route, the centralized route controller inside the Local Domain that this check defeats (Figure 1), and the route server case. No capitalised keyword; the obligations it motivates are stated in Section 4. |
| `2` | not stated | 0 | reviewed | Definitions of Terms Used in This Memo: Local Domain, eBGP and iBGP (an eBGP session inside one confederation counts as iBGP). Definitional only. The paragraph also holds the RFC 2119 and RFC 8174 key-words boilerplate, which binds no speaker and is excluded from the site inventory. |
| `3` | Motivation | 0 | reviewed | Motivation. Indicative: why step (b) of RFC 8955 Section 6 assumes the rule follows the path of the longest-match unicast route (Figure 2), and where that assumption fails. No keyword. |
| `4` | Revised Validation Procedure | 0 | reviewed | Revised Validation Procedure. A heading with no body text; its two subsections carry the sites. |
| `4.1` | Revision of Route Feasibility | 1 | reviewed | Revision of Route Feasibility. One MUST-level site, 4.1:1, the redefined step (b), mapped to RFC9117-4.1-1. The SHOULD (condition b.2 enabled by default) and the two MAYs (disable b.2 by configuration; permit a non-empty AS_PATH by policy) are rows RFC9117-4.1-2, 4.1-3 and 4.1-4, not gated. The Explanation paragraphs are indicative. |
| `4.2` | Revision of AS_PATH Validation | 2 | reviewed | Revision of AS_PATH Validation. Site 4.2:1 quotes the RFC 8955 Section 6 sentence this section replaces; site 4.2:2 is the replacement rule, mapped to RFC9117-4.2-1. The Explanation paragraphs are indicative, including the remark that the original rule's enforcement remains optional per RFC 4271 Section 6.3. |
| `5` | Topology Considerations | 0 | reviewed | Topology Considerations. Indicative: congruent topology between unicast and FlowSpec routes, and how condition (b.2) supports non-congruent topologies inside the Local Domain. The lowercase 'should be designed' is advice to network designers, not to a speaker. |
| `6` | IANA Considerations: no IANA actions | 0 | reviewed | IANA Considerations: no IANA actions. |
| `7` | Security Considerations | 0 | reviewed | Security Considerations. The capitalised OPTIONAL describes what Section 4.2 did to the RFC 4271 Section 6.3 rule. The SHOULD and SHOULD NOT advise enforcing that optional rule only when configuration indicates the peer is not a route server; both are recommendations about an optional rule, below the MUST-level the inventory counts. No MUST-level keyword. |
| `8` | References heading | 0 | skipped (references) | References heading. |
| `8.1` | Normative References | 0 | skipped (references) | Normative References. |
| `8.2` | Informative References | 0 | skipped (references) | Informative References. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A quotation: the section opens 'Section 6 of [RFC8955] states:' and cites this sentence in order to replace it. This document does not impose it; the replacement is site 4.2:2. | \| BGP implementations MUST also enforce that the AS_PATH \| attribute of a route received via the External Border Gateway \| Protocol (eBGP) contains the neighboring AS in the left-most \| position of the AS_PATH attribute. |

## Superseded

No document obsoletes RFC 9117, so its obligations are stated where they were written.
