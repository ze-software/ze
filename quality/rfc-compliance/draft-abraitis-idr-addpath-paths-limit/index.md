# DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT - Scalability Considerations for ADD-PATH with PATHS-LIMIT

Supported. Every requirement this repository extracted from DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 4 of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 4 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 58.6% | 17 of 29 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 7 | of 4 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 4 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

The 9 shares marked as a part above are the whole of the 4 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 29 |
| Tagged units | 29 |
| Recorded audit verdicts | 7 |
| Discrimination records | 17 |
| Summary | `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` |
| Requirement shard | `rfc/requirements/draft-abraitis-idr-addpath-paths-limit.md` |
| RFC text | `rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt` |

## Enrolment

Enrolled: BGP ADD-PATH Paths-Limit capability (code 76): four MUST-level requirements with positive and negative test carriers. For 3-1, coalescePathsLimit in internal/component/bgp/reactor/session_negotiate.go merges configuration and plugin declarations; TestBuildOpenCoalescesPathsLimit checks multiple families in one emitted instance and prevents repeated input instances from leaking into OPEN. For 3-2 and 3-3, core capability negotiation tests cover ADD-PATH present/absent and matching/unmatched families; for 3-4, TestParsePathsLimitDuplicateFirstWins covers retaining the first tuple and ignoring later duplicates. SHOULD-level zero handling (3-5) and session-wide sender enforcement (3-6) have separate parser, negotiation, and sender regression carriers.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

Receiver-advertised per-family path-count requests for ADD-PATH, with session-wide outbound enforcement across normal sends and forwarding, including the route-server fast path (`pathsLimitAdmission.section` in [`internal/component/bgp/reactor/session_paths_limit.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_paths_limit.go)). Under the peer's limit Ze sends the paths the bgp-rib election ranks first and withdraws a demoted path before announcing its replacement (`queuePathsLimitElection` in `session_paths_limit_elect.go`). The configuration knob `limit { receive N; }` is advertised and enforced on received paths: an excess path is dropped, counted and warned once per route, and the rest of its UPDATE installs (`receiveCeiling` in `session_paths_limit_recv.go`).

**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1`](#draft-abraitis-idr-addpath-paths-limit-3-1), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2`](#draft-abraitis-idr-addpath-paths-limit-3-2), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3`](#draft-abraitis-idr-addpath-paths-limit-3-3), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4`](#draft-abraitis-idr-addpath-paths-limit-3-4)

## Attainable ceiling

Mandatory ceiling: 4 of 4 (100.0%). Optional ceiling: 3 of 3 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: draft-abraitis-idr-addpath-paths-limit 4 of 4 (100.0%). Lowest optional ceiling: draft-abraitis-idr-addpath-paths-limit 3 of 3 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1` | A BGP speaker that wishes to indicate support for multiple AFI/SAFIs MUST do so by including the information in a single instance of the PATHS-LIMIT capability. (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L354). **positive:** `unit/verify` [`TestParsePeerCapabilityPathsLimitSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L604). **negative:** `unit/verify` [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L355) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2` | The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present. (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimit`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L630). **negative:** `unit/verify` [`TestNegotiatePathsLimitNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L685) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3` | An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability. (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L711). **negative:** `unit/verify` [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L712) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4` | If more than one tuple is received for the same AFI/SAFI pair, only the first tuple should be considered.  All others MUST be ignored. (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L934). **negative:** `unit/verify` [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L935) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-5` | If the received Paths Limit is zero (0), the tuple SHOULD be ignored. (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L786). **positive:** `unit/verify` [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L902). **negative:** `unit/verify` [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L787). **negative:** `unit/verify` [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L903). **negative:** `unit/verify` [`TestParsePathsLimitZeroFirst`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L962) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6` | A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability. (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L206). **positive:** `unit/verify` [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L157). **negative:** `unit/verify` [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L207). **negative:** `unit/verify` [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L158). **positive:** `functional/verify` [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L9). **negative:** `functional/verify` [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L10) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7` | An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender. (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_config_test.go#L13). **positive:** `unit/verify` [`TestParsePeerCapabilityPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L564). **positive:** `unit/verify` [`TestProcessMessageRefusesPathOverReceiveCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L51). **positive:** `unit/verify` [`TestReceiveCeilingAdmitsUpToLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L22). **negative:** `unit/verify` [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_config_test.go#L14). **negative:** `unit/verify` [`TestProcessMessageRefusesPathOverReceiveCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L52). **negative:** `unit/verify` [`TestReceiveCeilingAdmitsUpToLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L23). **positive:** `functional/verify` [`paths-limit-receive-ceiling.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-receive-ceiling.ci#L20). **negative:** `functional/verify` [`paths-limit-receive-ceiling.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-receive-ceiling.ci#L21) |

## Gaps and untested MUSTs

DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1`](#draft-abraitis-idr-addpath-paths-limit-3-1)

A BGP speaker that wishes to indicate support for multiple AFI/SAFIs MUST do so by including the information in a single instance of the PATHS-LIMIT capability. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-27 by an independent judge (authored none of the change) after phase 1 of spec-add-path-limit-send-receive respelled the config. Section 3: "A BGP speaker that wishes to indicate support for multiple AFI/SAFIs MUST do so by including the information in a single instance of the PATHS-LIMIT capability." TestBuildOpenCoalescesPathsLimit is unchanged and remains the proof: it drives buildOpen with several PATHS-LIMIT inputs (an empty configured instance, a configured ipv4/unicast tuple, two raw code-76 plugin payloads with a duplicate and a zero tuple), re-parses the encoded OPEN, and asserts require.Len(limits, 1) (negative) and the exact two-entry content [ipv4/unicast 4, ipv4/multicast 3] (positive); coalescePathsLimit in session_negotiate.go is the enforcing function. The only change to TestParsePeerCapabilityPathsLimitSingleInstance is the per-family limit spelling: "limit": "10"/"20" became receiveLimit("10")/("20"), i.e. {"receive": "N"}, which addPathReceiveLimit parses to the same uint16 the deleted parseUint16 returned; its assertion of one *capability.PathsLimit holding both families is byte-identical and passes. It stays a subordinate config-path guard one hop short of the wire, as previously judged. Enforced stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L355) | unit/verify | mutant, verified |
| positive | [`TestParsePeerCapabilityPathsLimitSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L604) | unit/verify | unproven |
| positive | [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L354) | unit/verify | mutant, verified |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2`](#draft-abraitis-idr-addpath-paths-limit-3-2)

The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged on 2026-09-20 by an independent reader, after Negotiate's signature moved from (local, remote []Capability, localASN, peerASN uint32) to (local, remote []Capability, identity PeerIdentity). Inside each unit named here the change is one line, the call shape, and every assertion is byte-identical (git diff HEAD over internal/core/bgp/capability/negotiated_test.go). The new shape cannot move what these cases prove: Negotiate reads identity only where it stores LocalASN and PeerASN on the Negotiated, and where buildSubComponents copies the struct into n.Identity. No branch in Negotiate, and none in negotiatePathsLimitDirection, reads Internal or either AS number, so the iBGP verdict the struct now carries has no path to the PATHS-LIMIT maps. The identity each call passes is self-consistent too: 65001 against 65002 with Internal left false is an external session, which is what the two positional AS numbers meant before. Section 3: "The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present." TestNegotiatePathsLimit is the positive: ADD-PATH both-mode for ipv4/unicast on both sides, and the two directions are pinned on exact, distinct values, the remote's 10 in PathsLimitSend and the local's 5 in PathsLimitRecv, so a direction swap and a floor reading each fail it. TestNegotiatePathsLimitNoAddPath is the negative: the same two PATHS-LIMIT capabilities with no ADD-PATH on either side, and both maps must stay at zero. The producing branch is "n.addPath[f]&direction == 0" in negotiatePathsLimitDirection (internal/core/bgp/capability/negotiated.go); remove it and the negative reads 10 and 5, so the assertion discriminates. The negative buffer holds Multiprotocol and PathsLimit only, so no neighbouring rule decides it. Ze is stricter than the sentence, because it requires the negotiated DIRECTION and not the bare presence of ADD-PATH. A limit on paths the session will never carry changes nothing on the wire, so the stricter reading implements the requirement rather than falling short of it. No discrimination record: both tags predate HEAD and the gate grandfathers them. 2026-09-26 RE-AUDIT, stale-requirement: the checklist line lost its quotation marks and gained the sentence's full stop, so it is now a verbatim span of Section 3 of rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt. The obligation is the sentence this note already quotes, and the tagged units are byte-identical (TestNegotiatePathsLimitNoAddPath re-read this pass: both PATHS-LIMIT maps must stay zero with no ADD-PATH). Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiatePathsLimitNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L685) | unit/verify | unproven |
| positive | [`TestNegotiatePathsLimit`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L630) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3`](#draft-abraitis-idr-addpath-paths-limit-3-3)

An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged on 2026-09-20 by an independent reader, after Negotiate's signature moved from (local, remote []Capability, localASN, peerASN uint32) to (local, remote []Capability, identity PeerIdentity). Inside each unit named here the change is one line, the call shape, and every assertion is byte-identical (git diff HEAD over internal/core/bgp/capability/negotiated_test.go). The new shape cannot move what these cases prove: Negotiate reads identity only where it stores LocalASN and PeerASN on the Negotiated, and where buildSubComponents copies the struct into n.Identity. No branch in Negotiate, and none in negotiatePathsLimitDirection, reads Internal or either AS number, so the iBGP verdict the struct now carries has no path to the PATHS-LIMIT maps. The identity each call passes is self-consistent too: 65001 against 65002 with Internal left false is an external session, which is what the two positional AS numbers meant before. Section 3: "An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability." TestNegotiatePathsLimitPartialAddPath carries both polarities on two distinct assertions, not one assertion wearing two hats: ipv4/unicast is present in ADD-PATH, so its tuple applies in both directions and is pinned at 10 and 5, and ipv6/unicast is absent from ADD-PATH while its PATHS-LIMIT tuple sits in the same capability, so both directions must stay zero. The buffer isolates this rule: both sides advertise Multiprotocol for ipv6/unicast, so ipv6 is a negotiated family and the only thing that can drop its tuple is the ADD-PATH check. Were ipv6 missing from Multiprotocol, the family intersection would decide the case and the test would prove nothing here. Same producing branch as 3-2, evaluated per family; remove it and ipv6 reads 8 and 3. No discrimination record: the tags predate HEAD. 2026-09-26 RE-AUDIT, stale-requirement: the checklist line lost its quotation marks and gained the sentence's full stop, so it is now a verbatim span of Section 3 of rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt. The obligation is the sentence this note already quotes, and the tagged units are byte-identical (TestNegotiatePathsLimitNoAddPath re-read this pass: both PATHS-LIMIT maps must stay zero with no ADD-PATH). Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L712) | unit/verify | unproven |
| positive | [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L711) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4`](#draft-abraitis-idr-addpath-paths-limit-3-4)

If more than one tuple is received for the same AFI/SAFI pair, only the first tuple should be considered.  All others MUST be ignored. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestParsePathsLimitDuplicateFirstWins parses two ipv4/unicast tuples, limit 10 then limit 20, and requires one entry whose limit is 10. The positive is the kept value and the negative is the count, which are separate assertions over the same buffer. Removing the "seen[f]" guard in parsePathsLimit (internal/core/bgp/capability/capability.go) yields two entries, so the count assertion discriminates. No discrimination record: the tags predate HEAD. Re-judged 2026-09-27 by QF-2 after the row was replaced by the verbatim §3 sentence; the obligation is the one the note above judged, and the tagged units are unchanged. The quote adds the sentence's lowercase 'should' for the first-tuple half; TestParsePathsLimitDuplicateFirstWins asserts one entry with limit 10 from tuples 10 then 20, so keeping the second or both goes red. Enforced stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L935) | unit/verify | unproven |
| positive | [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L934) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-5`](#draft-abraitis-idr-addpath-paths-limit-3-5)

If the received Paths Limit is zero (0), the tuple SHOULD be ignored. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged on 2026-09-20 by an independent reader, after Negotiate's signature moved from (local, remote []Capability, localASN, peerASN uint32) to (local, remote []Capability, identity PeerIdentity). Inside each unit named here the change is one line, the call shape, and every assertion is byte-identical (git diff HEAD over internal/core/bgp/capability/negotiated_test.go). The new shape cannot move what these cases prove: Negotiate reads identity only where it stores LocalASN and PeerASN on the Negotiated, and where buildSubComponents copies the struct into n.Identity. No branch in Negotiate, and none in negotiatePathsLimitDirection, reads Internal or either AS number, so the iBGP verdict the struct now carries has no path to the PATHS-LIMIT maps. The identity each call passes is self-consistent too: 65001 against 65002 with Internal left false is an external session, which is what the two positional AS numbers meant before. Section 3: "If the received Paths Limit is zero (0), the tuple SHOULD be ignored." The two parser carriers were re-read and are unchanged in the tree. parsePathsLimit marks a family seen BEFORE it skips a zero limit, which the draft requires: the first tuple is the one considered, and "All others MUST be ignored", so a first tuple of zero leaves that family with no limit whatever a later duplicate says. TestParsePathsLimitSkipZero pins the plain skip on exact entries, one entry holding ipv6 with 5, and TestParsePathsLimitZeroFirst pins that interplay on the exact entry slice, ipv4/unicast absent even though a later duplicate advertises 10. TestNegotiatePathsLimitDuplicateEntries runs one set of tuples through the direct API and through the wire parser and requires both negotiated maps to equal exactly {ipv6: 7}, with an explicit NotContains on ipv4. That assertion is a genuine 3-5 negative and not a 3-4 one in disguise: the ipv4 tuple that arrives FIRST is the zero, so an implementation that kept first-wins and dropped the zero skip would store ipv4 at 0 and the map equality would fail. All three units carry a discrimination record over "limit == 0" -> false in parsePathsLimit, under which each was observed red. Re-judged 2026-09-27 by QF-2 after the row was replaced by the verbatim §3 sentence; the obligation is the one the note above judged, and the tagged units are unchanged. Enforced stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L903) | unit/verify | mutant, verified |
| negative | [`TestParsePathsLimitZeroFirst`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L962) | unit/verify | mutant, verified |
| negative | [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L787) | unit/verify | unproven |
| positive | [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L902) | unit/verify | mutant, verified |
| positive | [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L786) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6`](#draft-abraitis-idr-addpath-paths-limit-3-6)

A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-27 by an independent judge (authored none of the change). Section 3: "A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability." The only change to test/plugin/paths-limit-live.ci is the daemon's container-level config line `limit 9` -> `limit { receive 9; }`: that is ze's own receive request, incidental to this row, while the bound under test comes from the peer script's OPEN (option=open paths-limit ipv4/unicast limit=1, ipv6/unicast limit=2), which is unchanged. Every expect hex, sequence group, sentinel and action line is byte-identical to HEAD, so the negative (an excess identifier absent before the next different-prefix sentinel, admitted only after withdrawal and explicit retry) and the positive (admitted, replaced and reused identifiers reach the wire, per family) read exactly as before. The two Go units are unchanged. Ran green on the current tree, which carries phase 3's move of the send filter into pathsLimitAdmission: ./le test bgp plugin paths-limit-live 1/1 PASS. Separately, ./le rfc check reports this row's discrimination records producer-gone because phase 3 moved pathsLimitSection; that is a stale proof record owed a re-record, not a change to what the tests assert. Enforced stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L207) | unit/verify | mutant, verified |
| negative | [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L158) | unit/verify | mutant, verified |
| negative | [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L10) | functional/verify | mutant, verified |
| positive | [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L206) | unit/verify | mutant, verified |
| positive | [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_session_test.go#L157) | unit/verify | mutant, verified |
| positive | [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L9) | functional/verify | mutant, verified |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7`](#draft-abraitis-idr-addpath-paths-limit-3-7)

An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-27 by an independent judge (authored none of the tests). Section 3: "An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender." Both halves are now proven. Knob: TestParsePeerCapabilityPathsLimit and TestConfiguredPathsLimitOpen (unchanged) show limit { receive N; } at container and per-family level decides the advertised tuples, an omitted limit advertising none. Accept ceiling: the knob reaches session_negotiate.go initPathsLimitRecv(Encoding.PathsLimitRecv), and session_read.go calls receiveCeiling before checkPrefixLimits and delivery, which filters NLRIs admitPath refuses at len(prefix.ids) >= f.limit. test/plugin/paths-limit-receive-ceiling.ci drives it end to end from config: with receive 2, the plugin sees ids 1 and 2 of 10.0.0.0/8 plus 20.0.0.0/8 in one exact event line (positive, also proving the other prefix is untouched), id 3 only after a withdrawal of admitted id 1 frees a slot, and never id 4 (negative; reject=path-id 4). TestProcessMessageRefusesPathOverReceiveCeiling asserts the exact delivered path list through processMessage and the admitted/refused counts 2/1; TestReceiveCeilingAdmitsUpToLimit asserts exact admitted ids [1 2 7], refused counter 1, and a replacement at the ceiling kept. Positive inputs are conforming (at or under the ceiling), negatives exceed this ceiling and no neighbouring rule; assertions are exact, not floors. Discrimination records for all six tags observed red under admitPath breaks: ceiling check -> false (negatives) and limit -> limit-1 (positives).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_config_test.go#L14) | unit/verify | unproven |
| negative | [`TestProcessMessageRefusesPathOverReceiveCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L52) | unit/verify | mutant, verified |
| negative | [`TestReceiveCeilingAdmitsUpToLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L23) | unit/verify | mutant, verified |
| negative | [`paths-limit-receive-ceiling.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-receive-ceiling.ci#L21) | functional/verify | mutant, verified |
| positive | [`TestParsePeerCapabilityPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L564) | unit/verify | unproven |
| positive | [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_config_test.go#L13) | unit/verify | unproven |
| positive | [`TestProcessMessageRefusesPathOverReceiveCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L51) | unit/verify | mutant, verified |
| positive | [`TestReceiveCeilingAdmitsUpToLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_receive_test.go#L22) | unit/verify | mutant, verified |
| positive | [`paths-limit-receive-ceiling.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-receive-ceiling.ci#L20) | functional/verify | mutant, verified |

## Extraction sign-off

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
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: ADD-PATH lets a speaker advertise several paths for one prefix, a large number of such paths can exhaust the receiver's memory, and this document defines a capability that tells the sender the receiver's ceiling. No sentence directs a speaker. |
| `2` | Specification of Requirements | 0 | walked | Specification of Requirements. The BCP 14 key-words paragraph, which binds no speaker and states how to read the other sections. The derivation excludes it from the site inventory for that reason. |
| `3` | PATHS-LIMIT Capability | 4 | walked | PATHS-LIMIT Capability. The document's only normative section: Capability Code 76, the 5-octet AFI/SAFI/Paths-Limit tuple, and four capitalised MUST-level sites mapped below to DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1 through -3-4. Its three remaining directives are advisory and carry no site: ignore a tuple whose Paths Limit is zero, send no more than the advertised maximum, and offer a configuration knob. Those are the unsourced ids below. Two indicative sentences state no obligation: the Paths Limit field description, and "If the PATHS-LIMIT capability is empty (i.e. the Capability Length field is set to 0), it means that the sender doesn't have any specific limits to communicate", which defines what an empty capability means rather than directing a speaker. |
| `4` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records capability number 76 in the BGP Capability Codes registry. Binds IANA, not a speaker. |
| `5` | not stated | 0 | walked | Security Considerations, and with it the unnumbered Acknowledgements, References and Authors' Addresses blocks, which carry no section number and so stay in this body under the heading derivation. The security text states that PATHS-LIMIT mitigates some RFC 7911 concerns and that a rogue or misconfigured node can advertise a limit too low for the application. Its one direction, "Users of the PATHS-LIMIT Capability are encouraged to examine the behavior and potential impact", is an encouragement with no RFC 2119 keyword and binds no speaker. |
| `A` | Implementation Report | 0 | skipped (appendix-non-normative) | Implementation Report. An RFC 7942 note naming the FRRouting commit that implements the draft. It reports on another implementation and states no obligation. |

### Excluded sentences

The walk over DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT, so its obligations are stated where they were written.
