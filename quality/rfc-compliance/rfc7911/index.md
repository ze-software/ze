# RFC 7911 - Advertisement of Multiple Paths in BGP

Supported. Every requirement this repository extracted from RFC 7911, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 9 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 61.7% | 37 of 60 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 9 | of 9 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 13 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 60 |
| Tagged units | 60 |
| Recorded audit verdicts | 9 |
| Discrimination records | 37 |
| Summary | `rfc/short/rfc7911.md` |
| Requirement shard | `rfc/requirements/rfc7911.md` |
| RFC text | `rfc/full/rfc7911.txt` |

## Enrolment

Enrolled: Advertisement of Multiple Paths in BGP (ADD-PATH): nine MUST-level requirements. Eight are met: 3-1 (a 4-octet Path Identifier is prepended to NLRI when ADD-PATH is negotiated), 4-1 (a single ADD-PATH capability instance lists all AFI/SAFIs), 5-1 and 5-2 (a path is sent only if the local speaker advertised Send/Both and the remote advertised Receive/Both), 5-3 (Path IDs are not added when ADD-PATH is not negotiated for the family), 5-4 (the RIB is keyed by prefix and Path ID and the egress encodes the Path ID), and 5-5 (a negotiated family's received NLRI is parsed with its 4-octet Path ID) carry positive+negative tags. 2-1 (the same prefix with different Path IDs is treated as different paths) is {single-polarity: positive}: the per-peer RIB key is (prefix, Path ID) by construction. 2-2 (a speaker re-advertising a path generates its own Path Identifier) carries positive+negative tags since 2026-08-14: ze mints its own value per ingress path in `internal/component/bgp/reactor/forward_path_id.go` and both forward rails read it, so no requirement of this RFC is a gap.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

- Per-family send and receive modes, Path ID packing, NLRI path IDs where negotiated
- a re-advertised route carries ze's own Path Identifier ([`RFC7911-2-2`](#rfc7911-2-2)), assigned per ingress path in [`internal/component/bgp/reactor/forward_path_id.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id.go) and read by both the raw same-context forward and the re-encode, so an announcement and its withdraw leave under one value and two clients that chose one identifier for a prefix stay two paths at a third
- tests bound per requirement in [`rfc/requirements/rfc7911.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7911.md).


**What the ledger says remains:**

Closed 2026-08-14: [`RFC7911-2-2`](#rfc7911-2-2). Until then ze relayed the ingress Path Identifier, so a route server merged two clients' paths for one prefix into one and lost a route.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC7911-2-1`](#rfc7911-2-1), [`RFC7911-2-2`](#rfc7911-2-2), [`RFC7911-3-1`](#rfc7911-3-1), [`RFC7911-4-1`](#rfc7911-4-1), [`RFC7911-5-1`](#rfc7911-5-1), [`RFC7911-5-2`](#rfc7911-5-2), [`RFC7911-5-3`](#rfc7911-5-3), [`RFC7911-5-4`](#rfc7911-5-4), [`RFC7911-5-5`](#rfc7911-5-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7911-2-1` | However, the Path Identifier MUST be assigned in such a way that the BGP speaker is able to use the (Prefix, Path Identifier) to uniquely identify a path advertised to a neighbor. (Section 2) | MUST | 2 - How to Identify a Path | **positive:** `unit/verify` [`TestForwardPathIDStableAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L96). **negative:** `unit/verify` [`TestForwardPathIDsDifferForCollidingSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L45) |
| `RFC7911-2-2` | A BGP speaker that re-advertises a route MUST generate its own Path Identifier to be associated with the re-advertised route. (Section 2) | MUST | 2 - How to Identify a Path | **positive:** `unit/verify` [`TestForwardPathIDBoundaryReceivedValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L166). **positive:** `unit/verify` [`TestForwardPathIDDiffersForTwoSourcePeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L48). **positive:** `unit/verify` [`TestForwardPathIDFiniteNamespace`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L244). **positive:** `unit/verify` [`TestForwardPathIDKeepsTheSourceOfARebuiltFrame`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L294). **positive:** `unit/verify` [`TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L329). **positive:** `unit/verify` [`TestForwardPathIDMPGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_mp_test.go#L25). **positive:** `unit/verify` [`TestForwardPathIDMatchesAnnounceAndWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L75). **positive:** `unit/verify` [`TestForwardPathIDSeparatesNonAddPathSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L137). **positive:** `unit/verify` [`TestForwardPathIDStableForEachDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L190). **positive:** `unit/verify` [`TestForwardPathIDSurvivesAttributeChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L108). **positive:** `unit/verify` [`TestForwardPathIDWithdrawCarriesTheAnnouncedValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L239). **positive:** `unit/verify` [`TestForwardPathIDsDifferForCollidingSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L33). **positive:** `unit/verify` [`TestForwardPathIDsFreedOnRelayedWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L200). **positive:** `unit/verify` [`TestPathIDKeyFollowsWhatTheSourceFramed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_zz_pathid_growth_probe_test.go#L22). **negative:** `unit/verify` [`TestForwardPathIDFiniteNamespace`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L246). **negative:** `unit/verify` [`TestForwardPathIDMPGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_mp_test.go#L26). **negative:** `unit/verify` [`TestForwardPathIDStableAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L90). **positive:** `functional/verify` [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L23). **positive:** `interop/nightly` [`checkAddPathReadvertiseCollision`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L16) |
| `RFC7911-3-1` | In order to carry the Path Identifier in an UPDATE message, the NLRI encoding MUST be extended by prepending the Path Identifier field, which is of four octets. (Section 3) | MUST | 3 - Extended NLRI Encodings | **positive:** `unit/verify` [`TestINETWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_inet_test.go#L112). **positive:** `unit/verify` [`TestPathInformationZeroIsAnIdentifierOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc7911_addpath_pathid_test.go#L42). **positive:** `unit/verify` [`TestWriteNLRI_WithStoredPathID`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_base_len_test.go#L182). **negative:** `unit/verify` [`TestPathInformationIsNotWrittenWithoutTheNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc7911_addpath_pathid_test.go#L84). **negative:** `unit/verify` [`TestWriteNLRI_AddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_base_len_test.go#L143). **positive:** `functional/verify` [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L16). **negative:** `functional/verify` [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L19) |
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

RFC 7911 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7911-2-1`](#rfc7911-2-1)

However, the Path Identifier MUST be assigned in such a way that the BGP speaker is able to use the (Prefix, Path Identifier) to uniquely identify a path advertised to a neighbor. (Section 2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read RFC7911 Section 2 in full and both current tagged functions. TestForwardPathIDsDifferForCollidingSources supplies two distinct ingress SourceIDs announcing the same native prefix under the same received identifier and requires different outgoing identifiers. TestForwardPathIDStableAcrossUpdates requires the first source's exact native prefix/identifier pair to remain stable after the colliding second source and a repeat advertisement. The shared output reader checks the actual prefix as well as the identifier, one complete NLRI and no trailing framing. These discriminate source collision and per-message minting without inventing a requirement that a local number differ numerically from the received number. These two carriers exercise buildFwdBody's same-context output, not a remote receiver; complementary RFC7911-2-2 carriers cover converted output and runtime writing. Inspected generatePath, canonical ingress keys, same-context rewrite, converted NLRI handling and both final writer dispatches preserve the generated association. No residual whole-clause gap identified.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardPathIDsDifferForCollidingSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDStableAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L96) | unit/verify | revert, verified |

### [`RFC7911-2-2`](#rfc7911-2-2)

A BGP speaker that re-advertises a route MUST generate its own Path Identifier to be associated with the re-advertised route. (Section 2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read RFC7911 Section 2 and all current tagged carriers. Colliding sources must remain distinct for the same native prefix, while replacements and inverse withdrawals retain the locally generated association. Current assertions cover received identifiers zero and maximum, occupied wrap candidates, unframed ingress, independent destinations, IPv4 same/cross-context conversion, and IPv6 MP_REACH/MP_UNREACH same-context, cross-context and unframed-source conversion. The changed probe checks exact prefix/identifier pairing and that retiring one prefix cannot renumber another using the same ingress identifier. Churn carriers drive reactorForwardRS through actual destination writes and cache eviction, including rebuilt reflection frames and a mixed withdraw/reannounce; they check exact recipient-visible identities rather than treating internal table counts as the RFC obligation. The foreign FRR checker separately requires two source-attributed paths with distinct identifiers and stable replay; the functional carrier separately checks live/replay wire framing. Its pinned zero is a deterministic fixture expectation, not an RFC allocation rule. The allocator, rewrite, rebuilt SourceID preservation, release boundary and raw/converted writer consumers were traced. No numerical inequality from ingress, globally unique identifiers across prefixes, identical numbers across destinations or allocator reclamation policy is claimed as a normative requirement. No residual whole-clause gap identified.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardPathIDFiniteNamespace`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L246) | unit/verify | revert, verified |
| negative | [`TestForwardPathIDMPGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_mp_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestForwardPathIDStableAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDKeepsTheSourceOfARebuiltFrame`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L294) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L329) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDWithdrawCarriesTheAnnouncedValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L239) | unit/verify | revert, verified |
| positive | [`TestForwardPathIDsFreedOnRelayedWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_churn_test.go#L200) | unit/verify | revert, verified |
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
| positive | [`checkAddPathReadvertiseCollision`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L16) | interop/nightly | revert, verified |
| positive | [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L23) | functional/verify | revert, verified |

### [`RFC7911-3-1`](#rfc7911-3-1)

In order to carry the Path Identifier in an UPDATE message, the NLRI encoding MUST be extended by prepending the Path Identifier field, which is of four octets. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: NLRI encoding MUST be extended by prepending the four-octet Path Identifier field. Forbidden: no field, a field of other size, or the field placed after the prefix. TestWriteNLRI_WithStoredPathID asserts buf[0..3]==0000002a and n==4+Len() (red if absent, mis-sized, or appended, since buf[0] would then be the prefix length); TestWriteNLRI_AddPath disabled subtest asserts n==Len() without ADD-PATH; TestINETWithAddPath decodes 00000001 then 8,10 as id 1 and 10.0.0.0/8; test/plugin/adj-rib-in-replay-addpath-source.ci asserts exact wire hex 00000000180A0000 vs 180A0000 per destination negotiation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPathInformationIsNotWrittenWithoutTheNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc7911_addpath_pathid_test.go#L84) | unit/verify | unproven |
| negative | [`TestWriteNLRI_AddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_base_len_test.go#L143) | unit/verify | unproven |
| negative | [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L19) | functional/verify | unproven |
| positive | [`TestPathInformationZeroIsAnIdentifierOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc7911_addpath_pathid_test.go#L42) | unit/verify | unproven |
| positive | [`TestWriteNLRI_WithStoredPathID`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_base_len_test.go#L182) | unit/verify | unproven |
| positive | [`TestINETWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/rfc7911_inet_test.go#L112) | unit/verify | unproven |
| positive | [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L16) | functional/verify | unproven |

### [`RFC7911-4-1`](#rfc7911-4-1)

A BGP speaker that wishes to indicate support for multiple AFI/SAFIs MUST do so by including the information in a single instance of the ADD-PATH Capability. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Now reads Ze's built OPEN. Positive: default send/receive over two families -> exactly one code-69 instance listing both, mode 3. Negative: per-family overrides only over three families with directions 2/1/3 -> still one instance holding all three. Judge break: config_capabilities.go appending one AddPath capability per family turned both red. Old parse-level TestAddPathCapability/TestAddPathMultipleFamilies tags are supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7911PerFamilyAddPathStaysOneInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L194) | unit/verify | revert, verified |
| negative | [`TestAddPathCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L374) | unit/verify | unproven |
| positive | [`TestRFC7911OpenCarriesOneAddPathInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L174) | unit/verify | revert, verified |
| positive | [`TestAddPathMultipleFamilies`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L403) | unit/verify | unproven |

### [`RFC7911-5-1`](#rfc7911-5-1)

For a BGP speaker to be able to send multiple paths to its peer, that BGP speaker MUST advertise the ADD-PATH Capability with the Send/ Receive field set to either 2 or 3, and MUST receive from its peer the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the corresponding <AFI, SAFI>. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7911 Section 5: "For a BGP speaker to be able to send multiple paths to its peer, that BGP speaker MUST advertise the ADD-PATH Capability with the Send/Receive field set to either 2 or 3, and MUST receive from its peer the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the corresponding <AFI, SAFI>." All three current carriers read: capability/negotiated_test.go TestNegotiateAddPath, context/rfc7911_negotiated_test.go TestFromNegotiatedAddPath (all nine direction-mode pairs), capability/rfc7911_family_scope_test.go TestRFC7911AddPathNegotiationIsPerFamily. The new matrix crosses IPv4/IPv6 and unicast/multicast, both send-capable local modes and both receive-capable peer modes, while MP negotiation offers all families; exactly the intersection family can send. Thus local Receive-only, remote Send-only, wrong AFI and wrong SAFI independently cannot grant send. Negotiate keys both mode maps by the full Family and FromNegotiatedSend/Recv preserve the directional result. This closes the prior family-blindness gap rather than merely re-sealing it. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step. Post-lint independent source rejudgment: TestRFC7911AddPathNegotiationIsPerFamily now computes matchingFamilies = (localFamily == remoteFamily), then wantSend = matchingFamilies && (fam == localFamily). By equality transitivity this is exactly (fam == localFamily && fam == remoteFamily): false for crossed AFI or SAFI; true only for the matching queried family. All three session families are offered by MP on both sides, so lack of MP negotiation cannot mask leakage. All four local-send/remote-receive combinations and all nine family pairs still run. Re-read both companion carriers, including the nine direction-mode context matrix, and Negotiate maps indexed by the full Family. Positive and negative tags test distinct matrix inputs. Enforced remains; all four existing records on this changed unit require native renewal, which is not observed here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L13) | unit/verify | revert, verified |
| negative | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L168) | unit/verify | unproven |
| positive | [`TestNegotiateAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L81) | unit/verify | unproven |
| positive | [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L12) | unit/verify | revert, verified |
| positive | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L167) | unit/verify | unproven |

### [`RFC7911-5-2`](#rfc7911-5-2)

For a BGP speaker to be able to send multiple paths to its peer, that BGP speaker MUST advertise the ADD-PATH Capability with the Send/ Receive field set to either 2 or 3, and MUST receive from its peer the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the corresponding <AFI, SAFI>. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7911 Section 5: "For a BGP speaker to be able to send multiple paths to its peer, that BGP speaker MUST advertise the ADD-PATH Capability with the Send/Receive field set to either 2 or 3, and MUST receive from its peer the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the corresponding <AFI, SAFI>." All three current carriers read: capability/negotiated_test.go TestNegotiateAddPath, context/rfc7911_negotiated_test.go TestFromNegotiatedAddPath (all nine direction-mode pairs), capability/rfc7911_family_scope_test.go TestRFC7911AddPathNegotiationIsPerFamily. The new matrix crosses IPv4/IPv6 and unicast/multicast, both send-capable local modes and both receive-capable peer modes, while MP negotiation offers all families; exactly the intersection family can send. Thus local Receive-only, remote Send-only, wrong AFI and wrong SAFI independently cannot grant send. Negotiate keys both mode maps by the full Family and FromNegotiatedSend/Recv preserve the directional result. This closes the prior family-blindness gap rather than merely re-sealing it. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step. Post-lint independent source rejudgment: the family-equality hoist preserves the truth table, not merely the assertion count. A queried family is send-enabled exactly when localFamily == remoteFamily == fam; crossed AFI, crossed SAFI, or a different queried family are false. Both remote Receive and Both and local Send and Both are crossed while MP offers all families. The companion TestNegotiateAddPath pins Both/Receive to exact Send, and TestFromNegotiatedAddPath tests all nine modes including peer Send-only disabling transmission. Negotiate reads localModes[f] and remoteModes[f] for each negotiated family. Enforced remains, with native renewal owed for both polarities of each row on the changed family-scope unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L15) | unit/verify | revert, verified |
| negative | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L170) | unit/verify | unproven |
| positive | [`TestNegotiateAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L82) | unit/verify | unproven |
| positive | [`TestRFC7911AddPathNegotiationIsPerFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7911_family_scope_test.go#L14) | unit/verify | revert, verified |
| positive | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/rfc7911_negotiated_test.go#L169) | unit/verify | unproven |

### [`RFC7911-5-3`](#rfc7911-5-3)

A BGP speaker MUST follow the procedures defined in [RFC4271] when generating an UPDATE message for a particular <AFI, SAFI> to a peer unless the BGP speaker advertises the ADD-PATH Capability to the peer indicating its ability to send multiple paths for the <AFI, SAFI>, and also receives the ADD-PATH Capability from the peer indicating its ability to receive multiple paths for the <AFI, SAFI> (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7911 §5, rfc/full/rfc7911.txt:241-250: 'A BGP speaker MUST follow the procedures defined in [RFC4271] when generating an UPDATE message for a particular <AFI, SAFI> to a peer unless the BGP speaker advertises the ADD-PATH Capability to the peer indicating its ability to send multiple paths for the <AFI, SAFI>, and also receives the ADD-PATH Capability from the peer indicating its ability to receive multiple paths for the <AFI, SAFI>, in which case the speaker MUST generate a route update for the <AFI, SAFI> based on the combination of the address prefix and the Path Identifier, and use the extended NLRI encodings specified in this document.' Read the full sentence, not only the short row's first clause.
All five covers read: both polarities of rfc7911_family_generation_test.go::TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily negotiate local Send/remote Receive for each of IPv4 unicast, IPv6 unicast and IPv4 multicast, call AnnounceNLRIBatch and inspect written UPDATEs. Exact concatenated NLRI equality requires IDs7/9 only for the negotiated family; other AFI/SAFI output is plain. rfc7911_forward_body_test.go::TestForwardSplitConvertsAddPathContext requires destination-context parsed splitting, size limits, fully consumed plain /24 entries and exact concatenation of all80 original prefixes without IDs. ::TestForwardSplitSameContextKeepsRawSplit requires raw split output, exact source prefixes/order, complete ADD-PATH parsing, unique local identifiers and different identifiers for the same prefixes from two sources. rfc7911_forward_path_id_gen_test.go::TestForwardPathIDLeavesNonAddPathDestinationAlone requires one raw body aliasing the original and no borrowed buffer; this last unit alone is principally a zero-copy control, not the strongest byte-content oracle, and the family/output units supply that oracle.
Producers/consumers read: buildFwdBody → fwdUpdateForDestination/fwdReencodeNLRIs and raw ID regeneration; forward_pool.go::fwdBatchHandler explicitly sends both rawBodies and parsed updates then flushes. Originating AnnounceNLRIBatch uses announceFactsFor's per-family addPath, buildBatchAnnounceUpdate/writeBatchNLRI and announceBatchToPeers/sendUpdateWithSplit. Q1 yes; Q2 yes by exact byte and identifier assertions; Q3 yes through distinct family/context controls; Q4 yes for negotiated-family framing/generation, not every RFC4271 decision rule. No semantic mutant or foreign interoperability was run.
Pending note: retain enforced with this full population, cross-boundary routing and evidence limitations.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L22) | unit/verify | revert, verified |
| negative | [`TestForwardSplitConvertsAddPathContext`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_body_test.go#L24) | unit/verify | unproven |
| negative | [`TestForwardPathIDLeavesNonAddPathDestinationAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_path_id_gen_test.go#L221) | unit/verify | unproven |
| positive | [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestForwardSplitSameContextKeepsRawSplit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_body_test.go#L128) | unit/verify | revert, verified |

### [`RFC7911-5-4`](#rfc7911-5-4)

A BGP speaker MUST follow the procedures defined in [RFC4271] when generating an UPDATE message for a particular <AFI, SAFI> to a peer unless the BGP speaker advertises the ADD-PATH Capability to the peer indicating its ability to send multiple paths for the <AFI, SAFI>, and also receives the ADD-PATH Capability from the peer indicating its ability to receive multiple paths for the <AFI, SAFI>, in which case the speaker MUST generate a route update for the <AFI, SAFI> based on the combination of the address prefix and the Path Identifier, and use the extended NLRI encodings specified in this document. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7911 Section 5: "A BGP speaker MUST follow the procedures defined in [RFC4271] when generating an UPDATE message for a particular <AFI, SAFI> to a peer unless the BGP speaker advertises the ADD-PATH Capability to the peer indicating its ability to send multiple paths for the <AFI, SAFI>, and also receives the ADD-PATH Capability from the peer indicating its ability to receive multiple paths for the <AFI, SAFI>, in which case the speaker MUST generate a route update for the <AFI, SAFI> based on the combination of the address prefix and the Path Identifier, and use the extended NLRI encodings specified in this document." TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily additionally sends identifiers 7 and 9 for the SAME prefix and requires both extended entries exactly; all crossed AFI/SAFI outputs remain plain. reactor_test.go TestWriteAnnounceUpdateWithAddPath contributes the width/control check (not sufficient alone); rfc7911_forward_split_test.go TestSplitUpdateAddPathEndToEnd and TestSplitUpdateEndToEnd retain exact concatenated extended/plain NLRI through splitting. Producers buildBatchAnnounceUpdate generate from each NLRI and the destination family-specific addPath fact. Prefix-only collapse, wrong identifiers, family leakage, or adding the four octets without negotiation fail. The unrelated unknown-withdraw tag is no longer attached to this row. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWriteAnnounceUpdateWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L518) | unit/verify | unproven |
| negative | [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L24) | unit/verify | revert, verified |
| negative | [`TestSplitUpdateEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_split_test.go#L254) | unit/verify | revert, verified |
| positive | [`TestWriteAnnounceUpdateWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L517) | unit/verify | unproven |
| positive | [`TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_family_generation_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestSplitUpdateAddPathEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_forward_split_test.go#L304) | unit/verify | revert, verified |

### [`RFC7911-5-5`](#rfc7911-5-5)

The peer SHALL act accordingly in processing an UPDATE message related to a particular <AFI, SAFI>. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7911 Section 5: "The peer SHALL act accordingly in processing an UPDATE message related to a particular <AFI, SAFI>." Read together with the immediately preceding full send/receive negotiation and extended-(prefix,Path Identifier) encoding sentence. All four current carriers read: TestRFC7911AddPathStateAppliesPerFamily, TestEnforceRFC7606_MPAddPathLargePathIDAccepted, TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted and TestRFC7606Section54ReadsTypedNLRIUnderAddPath. The receive matrix positively accepts only the encoding selected for each family and negatively requires exact SessionReset on wrong framing in both IPv4/IPv6 orientations; large identifier octets isolate the framing misread. The typed test now installs a real receive context and verifies the recognized EVPN route retains its identifier while the unknown route disappears. enforceRFC7606 and typedNLRIEdit consult recvCtx.AddPathFor on the specific family. This is receive-side evidence, not an extra generation claim. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7911AddPathStateAppliesPerFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_reactor_b_test.go#L79) | unit/verify | revert, verified |
| negative | [`TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L122) | unit/verify | unproven |
| negative | [`TestEnforceRFC7606_MPAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L74) | unit/verify | unproven |
| positive | [`TestRFC7911AddPathStateAppliesPerFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_reactor_b_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L121) | unit/verify | unproven |
| positive | [`TestEnforceRFC7606_MPAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7911_session_addpath_test.go#L73) | unit/verify | unproven |
| positive | [`TestRFC7606Section54ReadsTypedNLRIUnderAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go#L88) | unit/verify | revert, verified |

## Extraction sign-off

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
| `front` | not stated | 1 | walked | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. Walked rather than skipped because the site scan attributes one site here. That site is the IETF Trust Legal Provisions boilerplate and is excluded below. Nothing before section 1 binds a BGP speaker. |
| `1` | Introduction | 0 | walked | Introduction. States that RFC 4271 makes no provision for advertising several paths for one prefix, and that this document defines the Path Identifier that lifts the limit. It reports what the extension does and directs nobody. |
| `1.1` | not stated | 0 | walked | Specification of Requirements: the RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker. It is also what puts the lower-case 'must' of the copyright notice outside the normative set. |
| `2` | How to Identify a Path | 2 | walked | How to Identify a Path. Two capitalised MUSTs, both mapped below: the Path Identifier is assigned so that (Prefix, Path Identifier) identifies a path uniquely toward one neighbor, and a re-advertising speaker generates its own identifier. The section's third normative sentence, 'A BGP speaker that receives a route should not assume that the identifier carries any particular semantics', writes 'should not' in lower case, so section 1.1 puts it outside the RFC 2119 set. It states no gated obligation and the summary records it under Encoding Rules rather than as a checklist row. |
| `3` | Extended NLRI Encodings | 1 | walked | Extended NLRI Encodings. One MUST, mapped below, plus the four-octet Path Identifier diagram. The diagram assigns a field width and states no separate obligation. |
| `4` | ADD-PATH Capability | 1 | walked | ADD-PATH Capability. Defines capability code 69 and the AFI/SAFI/Send-Receive tuple, and states one MUST, mapped below. The field descriptions assign values 1, 2 and 3 to receive, send and both; a value assignment is not a directive. The section's one SHOULD, on treating any other Send/Receive value as not understood, is advisory, so the site scan does not see it and it is listed unsourced here. |
| `5` | Operation | 3 | walked | Operation. The only section carrying more obligations than sites. Three sites are mapped below, and each of the first two fuses two MUSTs into one sentence, so RFC7911-5-2 (receive the peer's capability with Send/Receive 1 or 3) and RFC7911-5-4 (generate the update on (prefix, Path Identifier) and use the extended encodings) are listed unsourced: 'mapped-to' names one id per site. The section's three SHOULDs are advisory and are listed here for the same reason. Its opening paragraph, that the RFC 4271 advertisement rules are otherwise unchanged and that a new advertisement for the same (prefix, Path Identifier) replaces the previous one, is indicative and states no separate obligation. |
| `6` | Deployment Considerations | 0 | walked | Deployment Considerations. States that care is needed in deployment, that the capability exchange is the only explicit indication the extended encoding is in use, and that a packet analyzer without that state cannot decode the UPDATEs. Written in the indicative and in 'could', it directs no speaker. |
| `7` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA has assigned value 69 for the ADD-PATH Capability in the Capability Codes registry. It binds IANA, and the assignment is already an action taken. |
| `8` | Security Considerations | 0 | walked | Security Considerations. Names the memory-exhaustion exposure of holding several paths per prefix, states it is not a new vulnerability, and encourages a reader to study [ADDPATH]. No countermeasure is directed at a speaker. |
| `9` | References heading | 0 | skipped (references) | References heading. |
| `9.1` | Normative References: RFC 2119, RFC 4271, RFC 4760, RFC 5492 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 4760, RFC 5492. |
| `9.2` | not stated | 0 | skipped (references) | Informative References: [ADDPATH], [FAST], RFC 3345, RFC 4272, RFC 4724, [STOP-OSC]. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The IETF Trust Legal Provisions boilerplate of the Copyright Notice. Its 'must' is lower case, which section 1.1 puts outside the normative set, and it binds a person who reuses Code Components from the document, never a BGP speaker on the wire. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |

## Superseded

No document obsoletes RFC 7911, so its obligations are stated where they were written.
