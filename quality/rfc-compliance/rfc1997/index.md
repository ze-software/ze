# RFC 1997 - BGP Communities Attribute

Standard communities. Every requirement this repository extracted from RFC 1997, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 5 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 5 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 5 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 5 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 26 | 26 of 26 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 5 | of 5 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 5 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 5 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 5 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

The 9 counts marked as a part above are the whole of the 5 gated MUSTs: they add up to 5. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | Standard communities |
| Enrolment | Enrolled |
| Requirements | 9 |
| Gated MUST-level | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 26 |
| Tagged units | 26 |
| Recorded audit verdicts | 5 |
| Discrimination records | 26 |
| Summary | `rfc/short/rfc1997.md` |
| Requirement shard | `rfc/requirements/rfc1997.md` |
| RFC text | `rfc/full/rfc1997.txt` |

## Enrolment

Enrolled: BGP Communities: five MUST-level requirements. RFC1997-Encoding-1 (attribute length a multiple of 4) is enforced on the wire-decode path by ParseCommunities (internal/core/bgp/attribute/community.go:201, wired via wire.go knownAttrParsers); the positive/negative pair proves a valid multiple-of-4 parses into length/4 communities while a non-multiple-of-4 is rejected as malformed (ErrInvalidLength). The three egress MUST-NOTs (Well-1 NO_EXPORT, Well-2 NO_ADVERTISE, Well-3 NO_EXPORT_SUBCONFED) and the Well-4 SHALL are enforced automatically, with no operator policy involved: wireu.ScanWellKnown reads the RECEIVED payload once per UPDATE and WellKnown.AllowsEgressTo (internal/component/bgp/wireu/wellknown.go) decides per destination, asked by both forward rails through Reactor.wellKnownAllowsEgress (internal/component/bgp/reactor/forward_wellknown.go). The word "received" in each clause is load-bearing and bounds the check: a route Ze ORIGINATES carrying NO_EXPORT is using the community for its purpose and is still advertised (test/interop/scenarios/as112-community-frr). The confederation boundary is the AS boundary because Ze configures no confederation, which is what the NO_EXPORT clause itself directs for "a stand-alone autonomous system that is not part of a confederation". Proof runs at three tiers: unit pairs over the decision and both rails, test/plugin/wellknown-no-export-egress.ci and wellknown-no-advertise-egress.ci over a running daemon's wire in the verify tier, and test/interop/scenarios/bgp-wellknown-noexport-frr with FRR (external, must not learn it) and BIRD (internal, must) as independent observers.

## What the public ledger says

**Area:** Standard communities

**Proof:** 5 of 5 MUST proven

**Evidence:** interop 1, functional 2, unit 2

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC1997-Well-1`](#rfc1997-well-1), [`RFC1997-Well-2`](#rfc1997-well-2), [`RFC1997-Well-3`](#rfc1997-well-3), [`RFC1997-Encoding-1`](#rfc1997-encoding-1), [`RFC1997-Well-4`](#rfc1997-well-4)

## Figures

- **MUST bar**: Ze proven 5
- **Overview bar**: Ze MUST proven 5, SHOULD not decided 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze proven | 5 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 4 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 4 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc1997.json](../../../data/rfc/rfc1997.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1997-Well-1` | NO_EXPORT (0xFFFFFF01) All routes received carrying a communities attribute containing this value MUST NOT be advertised outside a BGP confederation boundary (a stand-alone autonomous system that is not part of a confederation should be considered a confederation itself). (§Well-known Communities) | MUST NOT | Well | **positive:** `unit/verify` [`TestForwardNoExportSkipsExternalPeerOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L213). **positive:** `unit/verify` [`TestWellKnownNoExportRefusesExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L34). **negative:** `unit/verify` [`TestForwardNoExportStillWithdrawsFromExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L390). **negative:** `unit/verify` [`TestForwardWithoutNoExportReachesExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L239). **negative:** `unit/verify` [`TestWellKnownNoExportAllowsInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L48). **positive:** `functional/verify` [`wellknown-no-export-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-export-egress.ci#L1). **negative:** `functional/verify` [`wellknown-no-export-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-export-egress.ci#L8). **negative:** `functional/verify` [`wellknown-no-export-withdraw-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-export-withdraw-egress.ci#L1). **positive:** `interop/nightly` [`checkNoExportBoundary`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1183). **negative:** `interop/nightly` [`checkNoExportBoundary`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1184) |
| `RFC1997-Well-2` | NO_ADVERTISE (0xFFFFFF02) All routes received carrying a communities attribute containing this value MUST NOT be advertised to other BGP peers. (§Well-known Communities) | MUST NOT | Well | **positive:** `unit/verify` [`TestForwardNoAdvertiseSkipsEveryPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L258). **positive:** `unit/verify` [`TestWellKnownNoAdvertiseRefusesEveryPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L59). **negative:** `unit/verify` [`TestWellKnownNoAdvertiseAbsentAdvertisesToEveryPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L72). **positive:** `functional/verify` [`wellknown-no-advertise-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-advertise-egress.ci#L1). **negative:** `functional/verify` [`wellknown-no-advertise-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-advertise-egress.ci#L5) |
| `RFC1997-Well-3` | NO_EXPORT_SUBCONFED (0xFFFFFF03) All routes received carrying a communities attribute containing this value MUST NOT be advertised to external BGP peers (this includes peers in other members autonomous systems inside a BGP confederation). (§Well-known Communities) | MUST NOT | Well | **positive:** `unit/verify` [`TestForwardNoExportSubconfedSkipsExternalPeerOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L275). **positive:** `unit/verify` [`TestWellKnownNoExportSubconfedRefusesExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L83). **negative:** `unit/verify` [`TestWellKnownNoExportSubconfedAllowsInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L95). **positive:** `functional/verify` [`wellknown-no-advertise-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-advertise-egress.ci#L8). **negative:** `functional/verify` [`wellknown-no-advertise-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-advertise-egress.ci#L12) |
| `RFC1997-Encoding-1` | The attribute consists of a set of four octet values, each of which specify a community. (§COMMUNITIES attribute) | MUST | COMMUNITIES | **positive:** `unit/verify` [`TestCommunitiesParse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L44). **negative:** `unit/verify` [`TestCommunitiesParseRejectsNonMultipleOf4`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L54) |
| `RFC1997-Well-4` | The following communities have global significance and their operations shall be implemented in any community-attribute-aware BGP speaker. (§Well-known Communities) | SHALL | Well | **positive:** `unit/verify` [`TestForwardWellKnownNeedsNoOperatorPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L295). **positive:** `unit/verify` [`TestWellKnownAllThreeOperationsImplemented`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L104). **negative:** `unit/verify` [`TestForwardOtherReservedCommunitiesReachExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L314). **negative:** `unit/verify` [`TestWellKnownIgnoresOtherReservedCommunities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L122) |
| `RFC1997-Aggregation-1` | If a range of routes is to be aggregated and the resultant aggregates attribute section does not carry the ATOMIC_AGGREGATE attribute, then the resulting aggregate should have a COMMUNITIES path attribute which contains all communities from all of the aggregated routes. (§Aggregation) | SHOULD | Aggregation | **positive:** no positive test. **negative:** no negative test |
| `RFC1997-Operation-1` | A BGP speaker may use this attribute to control which routing information it accepts, prefers or distributes to other neighbors. (§Operation) | MAY | Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC1997-Operation-2` | A BGP speaker receiving a route that does not have the COMMUNITIES path attribute may append this attribute to the route when propagating it to its peers. (§Operation) | MAY | Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC1997-Operation-3` | A BGP speaker receiving a route with the COMMUNITIES path attribute may modify this attribute according to the local policy. (§Operation) | MAY | Operation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 1997 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1997-Well-1`](#rfc1997-well-1)

NO_EXPORT (0xFFFFFF01) All routes received carrying a communities attribute containing this value MUST NOT be advertised outside a BGP confederation boundary (a stand-alone autonomous system that is not part of a confederation should be considered a confederation itself). (§Well-known Communities)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc1997): "All routes received carrying a communities attribute containing this value MUST NOT be advertised outside a BGP confederation boundary (a stand-alone autonomous system that is not part of a confederation should be considered a confederation itself)." The only edit to test/plugin/wellknown-no-export-egress.ci, test/plugin/wellknown-no-export-withdraw-egress.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardNoExportStillWithdrawsFromExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L390) | unit/verify | revert, verified |
| negative | [`TestForwardWithoutNoExportReachesExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L239) | unit/verify | revert, verified |
| negative | [`TestWellKnownNoExportAllowsInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L48) | unit/verify | revert, verified |
| negative | [`checkNoExportBoundary`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1184) | interop/nightly | revert, verified |
| negative | [`wellknown-no-export-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-export-egress.ci#L8) | functional/verify | revert, verified |
| negative | [`wellknown-no-export-withdraw-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-export-withdraw-egress.ci#L1) | functional/verify | revert, verified |
| positive | [`TestForwardNoExportSkipsExternalPeerOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L213) | unit/verify | revert, verified |
| positive | [`TestWellKnownNoExportRefusesExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L34) | unit/verify | revert, verified |
| positive | [`checkNoExportBoundary`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1183) | interop/nightly | revert, verified |
| positive | [`wellknown-no-export-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-export-egress.ci#L1) | functional/verify | revert, verified |

### [`RFC1997-Well-2`](#rfc1997-well-2)

NO_ADVERTISE (0xFFFFFF02) All routes received carrying a communities attribute containing this value MUST NOT be advertised to other BGP peers. (§Well-known Communities)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc1997): "All routes received carrying a communities attribute containing this value MUST NOT be advertised to other BGP peers." The only edit to test/plugin/wellknown-no-advertise-egress.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWellKnownNoAdvertiseAbsentAdvertisesToEveryPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L72) | unit/verify | revert, verified |
| negative | [`wellknown-no-advertise-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-advertise-egress.ci#L5) | functional/verify | revert, verified |
| positive | [`TestForwardNoAdvertiseSkipsEveryPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L258) | unit/verify | revert, verified |
| positive | [`TestWellKnownNoAdvertiseRefusesEveryPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L59) | unit/verify | revert, verified |
| positive | [`wellknown-no-advertise-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-advertise-egress.ci#L1) | functional/verify | revert, verified |

### [`RFC1997-Well-3`](#rfc1997-well-3)

NO_EXPORT_SUBCONFED (0xFFFFFF03) All routes received carrying a communities attribute containing this value MUST NOT be advertised to external BGP peers (this includes peers in other members autonomous systems inside a BGP confederation). (§Well-known Communities)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc1997): "All routes received carrying a communities attribute containing this value MUST NOT be advertised to external BGP peers (this includes peers in other members autonomous systems inside a BGP confederation)." The only edit to test/plugin/wellknown-no-advertise-egress.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWellKnownNoExportSubconfedAllowsInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L95) | unit/verify | revert, verified |
| negative | [`wellknown-no-advertise-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-advertise-egress.ci#L12) | functional/verify | revert, verified |
| positive | [`TestForwardNoExportSubconfedSkipsExternalPeerOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L275) | unit/verify | revert, verified |
| positive | [`TestWellKnownNoExportSubconfedRefusesExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L83) | unit/verify | revert, verified |
| positive | [`wellknown-no-advertise-egress.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/wellknown-no-advertise-egress.ci#L8) | functional/verify | revert, verified |

### [`RFC1997-Encoding-1`](#rfc1997-encoding-1)

The attribute consists of a set of four octet values, each of which specify a community. (§COMMUNITIES attribute)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) a COMMUNITIES value that is not a whole set of four-octet values, e.g. 6 octets, silently truncated to one community; (b) TestCommunitiesParseRejectsNonMultipleOf4 require.ErrorIs ErrInvalidLength on 6 octets goes red. Positive TestCommunitiesParse asserts the exact two four-octet communities decoded from 8 octets.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCommunitiesParseRejectsNonMultipleOf4`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestCommunitiesParse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L44) | unit/verify | revert, verified |

### [`RFC1997-Well-4`](#rfc1997-well-4)

The following communities have global significance and their operations shall be implemented in any community-attribute-aware BGP speaker. (§Well-known Communities)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc1997): "The following communities have global significance and their operations shall be implemented in any community-attribute-aware BGP speaker." be95c10053 gave the wkPeer fixture explicit accepting import and export policies (FinalAccept, no refs) for an EBGP peer, and reworded TestForwardWellKnownNeedsNoOperatorPolicy's comment and precondition from 'no operator export policy' to 'export policy names NO operator filter'. Under RFC 8212 an EBGP destination with no policy refuses everything, so the old precondition can no longer hold on a delivering session; the precondition still pins that no filter, community-match rule or in-process egress filter exists, so the NO_EXPORT/NO_ADVERTISE suppression the test asserts still comes from the wire values alone, which is what 'shall be implemented' asks. Assertions unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardOtherReservedCommunitiesReachExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L314) | unit/verify | revert, verified |
| negative | [`TestWellKnownIgnoresOtherReservedCommunities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L122) | unit/verify | revert, verified |
| positive | [`TestForwardWellKnownNeedsNoOperatorPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc1997_forward_wellknown_test.go#L295) | unit/verify | revert, verified |
| positive | [`TestWellKnownAllThreeOperationsImplemented`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc1997_wellknown_test.go#L104) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc1997 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc1997.txt |
| Source fingerprint | ea2a9bee2501b9f7 |
| Record | rfc/extraction/rfc1997.json |
| Mapped sentences | 4 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | The whole document | 5 | walked | The whole document. RFC 1997 carries no numbered section headings, so the derivation returns one section spanning the title block to the References, and a skip of it would hide every gated id behind a skip kind. Walked heading by heading. Status of This Memo, the Abstract and the Introduction are indicative: BGP controls distribution by prefix or AS_PATH today, and this document proposes grouping destinations so a routing decision can also be made on the identity of a group. Terms and Definitions defines a community as a group of destinations sharing a common property and says each autonomous system administrator may define which communities a destination belongs to, which names the role site front:1 binds. Examples is two operational narratives (NSFNET AUP tagging, and filtering the more-specific components of an aggregate) and directs nobody. The COMMUNITIES attribute heading is the wire-format section, written entirely in the indicative: the attribute is optional transitive of variable length, it has Type Code 8, and 'The attribute consists of a set of four octet values, each of which specify a community.' That last sentence is where RFC1997-Encoding-1 is read from, and it is declared unsourced below because no capitalised or lowercase MUST-level keyword states it. The reserved ranges and the AS-in-the-first-two-octets convention follow under 'the following presumptions may be made', and are carried by the Reserved Ranges and Community Value Structure tables of rfc/short/rfc1997.md. Well-known Communities holds four of the five sites and every gated row this summary declares: sites front:2 to front:5, all mapped below. Operation is three lowercase-'may' sentences the site scan does not see, declared unsourced below as RFC1997-Operation-1 to -3. Aggregation is one lowercase-'should' sentence, declared unsourced below as RFC1997-Aggregation-1. Applicability states the attribute may be used with BGP version 2 and all subsequent versions, which is a compatibility fact rather than a directive. Security Considerations is one sentence saying security issues are not discussed in the memo, so it states no countermeasure. Acknowledgments, Authors' Addresses and the References to RFC 1771 and RFC 1965 bind no speaker. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the autonomous system administrator who assigns community values, a role the RFC names itself in Terms and Definitions: 'Each autonomous system administrator may define which communities a destination belongs to.' The sentence sits under 'however for administrative assignment, the following presumptions may be made', and it tells that administrator to draw a value from the range keyed by their own AS number in the first two octets, with the final two octets defined by that AS (the RFC's own example is AS 690 using 0x02B20000 through 0x02B2FFFF). It directs no encoding, decoding or propagation behavior a BGP speaker performs: a speaker carries the 32-bit value the operator configured and never derives it from an AS number, and ParseCommunities in internal/core/bgp/attribute reads each 4-octet value opaquely. The value layout it describes is carried by the Community Value Structure table of rfc/short/rfc1997.md. The lowercase 'shall' is why the site scan sees it only under the prose register. | The rest of the community attribute values shall be encoded using an autonomous system number in the first two octets. |

## Superseded

No document obsoletes RFC 1997, so its obligations are stated where they were written.
