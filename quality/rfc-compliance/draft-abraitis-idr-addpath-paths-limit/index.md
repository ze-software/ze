# DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT - Scalability Considerations for ADD-PATH with PATHS-LIMIT

BGP PATHS-LIMIT. Every requirement listed for DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 4 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 4 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 4 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 4 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 17 | 17 of 29 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 7 | of 4 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 4 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 4 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 4 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 4 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 4 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 4 | of 7 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 4 mandatory requirements: they add up to 4. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP PATHS-LIMIT |
| Automatic checks | Included |
| Requirements | 7 |
| Mandatory requirements (MUST level) | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 29 |
| Tests linked to requirements | 29 |
| Recorded audit verdicts | 7 |
| Fault test records | 17 |
| Summary | `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` |
| Requirement table | `rfc/requirements/draft-abraitis-idr-addpath-paths-limit.md` |
| RFC text | `rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt` |

## Automatic checks

Included in automatic checks: BGP ADD-PATH Paths-Limit capability (code 76): four MUST-level requirements with positive and negative test carriers. For 3-1, coalescePathsLimit in internal/component/bgp/reactor/session_negotiate.go merges configuration and plugin declarations; TestBuildOpenCoalescesPathsLimit checks multiple families in one emitted instance and prevents repeated input instances from leaking into OPEN. For 3-2 and 3-3, core capability negotiation tests cover ADD-PATH present/absent and matching/unmatched families; for 3-4, TestParsePathsLimitDuplicateFirstWins covers retaining the first tuple and ignoring later duplicates. SHOULD-level zero handling (3-5) and session-wide sender enforcement (3-6) have separate parser, negotiation, and sender regression carriers.

## Published support status

**Area:** BGP PATHS-LIMIT

**Test verification:** 1 of 4 MUST proven

**Evidence:** unit 1

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **4** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (4):** [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1`](#draft-abraitis-idr-addpath-paths-limit-3-1), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2`](#draft-abraitis-idr-addpath-paths-limit-3-2), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3`](#draft-abraitis-idr-addpath-paths-limit-3-3), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4`](#draft-abraitis-idr-addpath-paths-limit-3-4)

## Figures

- **MUST bar**: Ze verified 1, Tests awaiting verification 3
- **Overview bar**: Ze MUST verified 1, SHOULD verified 3, Tests awaiting verification 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze verified | 1 |
| OS verified | 0 |
| Tests awaiting verification | 3 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD verified | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/draft-abraitis-idr-addpath-paths-limit.json](../../../data/rfc/draft-abraitis-idr-addpath-paths-limit.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1` | A BGP speaker that wishes to indicate support for multiple AFI/SAFIs MUST do so by including the information in a single instance of the PATHS-LIMIT capability. (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L355). **positive:** `unit/verify` [`TestParsePeerCapabilityPathsLimitSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L604). **negative:** `unit/verify` [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L356) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2` | The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present. (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimit`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L631). **negative:** `unit/verify` [`TestNegotiatePathsLimitNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L686) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3` | An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability. (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L712). **negative:** `unit/verify` [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L713) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4` | If more than one tuple is received for the same AFI/SAFI pair, only the first tuple should be considered.  All others MUST be ignored. (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L934). **negative:** `unit/verify` [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L935) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-5` | If the received Paths Limit is zero (0), the tuple SHOULD be ignored. (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L787). **positive:** `unit/verify` [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L902). **negative:** `unit/verify` [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L788). **negative:** `unit/verify` [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L903). **negative:** `unit/verify` [`TestParsePathsLimitZeroFirst`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L962) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6` | A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability. (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L206). **positive:** `unit/verify` [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L157). **negative:** `unit/verify` [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L207). **negative:** `unit/verify` [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L158). **positive:** `functional/verify` [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L9). **negative:** `functional/verify` [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L10) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7` | An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender. (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_config_test.go#L13). **positive:** `unit/verify` [`TestParsePeerCapabilityPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L564). **positive:** `unit/verify` [`TestProcessMessageRefusesPathOverReceiveCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L51). **positive:** `unit/verify` [`TestReceiveCeilingAdmitsUpToLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L22). **negative:** `unit/verify` [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_config_test.go#L14). **negative:** `unit/verify` [`TestProcessMessageRefusesPathOverReceiveCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L52). **negative:** `unit/verify` [`TestReceiveCeilingAdmitsUpToLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L23). **positive:** `functional/verify` [`paths-limit-receive-ceiling.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-receive-ceiling.ci#L20). **negative:** `functional/verify` [`paths-limit-receive-ceiling.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-receive-ceiling.ci#L21) |

## Gaps and untested MUSTs

DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1`](#draft-abraitis-idr-addpath-paths-limit-3-1)

A BGP speaker that wishes to indicate support for multiple AFI/SAFIs MUST do so by including the information in a single instance of the PATHS-LIMIT capability. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-27 by an independent judge (authored none of the change) after phase 1 of spec-add-path-limit-send-receive respelled the config. Section 3: "A BGP speaker that wishes to indicate support for multiple AFI/SAFIs MUST do so by including the information in a single instance of the PATHS-LIMIT capability." TestBuildOpenCoalescesPathsLimit is unchanged and remains the proof: it drives buildOpen with several PATHS-LIMIT inputs (an empty configured instance, a configured ipv4/unicast tuple, two raw code-76 plugin payloads with a duplicate and a zero tuple), re-parses the encoded OPEN, and asserts require.Len(limits, 1) (negative) and the exact two-entry content [ipv4/unicast 4, ipv4/multicast 3] (positive); coalescePathsLimit in session_negotiate.go is the enforcing function. The only change to TestParsePeerCapabilityPathsLimitSingleInstance is the per-family limit spelling: "limit": "10"/"20" became receiveLimit("10")/("20"), i.e. {"receive": "N"}, which addPathReceiveLimit parses to the same uint16 the deleted parseUint16 returned; its assertion of one *capability.PathsLimit holding both families is byte-identical and passes. It stays a subordinate config-path guard one hop short of the wire, as previously judged. Enforced stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L356) | unit/verify | mutant, verified |
| positive | [`TestParsePeerCapabilityPathsLimitSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L604) | unit/verify | not verified |
| positive | [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L355) | unit/verify | mutant, verified |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2`](#draft-abraitis-idr-addpath-paths-limit-3-2)

The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged on 2026-09-20 by an independent reader, after Negotiate's signature moved from (local, remote []Capability, localASN, peerASN uint32) to (local, remote []Capability, identity PeerIdentity). Inside each unit named here the change is one line, the call shape, and every assertion is byte-identical (git diff HEAD over internal/core/bgp/capability/negotiated_test.go). The new shape cannot move what these cases prove: Negotiate reads identity only where it stores LocalASN and PeerASN on the Negotiated, and where buildSubComponents copies the struct into n.Identity. No branch in Negotiate, and none in negotiatePathsLimitDirection, reads Internal or either AS number, so the iBGP verdict the struct now carries has no path to the PATHS-LIMIT maps. The identity each call passes is self-consistent too: 65001 against 65002 with Internal left false is an external session, which is what the two positional AS numbers meant before. Section 3: "The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present." TestNegotiatePathsLimit is the positive: ADD-PATH both-mode for ipv4/unicast on both sides, and the two directions are pinned on exact, distinct values, the remote's 10 in PathsLimitSend and the local's 5 in PathsLimitRecv, so a direction swap and a floor reading each fail it. TestNegotiatePathsLimitNoAddPath is the negative: the same two PATHS-LIMIT capabilities with no ADD-PATH on either side, and both maps must stay at zero. The producing branch is "n.addPath[f]&direction == 0" in negotiatePathsLimitDirection (internal/core/bgp/capability/negotiated.go); remove it and the negative reads 10 and 5, so the assertion discriminates. The negative buffer holds Multiprotocol and PathsLimit only, so no neighbouring rule decides it. Ze is stricter than the sentence, because it requires the negotiated DIRECTION and not the bare presence of ADD-PATH. A limit on paths the session will never carry changes nothing on the wire, so the stricter reading implements the requirement rather than falling short of it. No discrimination record: both tags predate HEAD and the gate grandfathers them. 2026-09-26 RE-AUDIT, stale-requirement: the checklist line lost its quotation marks and gained the sentence's full stop, so it is now a verbatim span of Section 3 of rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt. The obligation is the sentence this note already quotes, and the tagged units are byte-identical (TestNegotiatePathsLimitNoAddPath re-read this pass: both PATHS-LIMIT maps must stay zero with no ADD-PATH). Verdict unchanged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestNegotiatePathsLimitNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L686) | unit/verify | not verified |
| positive | [`TestNegotiatePathsLimit`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L631) | unit/verify | not verified |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3`](#draft-abraitis-idr-addpath-paths-limit-3-3)

An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged on 2026-09-20 by an independent reader, after Negotiate's signature moved from (local, remote []Capability, localASN, peerASN uint32) to (local, remote []Capability, identity PeerIdentity). Inside each unit named here the change is one line, the call shape, and every assertion is byte-identical (git diff HEAD over internal/core/bgp/capability/negotiated_test.go). The new shape cannot move what these cases prove: Negotiate reads identity only where it stores LocalASN and PeerASN on the Negotiated, and where buildSubComponents copies the struct into n.Identity. No branch in Negotiate, and none in negotiatePathsLimitDirection, reads Internal or either AS number, so the iBGP verdict the struct now carries has no path to the PATHS-LIMIT maps. The identity each call passes is self-consistent too: 65001 against 65002 with Internal left false is an external session, which is what the two positional AS numbers meant before. Section 3: "An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability." TestNegotiatePathsLimitPartialAddPath carries both polarities on two distinct assertions, not one assertion wearing two hats: ipv4/unicast is present in ADD-PATH, so its tuple applies in both directions and is pinned at 10 and 5, and ipv6/unicast is absent from ADD-PATH while its PATHS-LIMIT tuple sits in the same capability, so both directions must stay zero. The buffer isolates this rule: both sides advertise Multiprotocol for ipv6/unicast, so ipv6 is a negotiated family and the only thing that can drop its tuple is the ADD-PATH check. Were ipv6 missing from Multiprotocol, the family intersection would decide the case and the test would prove nothing here. Same producing branch as 3-2, evaluated per family; remove it and ipv6 reads 8 and 3. No discrimination record: the tags predate HEAD. 2026-09-26 RE-AUDIT, stale-requirement: the checklist line lost its quotation marks and gained the sentence's full stop, so it is now a verbatim span of Section 3 of rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt. The obligation is the sentence this note already quotes, and the tagged units are byte-identical (TestNegotiatePathsLimitNoAddPath re-read this pass: both PATHS-LIMIT maps must stay zero with no ADD-PATH). Verdict unchanged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L713) | unit/verify | not verified |
| positive | [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L712) | unit/verify | not verified |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4`](#draft-abraitis-idr-addpath-paths-limit-3-4)

If more than one tuple is received for the same AFI/SAFI pair, only the first tuple should be considered.  All others MUST be ignored. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestParsePathsLimitDuplicateFirstWins parses two ipv4/unicast tuples, limit 10 then limit 20, and requires one entry whose limit is 10. The positive is the kept value and the negative is the count, which are separate assertions over the same buffer. Removing the "seen[f]" guard in parsePathsLimit (internal/core/bgp/capability/capability.go) yields two entries, so the count assertion discriminates. No discrimination record: the tags predate HEAD. Re-judged 2026-09-27 by QF-2 after the row was replaced by the verbatim §3 sentence; the obligation is the one the note above judged, and the tagged units are unchanged. The quote adds the sentence's lowercase 'should' for the first-tuple half; TestParsePathsLimitDuplicateFirstWins asserts one entry with limit 10 from tuples 10 then 20, so keeping the second or both goes red. Enforced stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L935) | unit/verify | not verified |
| positive | [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L934) | unit/verify | not verified |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-5`](#draft-abraitis-idr-addpath-paths-limit-3-5)

If the received Paths Limit is zero (0), the tuple SHOULD be ignored. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged on 2026-09-20 by an independent reader, after Negotiate's signature moved from (local, remote []Capability, localASN, peerASN uint32) to (local, remote []Capability, identity PeerIdentity). Inside each unit named here the change is one line, the call shape, and every assertion is byte-identical (git diff HEAD over internal/core/bgp/capability/negotiated_test.go). The new shape cannot move what these cases prove: Negotiate reads identity only where it stores LocalASN and PeerASN on the Negotiated, and where buildSubComponents copies the struct into n.Identity. No branch in Negotiate, and none in negotiatePathsLimitDirection, reads Internal or either AS number, so the iBGP verdict the struct now carries has no path to the PATHS-LIMIT maps. The identity each call passes is self-consistent too: 65001 against 65002 with Internal left false is an external session, which is what the two positional AS numbers meant before. Section 3: "If the received Paths Limit is zero (0), the tuple SHOULD be ignored." The two parser carriers were re-read and are unchanged in the tree. parsePathsLimit marks a family seen BEFORE it skips a zero limit, which the draft requires: the first tuple is the one considered, and "All others MUST be ignored", so a first tuple of zero leaves that family with no limit whatever a later duplicate says. TestParsePathsLimitSkipZero pins the plain skip on exact entries, one entry holding ipv6 with 5, and TestParsePathsLimitZeroFirst pins that interplay on the exact entry slice, ipv4/unicast absent even though a later duplicate advertises 10. TestNegotiatePathsLimitDuplicateEntries runs one set of tuples through the direct API and through the wire parser and requires both negotiated maps to equal exactly {ipv6: 7}, with an explicit NotContains on ipv4. That assertion is a genuine 3-5 negative and not a 3-4 one in disguise: the ipv4 tuple that arrives FIRST is the zero, so an implementation that kept first-wins and dropped the zero skip would store ipv4 at 0 and the map equality would fail. All three units carry a discrimination record over "limit == 0" -> false in parsePathsLimit, under which each was observed red. Re-judged 2026-09-27 by QF-2 after the row was replaced by the verbatim §3 sentence; the obligation is the one the note above judged, and the tagged units are unchanged. Enforced stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L903) | unit/verify | mutant, verified |
| negative | [`TestParsePathsLimitZeroFirst`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L962) | unit/verify | mutant, verified |
| negative | [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L788) | unit/verify | not verified |
| positive | [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L902) | unit/verify | mutant, verified |
| positive | [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L787) | unit/verify | not verified |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6`](#draft-abraitis-idr-addpath-paths-limit-3-6)

A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-10-31 by an auditor who authored none of the change. Draft Section 3: "A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability." test/plugin/paths-limit-live.ci changed since the last stamp only in setup: 5cf82621ba added the system authorization profile admin block, and 7c22ceb0d0 added a bgp-level filter {import {final accept;} export {final accept;}} so RFC 8212 default-reject does not withhold the routes the assertions read. The bound under test still comes from the peer script's OPEN (paths-limit ipv4/unicast limit=1, ipv6/unicast limit=2); every expect hex, sequence group, sentinel and action line is unchanged, so the negative and positive read as before. Verdict unchanged. Earlier judgment follows. Re-judged 2026-10-27 by an independent judge (authored none of the change). Section 3: "A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability." The only change to test/plugin/paths-limit-live.ci is the daemon's container-level config line `limit 9` -> `limit { receive 9; }`: that is ze's own receive request, incidental to this row, while the bound under test comes from the peer script's OPEN (option=open paths-limit ipv4/unicast limit=1, ipv6/unicast limit=2), which is unchanged. Every expect hex, sequence group, sentinel and action line is byte-identical to HEAD, so the negative (an excess identifier absent before the next different-prefix sentinel, admitted only after withdrawal and explicit retry) and the positive (admitted, replaced and reused identifiers reach the wire, per family) read exactly as before. The two Go units are unchanged. Ran green on the current tree, which carries phase 3's move of the send filter into pathsLimitAdmission: ./le test bgp plugin paths-limit-live 1/1 PASS. Separately, ./le rfc check reports this row's discrimination records producer-gone because phase 3 moved pathsLimitSection; that is a stale proof record owed a re-record, not a change to what the tests assert. Enforced stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L207) | unit/verify | mutant, verified |
| negative | [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L158) | unit/verify | mutant, verified |
| negative | [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L10) | functional/verify | mutant, verified |
| positive | [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L206) | unit/verify | mutant, verified |
| positive | [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L157) | unit/verify | mutant, verified |
| positive | [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L9) | functional/verify | mutant, verified |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7`](#draft-abraitis-idr-addpath-paths-limit-3-7)

An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (draft-abraitis-idr-addpath-paths-limit): "An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender." The only edit to test/plugin/paths-limit-receive-ceiling.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_config_test.go#L14) | unit/verify | not verified |
| negative | [`TestProcessMessageRefusesPathOverReceiveCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L52) | unit/verify | mutant, verified |
| negative | [`TestReceiveCeilingAdmitsUpToLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L23) | unit/verify | mutant, verified |
| negative | [`paths-limit-receive-ceiling.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-receive-ceiling.ci#L21) | functional/verify | mutant, verified |
| positive | [`TestParsePeerCapabilityPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L564) | unit/verify | not verified |
| positive | [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_config_test.go#L13) | unit/verify | not verified |
| positive | [`TestProcessMessageRefusesPathOverReceiveCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L51) | unit/verify | mutant, verified |
| positive | [`TestReceiveCeilingAdmitsUpToLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L22) | unit/verify | mutant, verified |
| positive | [`paths-limit-receive-ceiling.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-receive-ceiling.ci#L20) | functional/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, draft-abraitis-idr-addpath-paths-limit |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt |
| Source fingerprint | 87330146d8f7b5c0 |
| Record | rfc/extraction/draft-abraitis-idr-addpath-paths-limit.json |
| Mapped sentences | 4 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates section 1 and directs no speaker. |
| `1` | Introduction | 0 | reviewed | Introduction. Indicative prose: ADD-PATH lets a speaker advertise several paths for one prefix, a large number of such paths can exhaust the receiver's memory, and this document defines a capability that tells the sender the receiver's ceiling. No sentence directs a speaker. |
| `2` | Specification of Requirements | 0 | reviewed | Specification of Requirements. The BCP 14 key-words paragraph, which binds no speaker and states how to read the other sections. The derivation excludes it from the site inventory for that reason. |
| `3` | PATHS-LIMIT Capability | 4 | reviewed | PATHS-LIMIT Capability. The document's only normative section: Capability Code 76, the 5-octet AFI/SAFI/Paths-Limit tuple, and four capitalised MUST-level sites mapped below to DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1 through -3-4. Its three remaining directives are advisory and carry no site: ignore a tuple whose Paths Limit is zero, send no more than the advertised maximum, and offer a configuration knob. Those are the unsourced ids below. Two indicative sentences state no obligation: the Paths Limit field description, and "If the PATHS-LIMIT capability is empty (i.e. the Capability Length field is set to 0), it means that the sender doesn't have any specific limits to communicate", which defines what an empty capability means rather than directing a speaker. |
| `4` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records capability number 76 in the BGP Capability Codes registry. Binds IANA, not a speaker. |
| `5` | not stated | 0 | reviewed | Security Considerations, and with it the unnumbered Acknowledgements, References and Authors' Addresses blocks, which carry no section number and so stay in this body under the heading derivation. The security text states that PATHS-LIMIT mitigates some RFC 7911 concerns and that a rogue or misconfigured node can advertise a limit too low for the application. Its one direction, "Users of the PATHS-LIMIT Capability are encouraged to examine the behavior and potential impact", is an encouragement with no RFC 2119 keyword and binds no speaker. |
| `A` | Implementation Report | 0 | skipped (appendix-non-normative) | Implementation Report. An RFC 7942 note naming the FRRouting commit that implements the draft. It reports on another implementation and states no obligation. |

### Excluded sentences

The review of DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT, so its obligations are stated where they were written.
