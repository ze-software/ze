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
| No test at all | 0.0% | 0 of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 47.8% | 11 of 23 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 7 | of 4 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 4 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 23 |
| Tagged units | 23 |
| Recorded audit verdicts | 7 |
| Discrimination records | 11 |
| Summary | `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` |
| Requirement shard | `rfc/requirements/draft-abraitis-idr-addpath-paths-limit.md` |
| RFC text | `rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt` |

## Enrolment

Enrolled: BGP ADD-PATH Paths-Limit capability (code 76): four MUST-level requirements with positive and negative test carriers. For 3-1, coalescePathsLimit in internal/component/bgp/reactor/session_negotiate.go merges configuration and plugin declarations; TestBuildOpenCoalescesPathsLimit checks multiple families in one emitted instance and prevents repeated input instances from leaking into OPEN. For 3-2 and 3-3, core capability negotiation tests cover ADD-PATH present/absent and matching/unmatched families; for 3-4, TestParsePathsLimitDuplicateFirstWins covers retaining the first tuple and ignoring later duplicates. SHOULD-level zero handling (3-5) and session-wide sender enforcement (3-6) have separate parser, negotiation, and sender regression carriers.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

Receiver-advertised per-family path-count requests for ADD-PATH, with session-wide outbound enforcement across normal sends and forwarding, including the route-server fast path. The configuration knob states a request to the peer and does not bound what Ze accepts: a peer that ignores it is not policed.

**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1`](#draft-abraitis-idr-addpath-paths-limit-3-1), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2`](#draft-abraitis-idr-addpath-paths-limit-3-2), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3`](#draft-abraitis-idr-addpath-paths-limit-3-3), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4`](#draft-abraitis-idr-addpath-paths-limit-3-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1` | A BGP speaker wishing to indicate support for multiple AFI/SAFIs "MUST do so by including the information in a single instance of the PATHS-LIMIT capability" (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L383). **positive:** `unit/verify` [`TestParsePeerCapabilityPathsLimitSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L604). **negative:** `unit/verify` [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L384) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2` | "The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present" (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimit`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L589). **negative:** `unit/verify` [`TestNegotiatePathsLimitNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L644) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3` | "An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability" (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L670). **negative:** `unit/verify` [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L671) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4` | When more than one tuple is received for the same AFI/SAFI pair, only the first tuple is considered and "All others MUST be ignored" (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L878). **negative:** `unit/verify` [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L879) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-5` | "If the received Paths Limit is zero (0), the tuple SHOULD be ignored" (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L745). **positive:** `unit/verify` [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L846). **negative:** `unit/verify` [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L746). **negative:** `unit/verify` [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L847). **negative:** `unit/verify` [`TestParsePathsLimitZeroFirst`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L906) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6` | "A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability" (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_paths_limit_test.go#L181). **positive:** `unit/verify` [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_paths_limit_test.go#L138). **negative:** `unit/verify` [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_paths_limit_test.go#L182). **negative:** `unit/verify` [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_paths_limit_test.go#L139). **positive:** `functional/verify` [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L9). **negative:** `functional/verify` [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L10) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7` | "An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender" (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_paths_limit_test.go#L13). **positive:** `unit/verify` [`TestParsePeerCapabilityPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L564). **negative:** `unit/verify` [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_paths_limit_test.go#L14) |

## Gaps and untested MUSTs

DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1`](#draft-abraitis-idr-addpath-paths-limit-3-1)

A BGP speaker wishing to indicate support for multiple AFI/SAFIs "MUST do so by including the information in a single instance of the PATHS-LIMIT capability" (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged on 2026-09-11 against the draft text and both carriers, after the config-parser carrier's claim was reworded. Section 3: "A BGP speaker that wishes to indicate support for multiple AFI/SAFIs MUST do so by including the information in a single instance of the PATHS-LIMIT capability." TestBuildOpenCoalescesPathsLimit is what proves it. It drives buildOpen, the one producer of ze's OPEN, with four PATHS-LIMIT inputs: an empty configured instance, a configured ipv4/unicast tuple, and two raw code-76 plugin payloads carrying a duplicate and a zero tuple. It re-parses the encoded OPEN and makes two distinct assertions, not one wearing two hats. The negative is the draft's own violation shape offered as input, several instances in and require.Len(limits, 1) out. The positive is that the information for both families travels inside that one instance, pinned as exactly [ipv4/unicast 4, ipv4/multicast 3]. buildOpen runs twice in the loop, so a second OPEN over the same caller-owned capabilities cannot leak a second instance either. The read-back drops the plugin's ipv6/unicast limit-0 tuple under 3-5, which is why the expectation names two families and not three; the instance-count assertion is untouched by that skip, so no neighbouring rule decides this one. coalescePathsLimit (internal/component/bgp/reactor/session_negotiate.go) is the enforcing function and buildOpen calls it on every OPEN. Both polarities carry a discrimination record over it: "merged == nil" -> true emits four instances and reds the negative, "seen[f]" -> true empties the merged instance and reds the positive. TestParsePeerCapabilityPathsLimitSingleInstance is a subordinate positive one hop short of the wire, and its reworded claim now matches its body: it reads the capability list parsePeerFromTree BUILDS, asserts one *capability.PathsLimit holding both configured families, and never encodes or parses an OPEN. It adds no proof of this requirement, because the regression it prevents cannot reach a peer: a parser emitting one instance per family is merged back into one by coalescePathsLimit before any OPEN is encoded, and the other readers of ps.Capabilities (configuredCapabilities, read in reactor.go and reactor_api.go) take Multiprotocol families and ConfigProvider values rather than PATHS-LIMIT framing. It stays tagged as a true config-path regression guard whose claim no longer over-states it. Whether a redundant carrier is worth keeping is an owner decision, not an audit finding.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L384) | unit/verify | mutant, verified |
| positive | [`TestParsePeerCapabilityPathsLimitSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L604) | unit/verify | unproven |
| positive | [`TestBuildOpenCoalescesPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L383) | unit/verify | mutant, verified |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2`](#draft-abraitis-idr-addpath-paths-limit-3-2)

"The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present" (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestNegotiatePathsLimit pins both stored directions on exact values, the remote's 10 constraining our send and the local's 5 constraining the peer's send, so a direction swap fails it. TestNegotiatePathsLimitNoAddPath offers the same two PATHS-LIMIT capabilities with no ADD-PATH on either side and requires both maps to stay at zero. The producing branch is "n.addPath[f]&direction == 0" in negotiatePathsLimitDirection (internal/core/bgp/capability/negotiated.go). The negative buffer holds Multiprotocol and PathsLimit only, so no neighbouring rule can trip first. No discrimination record: both tags predate HEAD and the gate grandfathers them.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiatePathsLimitNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L644) | unit/verify | unproven |
| positive | [`TestNegotiatePathsLimit`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L589) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3`](#draft-abraitis-idr-addpath-paths-limit-3-3)

"An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability" (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. One test carries both polarities on two distinct assertions rather than one assertion wearing two hats: ipv4/unicast is present in ADD-PATH so its tuple applies in both directions (10 and 5), and ipv6/unicast is absent from ADD-PATH so both directions stay zero while its PATHS-LIMIT tuple is present in the same capability. Same producing branch as 3-2, evaluated per family. No discrimination record: the tags predate HEAD.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L671) | unit/verify | unproven |
| positive | [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L670) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4`](#draft-abraitis-idr-addpath-paths-limit-3-4)

When more than one tuple is received for the same AFI/SAFI pair, only the first tuple is considered and "All others MUST be ignored" (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestParsePathsLimitDuplicateFirstWins parses two ipv4/unicast tuples, limit 10 then limit 20, and requires one entry whose limit is 10. The positive is the kept value and the negative is the count, which are separate assertions over the same buffer. Removing the "seen[f]" guard in parsePathsLimit (internal/core/bgp/capability/capability.go) yields two entries, so the count assertion discriminates. No discrimination record: the tags predate HEAD.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L879) | unit/verify | unproven |
| positive | [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L878) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-5`](#draft-abraitis-idr-addpath-paths-limit-3-5)

"If the received Paths Limit is zero (0), the tuple SHOULD be ignored" (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. parsePathsLimit marks a family seen BEFORE it skips a zero limit, so a first zero tuple also suppresses a later duplicate for that family. TestParsePathsLimitSkipZero covers the plain skip and TestParsePathsLimitZeroFirst covers that interplay with 3-4, requiring ipv4/unicast to stay absent even though a later duplicate advertises 10. TestNegotiatePathsLimitDuplicateEntries runs one set of tuples through the direct API and through the wire parser and requires the negotiated maps to hold ipv6 alone in both directions. All three units carry a discrimination record over "limit == 0" -> false in parsePathsLimit, under which each goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L847) | unit/verify | mutant, verified |
| negative | [`TestParsePathsLimitZeroFirst`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L906) | unit/verify | mutant, verified |
| negative | [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L746) | unit/verify | unproven |
| positive | [`TestParsePathsLimitSkipZero`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L846) | unit/verify | mutant, verified |
| positive | [`TestNegotiatePathsLimitDuplicateEntries`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L745) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6`](#draft-abraitis-idr-addpath-paths-limit-3-6)

"A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability" (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The unit tests assert the exact identifier sequence that reached the wire, for ipv4/unicast and ipv6/unicast: announced [0 1 1 4] and withdrawn [99 0] across eight sends, which pins the withheld path, the replacement at capacity, the unknown withdrawal that frees nothing, and the reuse after a real withdrawal. TestPathsLimitForwardWritersShareState proves raw forwarding and re-encoded forwarding draw on one per-session budget, forwarding [5 7] and never 6. test/plugin/paths-limit-live.ci carries the same requirement at the daemon: it starts ze and a peer that advertises ipv4 1 and ipv6 2, drives the real `ze send bgp * update text` CLI over SSH, and asserts by hex, in sequence groups, that an excess identifier never reaches the wire before the next different-prefix sentinel and appears only after a withdrawal and an explicit retry. Six records: the four unit tags over the admission guard in pathsLimitSection, and the two .ci tags by revert of that same function, each citing the expect directive its red was tied to. The enforcing code is Session.pathsLimitSection, reached from writeRawUpdateBody through filterPathsLimit, which is why the route-server fast path is covered by the same state.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_paths_limit_test.go#L182) | unit/verify | mutant, verified |
| negative | [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_paths_limit_test.go#L139) | unit/verify | mutant, verified |
| negative | [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L10) | functional/verify | revert, verified |
| positive | [`TestPathsLimitForwardWritersShareState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_paths_limit_test.go#L181) | unit/verify | mutant, verified |
| positive | [`TestPathsLimitSessionAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_paths_limit_test.go#L138) | unit/verify | mutant, verified |
| positive | [`paths-limit-live.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/paths-limit-live.ci#L9) | functional/verify | revert, verified |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7`](#draft-abraitis-idr-addpath-paths-limit-3-7)

"An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender" (§3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The pair over TestConfiguredPathsLimitOpen is a strong test of half the sentence and nothing tests the other half. What it checks: the container knob `session > capability > add-path > limit`, the per-family override, and the interaction with disabled, refused and required families decide the tuples encoded in the OPEN, with an omitted limit advertising none and a disabled or refused family unable to carry an orphan tuple. parseAddPathFromTree (internal/component/bgp/reactor/config_capabilities.go) derives the tuples from addPath.Families, the same slice that reaches the wire. What nothing checks, because no code does it: the draft asks for a knob that specifies "the maximum number of paths to accept from a sender", and ze's knob states a number to the peer without bounding what ze accepts. EncodingCaps.PathsLimitRecv is read by internal/core/bgp/context/context.go, internal/component/bgp/format/json.go and internal/component/bgp/reactor/negotiated.go, all of which carry it into a negotiated context or into show output, and no reader drops a received path above it. A peer that ignores the request is not policed. The verdict is weak rather than enforced because the tag ties this body to the whole requirement, and the reading under which "accept" means local enforcement is not proven by anything. The absent behavior is disclosed on the summary's Support remaining row. No discrimination record was written, because the owner's open decision on building the receive side can reword this claim, and rewording stales a record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_paths_limit_test.go#L14) | unit/verify | unproven |
| positive | [`TestConfiguredPathsLimitOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_paths_limit_test.go#L13) | unit/verify | unproven |
| positive | [`TestParsePeerCapabilityPathsLimit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L564) | unit/verify | unproven |

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
