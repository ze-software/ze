# RFC 7911 - Advertisement of Multiple Paths in BGP

ADD-PATH. Every requirement listed for RFC 7911, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9 | of 9 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 9 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 9 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 9 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 9 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 42 | 42 of 61 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 9 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 9 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 9 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 9 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 9 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 9 | of 13 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 9 mandatory requirements: they add up to 9. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | ADD-PATH |
| Automatic checks | Included |
| Requirements | 13 |
| Mandatory requirements (MUST level) | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 61 |
| Tests linked to requirements | 61 |
| Recorded audit verdicts | 9 |
| Fault test records | 42 |
| Summary | `rfc/short/rfc7911.md` |
| Requirement table | `rfc/requirements/rfc7911.md` |
| RFC text | `rfc/full/rfc7911.txt` |

## Automatic checks

Included in automatic checks: Advertisement of Multiple Paths in BGP (ADD-PATH): nine MUST-level requirements. Eight are met: 3-1 (a 4-octet Path Identifier is prepended to NLRI when ADD-PATH is negotiated), 4-1 (a single ADD-PATH capability instance lists all AFI/SAFIs), 5-1 and 5-2 (a path is sent only if the local speaker advertised Send/Both and the remote advertised Receive/Both), 5-3 (Path IDs are not added when ADD-PATH is not negotiated for the family), 5-4 (the RIB is keyed by prefix and Path ID and the egress encodes the Path ID), and 5-5 (a negotiated family's received NLRI is parsed with its 4-octet Path ID) carry positive+negative tags. 2-1 (the same prefix with different Path IDs is treated as different paths) is {single-polarity: positive}: the per-peer RIB key is (prefix, Path ID) by construction. 2-2 (a speaker re-advertising a path generates its own Path Identifier) carries positive+negative tags since 2026-08-14: ze mints its own value per ingress path in `internal/component/bgp/reactor/forward_path_id.go` and both forward rails read it, so no requirement of this RFC is a gap.

## Published support status

**Area:** ADD-PATH

**Test verification:** 9 of 9 MUST proven

**Evidence:** interop 1, functional 1, unit 7

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **9** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (9):** [`RFC7911-2-1`](#rfc7911-2-1), [`RFC7911-2-2`](#rfc7911-2-2), [`RFC7911-3-1`](#rfc7911-3-1), [`RFC7911-4-1`](#rfc7911-4-1), [`RFC7911-5-1`](#rfc7911-5-1), [`RFC7911-5-2`](#rfc7911-5-2), [`RFC7911-5-3`](#rfc7911-5-3), [`RFC7911-5-4`](#rfc7911-5-4), [`RFC7911-5-5`](#rfc7911-5-5)

## Figures

- **MUST bar**: Ze verified 9
- **Overview bar**: Ze MUST verified 9, Not assessed 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze verified | 9 |
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
| SHOULD / MAY rows | 4 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7911.json](../../../data/rfc/rfc7911.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7911-2-1` | However, the Path Identifier MUST be assigned in such a way that the BGP speaker is able to use the (Prefix, Path Identifier) to uniquely identify a path advertised to a neighbor. (Section 2) | MUST | 2 - How to Identify a Path | **positive:** `unit/verify` [`TestForwardPathIDStableAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L96). **negative:** `unit/verify` [`TestForwardPathIDsDifferForCollidingSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L45) |
| `RFC7911-2-2` | A BGP speaker that re-advertises a route MUST generate its own Path Identifier to be associated with the re-advertised route. (Section 2) | MUST | 2 - How to Identify a Path | **positive:** `unit/verify` [`TestForwardPathIDBoundaryReceivedValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L166). **positive:** `unit/verify` [`TestForwardPathIDDiffersForTwoSourcePeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L48). **positive:** `unit/verify` [`TestForwardPathIDFiniteNamespace`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L244). **positive:** `unit/verify` [`TestForwardPathIDKeepsTheSourceOfARebuiltFrame`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L287). **positive:** `unit/verify` [`TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L329). **positive:** `unit/verify` [`TestForwardPathIDMPGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_mp_test.go#L25). **positive:** `unit/verify` [`TestForwardPathIDMatchesAnnounceAndWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L75). **positive:** `unit/verify` [`TestForwardPathIDSeparatesNonAddPathSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L137). **positive:** `unit/verify` [`TestForwardPathIDStableForEachDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L190). **positive:** `unit/verify` [`TestForwardPathIDSurvivesAttributeChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L108). **positive:** `unit/verify` [`TestForwardPathIDWithdrawCarriesTheAnnouncedValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L232). **positive:** `unit/verify` [`TestForwardPathIDsDifferForCollidingSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L33). **positive:** `unit/verify` [`TestForwardPathIDsFreedOnRelayedWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L193). **positive:** `unit/verify` [`TestPathIDKeyFollowsWhatTheSourceFramed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_zz_pathid_growth_probe_test.go#L22). **negative:** `unit/verify` [`TestForwardPathIDFiniteNamespace`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L246). **negative:** `unit/verify` [`TestForwardPathIDMPGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_mp_test.go#L26). **negative:** `unit/verify` [`TestForwardPathIDStableAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L90). **positive:** `functional/verify` [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L28). **positive:** `interop/nightly` [`checkAddPathReadvertiseCollision`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L17) |
| `RFC7911-3-1` | In order to carry the Path Identifier in an UPDATE message, the NLRI encoding MUST be extended by prepending the Path Identifier field, which is of four octets. (Section 3) | MUST | 3 - Extended NLRI Encodings | **positive:** `unit/verify` [`TestINETWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_inet_test.go#L112). **positive:** `unit/verify` [`TestPathInformationZeroIsAnIdentifierOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc7911_addpath_pathid_test.go#L42). **positive:** `unit/verify` [`TestWithdrawOnlyUpdateKeepsPathIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc7911_withdraw_only_test.go#L66). **positive:** `unit/verify` [`TestWriteNLRI_WithStoredPathID`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_base_len_test.go#L182). **negative:** `unit/verify` [`TestPathInformationIsNotWrittenWithoutTheNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc7911_addpath_pathid_test.go#L84). **negative:** `unit/verify` [`TestWriteNLRI_AddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_base_len_test.go#L143). **positive:** `functional/verify` [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L21). **negative:** `functional/verify` [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L24) |
| `RFC7911-4-1` | A BGP speaker that wishes to indicate support for multiple AFI/SAFIs MUST do so by including the information in a single instance of the ADD-PATH Capability. (Section 4) | MUST | 4 - ADD-PATH Capability | **positive:** `unit/verify` [`TestAddPathMultipleFamilies`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L403). **positive:** `unit/verify` [`TestRFC7911OpenCarriesOneAddPathInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L174). **negative:** `unit/verify` [`TestAddPathCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L374). **negative:** `unit/verify` [`TestRFC7911PerFamilyAddPathStaysOneInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L194) |
| `RFC7911-5-1` | For a BGP speaker to be able to send multiple paths to its peer, that BGP speaker MUST advertise the ADD-PATH Capability with the Send/ Receive field set to either 2 or 3, and MUST receive from its peer the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the corresponding <AFI, SAFI>. (Section 5) | MUST | 5 - Operation | **positive:** `unit/verify` [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L167). **positive:** `unit/verify` [`TestNegotiateAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L81). **positive:** `unit/verify` [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L12). **negative:** `unit/verify` [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L168). **negative:** `unit/verify` [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L13) |
| `RFC7911-5-2` | For a BGP speaker to be able to send multiple paths to its peer, that BGP speaker MUST advertise the ADD-PATH Capability with the Send/ Receive field set to either 2 or 3, and MUST receive from its peer the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the corresponding <AFI, SAFI>. (Section 5) | MUST | 5 - Operation | **positive:** `unit/verify` [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L169). **positive:** `unit/verify` [`TestNegotiateAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L82). **positive:** `unit/verify` [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L14). **negative:** `unit/verify` [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L170). **negative:** `unit/verify` [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L15) |
| `RFC7911-5-3` | A BGP speaker MUST follow the procedures defined in [RFC4271] when generating an UPDATE message for a particular <AFI, SAFI> to a peer unless the BGP speaker advertises the ADD-PATH Capability to the peer indicating its ability to send multiple paths for the <AFI, SAFI>, and also receives the ADD-PATH Capability from the peer indicating its ability to receive multiple paths for the <AFI, SAFI> (Section 5) | MUST | 5 - Operation | **positive:** `unit/verify` [`TestForwardSplitSameContextKeepsRawSplit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_body_test.go#L128). **positive:** `unit/verify` [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L21). **negative:** `unit/verify` [`TestForwardPathIDLeavesNonAddPathDestinationAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L221). **negative:** `unit/verify` [`TestForwardSplitConvertsAddPathContext`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_body_test.go#L24). **negative:** `unit/verify` [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L22) |
| `RFC7911-5-4` | A BGP speaker MUST follow the procedures defined in [RFC4271] when generating an UPDATE message for a particular <AFI, SAFI> to a peer unless the BGP speaker advertises the ADD-PATH Capability to the peer indicating its ability to send multiple paths for the <AFI, SAFI>, and also receives the ADD-PATH Capability from the peer indicating its ability to receive multiple paths for the <AFI, SAFI>, in which case the speaker MUST generate a route update for the <AFI, SAFI> based on the combination of the address prefix and the Path Identifier, and use the extended NLRI encodings specified in this document. (Section 5) | MUST | 5 - Operation | **positive:** `unit/verify` [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L23). **positive:** `unit/verify` [`TestSplitUpdateAddPathEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_split_test.go#L304). **positive:** `unit/verify` [`TestWriteAnnounceUpdateWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L517). **negative:** `unit/verify` [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L24). **negative:** `unit/verify` [`TestSplitUpdateEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_split_test.go#L254). **negative:** `unit/verify` [`TestWriteAnnounceUpdateWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L518) |
| `RFC7911-5-5` | The peer SHALL act accordingly in processing an UPDATE message related to a particular <AFI, SAFI>. (Section 5) | SHALL | 5 - Operation | **positive:** `unit/verify` [`TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L121). **positive:** `unit/verify` [`TestEnforceRFC7606_MPAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L73). **positive:** `unit/verify` [`TestRFC7606Section54ReadsTypedNLRIUnderAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go#L88). **positive:** `unit/verify` [`TestRFC7911AddPathStateAppliesPerFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_reactor_b_test.go#L78). **negative:** `unit/verify` [`TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L122). **negative:** `unit/verify` [`TestEnforceRFC7606_MPAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L74). **negative:** `unit/verify` [`TestRFC7911AddPathStateAppliesPerFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_reactor_b_test.go#L79) |
| `RFC7911-4-2` | If any other value is received, then the capability SHOULD be treated as not understood and ignored [RFC5492]. (Section 4) | SHOULD | 4 - ADD-PATH Capability | **positive:** no positive test. **negative:** no negative test |
| `RFC7911-5-6` | If a BGP speaker receives a message to withdraw a prefix with a Path Identifier not seen before, it SHOULD silently ignore it. (Section 5) | SHOULD | 5 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7911-5-7` | A BGP speaker SHOULD include the best route [RFC4271] when more than one path is advertised to a neighbor, unless it is a path received from that neighbor. (Section 5) | SHOULD | 5 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7911-5-8` | As the Path Identifiers are locally assigned, and may or may not be persistent across a control plane restart of a BGP speaker, an implementation SHOULD take special care so that the underlying forwarding plane of a "Receiving Speaker" as described in [RFC4724] is not affected during the graceful restart of a BGP session. (Section 5) | SHOULD | 5 - Operation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7911 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7911-2-1`](#rfc7911-2-1)

However, the Path Identifier MUST be assigned in such a way that the BGP speaker is able to use the (Prefix, Path Identifier) to uniquely identify a path advertised to a neighbor. (Section 2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/adj_rib_out.go::clearRoutes, internal/component/bgp/reactor/forward_path_id.go::generate, internal/component/bgp/reactor/forward_path_id.go::generatePath, internal/component/bgp/reactor/forward_path_id.go::lookupPath, internal/component/bgp/reactor/forward_path_id.go::releasePath, internal/component/bgp/reactor/forward_path_id.go::releaseSource, internal/component/bgp/reactor/session_ownership.go::allow, internal/component/bgp/reactor/session_ownership.go::beginAdjOut, internal/component/bgp/reactor/session_ownership.go::recordDeletion, internal/component/bgp/reactor/session_ownership.go::release, internal/component/bgp/reactor/session_ownership_walk.go::recordNLRI, internal/component/bgp/reactor/session_rfc7607.go::Session.transmittedASZeroAttribute, internal/component/bgp/reactor/session_rfc7607.go::Session.writeASZeroAsWithdrawal, internal/component/bgp/reactor/session_write.go::writeUpdateBody moved. RFC 7911 Section 2: "However, the Path Identifier MUST be assigned in such a way that the BGP speaker is able to use the (Prefix, Path Identifier) to uniquely identify a path advertised to a neighbor." Both tagged units were re-read. TestForwardPathIDsDifferForCollidingSources forces two ingress sources to use the same native prefix and received identifier, reads their destination identifier fields and requires inequality. TestForwardPathIDStableAcrossUpdates requires a repeat of one path to keep its local identifier while a colliding source remains distinct. Source tracing covers source/native-path allocation, recipient collision binding and retention, final ownership filtering, recorded deletion and session retirement. The new writeUpdateBody AS-zero gate reads the destination's negotiated ASN width, which the repaired fixtures now publish coherently. Its withdrawal branch projects original ingress identities through projectFwdWithdrawals and bindWithdrawalChunk instead of interpreting outgoing identifiers as source identifiers; normal valid advertisements keep the existing ownership path. These tagged units exercise buildFwdBody rather than the final AS-zero writer or a failed socket write. The independently inspected main AS-zero provenance fault additionally fails both ADD-PATH rails when original identities are replaced by outgoing ones, but is not an RFC7911 discrimination record. Rejudgment retains the identifier-uniqueness verdict without claiming allocator reclamation policy, complete session integration or a new foreign-peer run.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestForwardPathIDsDifferForCollidingSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L45) | unit/verify | mutant, verified |
| positive | [`TestForwardPathIDStableAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L96) | unit/verify | mutant, verified |

### [`RFC7911-2-2`](#rfc7911-2-2)

A BGP speaker that re-advertises a route MUST generate its own Path Identifier to be associated with the re-advertised route. (Section 2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/adj_rib_out.go::clearRoutes, internal/component/bgp/reactor/forward_body.go::buildFwdBody, internal/component/bgp/reactor/forward_path_id.go::framed, internal/component/bgp/reactor/forward_path_id.go::fwdRegenerateRawPathIDs, internal/component/bgp/reactor/forward_path_id.go::fwdReleaseSection, internal/component/bgp/reactor/forward_path_id.go::generate, internal/component/bgp/reactor/forward_path_id.go::generatePath, internal/component/bgp/reactor/forward_path_id.go::lookupPath, internal/component/bgp/reactor/forward_path_id.go::releasePath, internal/component/bgp/reactor/forward_path_id.go::releaseSource, internal/component/bgp/reactor/forward_path_id.go::unframed, internal/component/bgp/reactor/session_ownership.go::allow, internal/component/bgp/reactor/session_ownership.go::beginAdjOut, internal/component/bgp/reactor/session_ownership.go::recordDeletion, internal/component/bgp/reactor/session_ownership_walk.go::recordNLRI, internal/component/bgp/reactor/session_rfc7607.go::Session.writeASZeroAsWithdrawal, internal/component/bgp/reactor/session_write.go::writeUpdateBody moved. Independent committed-source rejudgment; this auditor authored none of the change. Producer/source baseline: 89642e2a872e0b6d0b9f97b309889856112d2ba6, read with git show; no cited committed producer or tagged Go unit changed through 74da4cb51663e795c063d3150552c9b42ec982fa. Existing tagged units were compared against the reviewed working tree; unrelated sibling checker edits are not imported. The strengthened CI and collision helper were separately read and must land as the exact reviewed carrier/helper before source head stamping. RFC 7911 Section 2: "A BGP speaker that re-advertises a route MUST generate its own Path Identifier to be associated with the re-advertised route." The adjoining requirement makes recipient (Prefix, Path Identifier) unique; Section 5 makes a repeated pair replace its previous advertisement. Every tagged cover was read. Collision/stability and generation units assert distinct same-prefix ingress paths, stable replacement despite changed attributes, exact inverse withdrawals, unframed sources, received/local zero and maximum values, separate destinations and occupied wrap candidates. The MP unit checks native prefix/ID pairs on raw, cross-context and unframed-source rails with reversed replacement/withdrawal order; the growth unit preserves a sibling after another prefix retires. The four churn units assert recipient-visible identity through relay, cache acknowledgement and final writes, including rebuilt frames and simultaneous withdrawal/reannouncement. Memory reclamation remains a separate Ze contract. The foreign checker requires negotiated receive, both origins under distinct IDs at FRR and preserved origin/ID pairings after reconnect; its predicates were read, but no foreign run is claimed. The strengthened functional carrier sends the SAME10.0.0.0/24 and receivedID1 from A and D. The AP recipient gets A under local0 on live/replay, then D under local2 while A lingers without withdrawal. Exact0/2 are fixture facts; numerical inequality from a received identifier is not an RFC obligation. The plain recipient retains both original controls and receives D without an identifier. Original observer/API/EOR barriers precede D's release; separate post-match AP/plain done fences precede shutdown. The completed final main-thread native collision trace and matching positive mutant entry in rfc/discrimination/rfc7911/RFC7911-2-2.json were independently inspected. The clean canonical carrier passed with AP IDs0/0/2. The final-wire mask mutation leaves both original AP live/replay frames byte-identical with ID0, then emits the same55-byte AS65004 third frame with ID0 instead of2 for10.0.0.0/24. Thus a real recipient(prefix,ID) collision is observed before the post-match marker timeout; neither the timeout nor a producer halt stands in for the violated wire assertion. The diagnostic text decoder does not apply negotiated AP layout, so recipient/sequence and exact hex are authoritative. Only the final post-source-END observation is accepted; the earlier collision attempt overlapping private API edits is excluded. The separately observed AP-omission and plain-insertion records support their own framing tags, not this identity claim. All41 existing producer keys are preserved. Committed raw/conversion generation, ingress-key recipient binding/reservations, final ownership filtering/recording/retirement and original identity through AS-zero withdrawal projection sustain this judgment. Private selected-output branches/indexes and private source-receipt assignment/equality guards in adj_rib_out, session_ownership and session_ownership_walk are excluded. The committed session_write SourceReceipt carrier copies metadata after wire admission; it is not evidence for private guards. Historical proofs and prior provenance remain preserved; earlier judgments remain in this audit file's git history. Native issue-only validation of these two IDs and source head rejudge stamping remain main-thread work.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestForwardPathIDFiniteNamespace`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L246) | unit/verify | revert, verified |
| negative | [`TestForwardPathIDMPGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_mp_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestForwardPathIDStableAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDKeepsTheSourceOfARebuiltFrame`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L287) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L329) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDWithdrawCarriesTheAnnouncedValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L232) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDsFreedOnRelayedWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L193) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDBoundaryReceivedValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L166) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDDiffersForTwoSourcePeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDFiniteNamespace`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L244) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDMatchesAnnounceAndWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDSeparatesNonAddPathSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L137) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDStableForEachDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L190) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDSurvivesAttributeChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDMPGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_mp_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDsDifferForCollidingSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestPathIDKeyFollowsWhatTheSourceFramed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_zz_pathid_growth_probe_test.go#L22) | unit/verify | revert, verified |
| positive | [`checkAddPathReadvertiseCollision`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L17) | interop/nightly | mutant, verified |
| positive | [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L28) | functional/verify | mutant, verified |

### [`RFC7911-3-1`](#rfc7911-3-1)

In order to carry the Path Identifier in an UPDATE message, the NLRI encoding MUST be extended by prepending the Path Identifier field, which is of four octets. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/format/rfc7911_withdraw_only_test.go::TestWithdrawOnlyUpdateKeepsPathIdentifier moved. Independent committed-source rejudgment; this auditor authored none of the change. Producer/source baseline: 89642e2a872e0b6d0b9f97b309889856112d2ba6, read with git show; no cited committed producer or tagged Go unit changed through 74da4cb51663e795c063d3150552c9b42ec982fa. Existing tagged units were compared against the reviewed working tree; unrelated sibling checker edits are not imported. The strengthened CI and collision helper were separately read and must land as the exact reviewed carrier/helper before source head stamping. RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE message, the NLRI encoding MUST be extended by prepending the Path Identifier field, which is of four octets." Sections 2 and 5 were also read. Every tagged cover was read: the stored-ID encoder pins the leading network-order field and exact length; the INET decoder pins consumption, ID and native prefix; the command pair preserves the labeled payload with ID zero and omits the field without negotiation; the withdraw-only formatter retains ID7 in JSON and the exact native withdrawal in text. The functional carrier retains the original exact AP/plain live/replay frames and adds the second source's correctly framed frames. No RIB plugin can supply the replay control; only A feeds the receive store. The original observer checks live sends, explicitly requests both replays and retains API/EOR barriers. The helper completes that scenario before releasing D and waits for separate post-match AP/plain done files. A lingers; timeouts and exit/panic/fatal/observer-failure checks remain. Explicit unchanged NEXT_HOP preserves this fixture's bytes; export policy permits its API marker and excludes returned D routes. The two polarities are separate recipient wire assertions. Native discrimination observations for canonical test/plugin/adj-rib-in-replay-addpath-source.ci were independently checked in the completed main-thread traces and matching positive/negative mutant entries in rfc/discrimination/rfc7911/RFC7911-3-1.json. Each clean carrier passed. The positive mutation removes only four final-wire identifier octets for AP: the clean AP frames are 55/55/55 bytes with IDs0/0/2, while the broken first AP frame is 51 bytes with identical attributes and bare suffix180A0000, failing the exact55-byte AP assertion. The negative mutation inserts four zero octets only for the plain recipient and corrects the outer length: its actual55-byte UPDATE retains the attributes but carries four spurious /0 prefixes before10/24, against the expected bare10/24. The resulting replay/observer failures follow these received-wire violations; they are not the attribution evidence. The diagnostic text decoder does not apply the negotiated AP layout, so exact bytes and recipient/sequence identify the violation. These are observed semantic mutants, not producer-panic substitutes or merely missing frames. The earlier overlapping collision attempt is excluded. No private selected-path or receipt-guard additions are counted as committed behavior, and no foreign run is claimed. Existing covered units and prior provenance are preserved; earlier judgments remain in this audit file's git history. Native issue-only validation of these two IDs and source head rejudge stamping remain main-thread work.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPathInformationIsNotWrittenWithoutTheNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc7911_addpath_pathid_test.go#L84) | unit/verify | not verified |
| negative | [`TestWriteNLRI_AddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_base_len_test.go#L143) | unit/verify | mutant, verified |
| negative | [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L24) | functional/verify | mutant, verified |
| positive | [`TestWithdrawOnlyUpdateKeepsPathIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc7911_withdraw_only_test.go#L66) | unit/verify | mutant, verified |
| positive | [`TestPathInformationZeroIsAnIdentifierOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc7911_addpath_pathid_test.go#L42) | unit/verify | not verified |
| positive | [`TestWriteNLRI_WithStoredPathID`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_base_len_test.go#L182) | unit/verify | mutant, verified |
| positive | [`TestINETWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_inet_test.go#L112) | unit/verify | not verified |
| positive | [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L21) | functional/verify | mutant, verified |

### [`RFC7911-4-1`](#rfc7911-4-1)

A BGP speaker that wishes to indicate support for multiple AFI/SAFIs MUST do so by including the information in a single instance of the ADD-PATH Capability. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Now reads Ze's built OPEN. Positive: default send/receive over two families -> exactly one code-69 instance listing both, mode 3. Negative: per-family overrides only over three families with directions 2/1/3 -> still one instance holding all three. Judge break: config_capabilities.go appending one AddPath capability per family turned both red. Old parse-level TestAddPathCapability/TestAddPathMultipleFamilies tags are supplementary.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7911PerFamilyAddPathStaysOneInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L194) | unit/verify | revert, verified |
| negative | [`TestAddPathCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L374) | unit/verify | not verified |
| positive | [`TestRFC7911OpenCarriesOneAddPathInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L174) | unit/verify | revert, verified |
| positive | [`TestAddPathMultipleFamilies`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L403) | unit/verify | not verified |

### [`RFC7911-5-1`](#rfc7911-5-1)

For a BGP speaker to be able to send multiple paths to its peer, that BGP speaker MUST advertise the ADD-PATH Capability with the Send/ Receive field set to either 2 or 3, and MUST receive from its peer the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the corresponding <AFI, SAFI>. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7911 Section 5: "For a BGP speaker to be able to send multiple paths to its peer, that BGP speaker MUST advertise the ADD-PATH Capability with the Send/Receive field set to either 2 or 3, and MUST receive from its peer the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the corresponding <AFI, SAFI>." All three current carriers read: capability/negotiated_test.go TestNegotiateAddPath, context/rfc7911_negotiated_test.go TestFromNegotiatedAddPath (all nine direction-mode pairs), capability/rfc7911_family_scope_test.go TestRFC7911AddPathNegotiationIsPerFamily. The new matrix crosses IPv4/IPv6 and unicast/multicast, both send-capable local modes and both receive-capable peer modes, while MP negotiation offers all families; exactly the intersection family can send. Thus local Receive-only, remote Send-only, wrong AFI and wrong SAFI independently cannot grant send. Negotiate keys both mode maps by the full Family and FromNegotiatedSend/Recv preserve the directional result. This closes the prior family-blindness gap rather than merely re-sealing it. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step. Post-lint independent source rejudgment: TestRFC7911AddPathNegotiationIsPerFamily now computes matchingFamilies = (localFamily == remoteFamily), then wantSend = matchingFamilies && (fam == localFamily). By equality transitivity this is exactly (fam == localFamily && fam == remoteFamily): false for crossed AFI or SAFI; true only for the matching queried family. All three session families are offered by MP on both sides, so lack of MP negotiation cannot mask leakage. All four local-send/remote-receive combinations and all nine family pairs still run. Re-read both companion carriers, including the nine direction-mode context matrix, and Negotiate maps indexed by the full Family. Positive and negative tags test distinct matrix inputs. Enforced remains; all four existing records on this changed unit require native renewal, which is not observed here.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L13) | unit/verify | revert, verified |
| negative | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L168) | unit/verify | not verified |
| positive | [`TestNegotiateAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L81) | unit/verify | not verified |
| positive | [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L12) | unit/verify | revert, verified |
| positive | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L167) | unit/verify | not verified |

### [`RFC7911-5-2`](#rfc7911-5-2)

For a BGP speaker to be able to send multiple paths to its peer, that BGP speaker MUST advertise the ADD-PATH Capability with the Send/ Receive field set to either 2 or 3, and MUST receive from its peer the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the corresponding <AFI, SAFI>. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7911 Section 5: "For a BGP speaker to be able to send multiple paths to its peer, that BGP speaker MUST advertise the ADD-PATH Capability with the Send/Receive field set to either 2 or 3, and MUST receive from its peer the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the corresponding <AFI, SAFI>." All three current carriers read: capability/negotiated_test.go TestNegotiateAddPath, context/rfc7911_negotiated_test.go TestFromNegotiatedAddPath (all nine direction-mode pairs), capability/rfc7911_family_scope_test.go TestRFC7911AddPathNegotiationIsPerFamily. The new matrix crosses IPv4/IPv6 and unicast/multicast, both send-capable local modes and both receive-capable peer modes, while MP negotiation offers all families; exactly the intersection family can send. Thus local Receive-only, remote Send-only, wrong AFI and wrong SAFI independently cannot grant send. Negotiate keys both mode maps by the full Family and FromNegotiatedSend/Recv preserve the directional result. This closes the prior family-blindness gap rather than merely re-sealing it. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step. Post-lint independent source rejudgment: the family-equality hoist preserves the truth table, not merely the assertion count. A queried family is send-enabled exactly when localFamily == remoteFamily == fam; crossed AFI, crossed SAFI, or a different queried family are false. Both remote Receive and Both and local Send and Both are crossed while MP offers all families. The companion TestNegotiateAddPath pins Both/Receive to exact Send, and TestFromNegotiatedAddPath tests all nine modes including peer Send-only disabling transmission. Negotiate reads localModes[f] and remoteModes[f] for each negotiated family. Enforced remains, with native renewal owed for both polarities of each row on the changed family-scope unit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L15) | unit/verify | revert, verified |
| negative | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L170) | unit/verify | not verified |
| positive | [`TestNegotiateAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L82) | unit/verify | not verified |
| positive | [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L14) | unit/verify | revert, verified |
| positive | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L169) | unit/verify | not verified |

### [`RFC7911-5-3`](#rfc7911-5-3)

A BGP speaker MUST follow the procedures defined in [RFC4271] when generating an UPDATE message for a particular <AFI, SAFI> to a peer unless the BGP speaker advertises the ADD-PATH Capability to the peer indicating its ability to send multiple paths for the <AFI, SAFI>, and also receives the ADD-PATH Capability from the peer indicating its ability to receive multiple paths for the <AFI, SAFI> (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/rfc7911_forward_body_test.go::TestForwardSplitConvertsAddPathContext, internal/component/bgp/reactor/rfc7911_forward_body_test.go::TestForwardSplitSameContextKeepsRawSplit, internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go::TestForwardPathIDLeavesNonAddPathDestinationAlone moved. RFC 7911 Section 5 makes the extended encoding conditional on local send and remote receive capability for the corresponding AFI/SAFI; without that condition the ordinary RFC4271 encoding applies. The complete scoped subject is this per-family generation/framing choice, not all requirements of the separately implemented base BGP specification. All five tagged covers were read. Both family-generation polarities negotiate local Send and remote Receive in turn for IPv4 unicast, IPv6 unicast and IPv4 multicast, call AnnounceNLRIBatch, and compare exact concatenated NLRI bytes: only the selected family carries identifiers 7 and 9; crossed AFI/SAFI cases remain plain. The direct body edit only publishes that existing Negotiated value to Session in addition to Peer. AnnounceNLRIBatch reaches announceFactsFor, buildBatchAnnounceUpdate and writeBatchNLRI with the destination family's addPath fact. TestForwardSplitConvertsAddPathContext strips identifiers and compares all 80 plain prefixes after destination-context splitting. TestForwardSplitSameContextKeepsRawSplit checks complete extended parsing, exact prefix order and separate identities for colliding sources after raw splitting. TestForwardPathIDLeavesNonAddPathDestinationAlone is principally the unchanged-buffer/no-borrow control; exact framing is supplied by the other covers. The repaired recording fixture exposes the same family/direction/ASN-width facts to the AS-zero final guard without changing these assertions. Retain enforced for the declared generation choice. Current source was independently re-read; native proof renewal is a separate main-thread prerequisite to stamping this judgment.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L22) | unit/verify | revert, verified |
| negative | [`TestForwardSplitConvertsAddPathContext`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_body_test.go#L24) | unit/verify | not verified |
| negative | [`TestForwardPathIDLeavesNonAddPathDestinationAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L221) | unit/verify | not verified |
| positive | [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestForwardSplitSameContextKeepsRawSplit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_body_test.go#L128) | unit/verify | revert, verified |

### [`RFC7911-5-4`](#rfc7911-5-4)

A BGP speaker MUST follow the procedures defined in [RFC4271] when generating an UPDATE message for a particular <AFI, SAFI> to a peer unless the BGP speaker advertises the ADD-PATH Capability to the peer indicating its ability to send multiple paths for the <AFI, SAFI>, and also receives the ADD-PATH Capability from the peer indicating its ability to receive multiple paths for the <AFI, SAFI>, in which case the speaker MUST generate a route update for the <AFI, SAFI> based on the combination of the address prefix and the Path Identifier, and use the extended NLRI encodings specified in this document. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7911 Section 5: "A BGP speaker MUST follow the procedures defined in [RFC4271] when generating an UPDATE message for a particular <AFI, SAFI> to a peer unless the BGP speaker advertises the ADD-PATH Capability to the peer indicating its ability to send multiple paths for the <AFI, SAFI>, and also receives the ADD-PATH Capability from the peer indicating its ability to receive multiple paths for the <AFI, SAFI>, in which case the speaker MUST generate a route update for the <AFI, SAFI> based on the combination of the address prefix and the Path Identifier, and use the extended NLRI encodings specified in this document." All six tagged covers were read. The family-generation positive and negative tags require exact output for two identifiers 7 and 9 on the same prefix, with no extended framing leaking into the crossed family. Its existing actual capability.Negotiate result is now also published to Session; inputs and exact byte comparisons are unchanged. The producer uses each NLRI's stored identifier and the destination family's negotiated addPath fact, so prefix-only collapse, wrong identifiers, unconditional identifier fields or family leakage fail. TestWriteAnnounceUpdateWithAddPath is only a four-octet width/attribute-length control and is not treated as an exact content proof. TestSplitUpdateAddPathEndToEnd and TestSplitUpdateEndToEnd compare all concatenated extended/plain NLRI and retained attributes through the shared Splitter; despite their historical names/comments, collectSplit calls Splitter.Split directly, not a socket send path. The exact family-generation API test supplies the recipient-byte boundary. Retain enforced for pair-based generation and extended versus ordinary framing, without claiming all base-BGP decisions, socket integration or foreign interoperability. Native renewal of the four family-generation covers remains a main-thread prerequisite to stamping.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestWriteAnnounceUpdateWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L518) | unit/verify | not verified |
| negative | [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L24) | unit/verify | revert, verified |
| negative | [`TestSplitUpdateEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_split_test.go#L254) | unit/verify | revert, verified |
| positive | [`TestWriteAnnounceUpdateWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L517) | unit/verify | not verified |
| positive | [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestSplitUpdateAddPathEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_split_test.go#L304) | unit/verify | revert, verified |

### [`RFC7911-5-5`](#rfc7911-5-5)

The peer SHALL act accordingly in processing an UPDATE message related to a particular <AFI, SAFI>. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7911 Section 5: "The peer SHALL act accordingly in processing an UPDATE message related to a particular <AFI, SAFI>." Read together with the immediately preceding full send/receive negotiation and extended-(prefix,Path Identifier) encoding sentence. All four current carriers read: TestRFC7911AddPathStateAppliesPerFamily, TestEnforceRFC7606_MPAddPathLargePathIDAccepted, TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted and TestRFC7606Section54ReadsTypedNLRIUnderAddPath. The receive matrix positively accepts only the encoding selected for each family and negatively requires exact SessionReset on wrong framing in both IPv4/IPv6 orientations; large identifier octets isolate the framing misread. The typed test now installs a real receive context and verifies the recognized EVPN route retains its identifier while the unknown route disappears. enforceRFC7606 and typedNLRIEdit consult recvCtx.AddPathFor on the specific family. This is receive-side evidence, not an extra generation claim. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7911AddPathStateAppliesPerFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_reactor_b_test.go#L79) | unit/verify | revert, verified |
| negative | [`TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L122) | unit/verify | not verified |
| negative | [`TestEnforceRFC7606_MPAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L74) | unit/verify | not verified |
| positive | [`TestRFC7911AddPathStateAppliesPerFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_reactor_b_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L121) | unit/verify | not verified |
| positive | [`TestEnforceRFC7606_MPAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L73) | unit/verify | not verified |
| positive | [`TestRFC7606Section54ReadsTypedNLRIUnderAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go#L88) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc7911 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc7911.txt |
| Source fingerprint | 950784683306b771 |
| Record | rfc/extraction/rfc7911.json |
| Mapped sentences | 7 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | reviewed | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. Walked rather than skipped because the site scan attributes one site here. That site is the IETF Trust Legal Provisions boilerplate and is excluded below. Nothing before section 1 binds a BGP speaker. |
| `1` | Introduction | 0 | reviewed | Introduction. States that RFC 4271 makes no provision for advertising several paths for one prefix, and that this document defines the Path Identifier that lifts the limit. It reports what the extension does and directs nobody. |
| `1.1` | not stated | 0 | reviewed | Specification of Requirements: the RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker. It is also what puts the lower-case 'must' of the copyright notice outside the normative set. |
| `2` | How to Identify a Path | 2 | reviewed | How to Identify a Path. Two capitalised MUSTs, both mapped below: the Path Identifier is assigned so that (Prefix, Path Identifier) identifies a path uniquely toward one neighbor, and a re-advertising speaker generates its own identifier. The section's third normative sentence, 'A BGP speaker that receives a route should not assume that the identifier carries any particular semantics', writes 'should not' in lower case, so section 1.1 puts it outside the RFC 2119 set. It states no gated obligation and the summary records it under Encoding Rules rather than as a checklist row. |
| `3` | Extended NLRI Encodings | 1 | reviewed | Extended NLRI Encodings. One MUST, mapped below, plus the four-octet Path Identifier diagram. The diagram assigns a field width and states no separate obligation. |
| `4` | ADD-PATH Capability | 1 | reviewed | ADD-PATH Capability. Defines capability code 69 and the AFI/SAFI/Send-Receive tuple, and states one MUST, mapped below. The field descriptions assign values 1, 2 and 3 to receive, send and both; a value assignment is not a directive. The section's one SHOULD, on treating any other Send/Receive value as not understood, is advisory, so the site scan does not see it and it is listed unsourced here. |
| `5` | Operation | 3 | reviewed | Operation. The only section carrying more obligations than sites. Three sites are mapped below, and each of the first two fuses two MUSTs into one sentence, so RFC7911-5-2 (receive the peer's capability with Send/Receive 1 or 3) and RFC7911-5-4 (generate the update on (prefix, Path Identifier) and use the extended encodings) are listed unsourced: 'mapped-to' names one id per site. The section's three SHOULDs are advisory and are listed here for the same reason. Its opening paragraph, that the RFC 4271 advertisement rules are otherwise unchanged and that a new advertisement for the same (prefix, Path Identifier) replaces the previous one, is indicative and states no separate obligation. |
| `6` | Deployment Considerations | 0 | reviewed | Deployment Considerations. States that care is needed in deployment, that the capability exchange is the only explicit indication the extended encoding is in use, and that a packet analyzer without that state cannot decode the UPDATEs. Written in the indicative and in 'could', it directs no speaker. |
| `7` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA has assigned value 69 for the ADD-PATH Capability in the Capability Codes registry. It binds IANA, and the assignment is already an action taken. |
| `8` | Security Considerations | 0 | reviewed | Security Considerations. Names the memory-exhaustion exposure of holding several paths per prefix, states it is not a new vulnerability, and encourages a reader to study [ADDPATH]. No countermeasure is directed at a speaker. |
| `9` | References heading | 0 | skipped (references) | References heading. |
| `9.1` | Normative References: RFC 2119, RFC 4271, RFC 4760, RFC 5492 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 4760, RFC 5492. |
| `9.2` | not stated | 0 | skipped (references) | Informative References: [ADDPATH], [FAST], RFC 3345, RFC 4272, RFC 4724, [STOP-OSC]. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The IETF Trust Legal Provisions boilerplate of the Copyright Notice. Its 'must' is lower case, which section 1.1 puts outside the normative set, and it binds a person who reuses Code Components from the document, never a BGP speaker on the wire. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |

## Superseded

No document obsoletes RFC 7911, so its obligations are stated where they were written.
