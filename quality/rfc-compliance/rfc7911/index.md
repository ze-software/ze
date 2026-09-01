# RFC 7911 - Advertisement of Multiple Paths in BGP

Supported. Every requirement this repository extracted from RFC 7911, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 88.9% | 8 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 11.1% | 1 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 43 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 9 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 13 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 43 |
| Tagged units | 43 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7911.md` |
| Requirement shard | `rfc/requirements/rfc7911.md` |
| RFC text | `rfc/full/rfc7911.txt` |

## Enrolment

Enrolled: Advertisement of Multiple Paths in BGP (ADD-PATH): nine MUST-level requirements. Eight are met: 3-1 (a 4-octet Path Identifier is prepended to NLRI when ADD-PATH is negotiated), 4-1 (a single ADD-PATH capability instance lists all AFI/SAFIs), 5-1 and 5-2 (a path is sent only if the local speaker advertised Send/Both and the remote advertised Receive/Both), 5-3 (Path IDs are not added when ADD-PATH is not negotiated for the family), 5-4 (the RIB is keyed by prefix and Path ID and the egress encodes the Path ID), and 5-5 (a negotiated family's received NLRI is parsed with its 4-octet Path ID) carry positive+negative tags. 2-1 (the same prefix with different Path IDs is treated as different paths) is {single-polarity: positive}: the per-peer RIB key is (prefix, Path ID) by construction. 2-2 (a speaker re-advertising a path generates its own Path Identifier) is {gap}: ze preserves the ingress Path Identifier on re-advertisement rather than minting its own. Disclosed in the docs/features/rfc-status.md RFC 7911 row.

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
| Positive and negative tests | 8 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC7911-2-2`](#rfc7911-2-2), [`RFC7911-3-1`](#rfc7911-3-1), [`RFC7911-4-1`](#rfc7911-4-1), [`RFC7911-5-1`](#rfc7911-5-1), [`RFC7911-5-2`](#rfc7911-5-2), [`RFC7911-5-3`](#rfc7911-5-3), [`RFC7911-5-4`](#rfc7911-5-4), [`RFC7911-5-5`](#rfc7911-5-5)

**Annotated instead of tested (1):** [`RFC7911-2-1`](#rfc7911-2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7911-2-1` | Path Identifier must be assigned so that (Prefix, Path Identifier) uniquely identifies a path advertised to a neighbor (Section 2) | MUST | 2 - How to Identify a Path | **positive:** `unit/verify` [`TestPeerRIB_AddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/peerrib_test.go#L187). **negative:** no negative test. **{single-polarity}:** the RIB keys on (prefix, Path ID) so the paired negative would be a same-(prefix, Path ID) collapse-to-one assertion, but no such dedup/replacement test exists in the suite; TestPeerRIB_AddPath asserts only that distinct Path IDs yield distinct entries |
| `RFC7911-2-2` | A BGP speaker that re-advertises a route must generate its own Path Identifier (not reuse received) (Section 2) | MUST | 2 - How to Identify a Path | **positive:** `unit/verify` [`TestForwardPathIDBoundaryReceivedValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L162). **positive:** `unit/verify` [`TestForwardPathIDDiffersForTwoSourcePeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L45). **positive:** `unit/verify` [`TestForwardPathIDIdenticalForEveryDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L192). **positive:** `unit/verify` [`TestForwardPathIDKeepsTheSourceOfARebuiltFrame`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_churn_test.go#L296). **positive:** `unit/verify` [`TestForwardPathIDMatchesAnnounceAndWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L74). **positive:** `unit/verify` [`TestForwardPathIDReleaseReturnsValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L246). **positive:** `unit/verify` [`TestForwardPathIDSeparatesNonAddPathSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L133). **positive:** `unit/verify` [`TestForwardPathIDSurvivesAttributeChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L104). **positive:** `unit/verify` [`TestForwardPathIDWithdrawCarriesTheAnnouncedValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_churn_test.go#L232). **positive:** `unit/verify` [`TestForwardPathIDsDifferForCollidingSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_test.go#L33). **positive:** `unit/verify` [`TestForwardPathIDsFreedOnRelayedWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_churn_test.go#L196). **positive:** `unit/verify` [`TestPathIDKeyFollowsWhatTheSourceFramed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/zz_pathid_growth_probe_test.go#L208). **positive:** `unit/verify` [`TestWithdrawOnlyUpdateFreesEveryIdentifierItBuys`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/zz_pathid_growth_probe_test.go#L89). **negative:** `unit/verify` [`TestForwardPathIDStableAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_test.go#L85). **positive:** `functional/verify` [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L23). **positive:** `interop/nightly` [`checkAddPathReadvertiseCollision`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L16) |
| `RFC7911-3-1` | NLRI encoding must be extended by prepending the 4-octet Path Identifier field (Section 3) | MUST | 3 - Extended NLRI Encodings | **positive:** `unit/verify` [`TestINETWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/inet_test.go#L112). **positive:** `unit/verify` [`TestWriteNLRI_WithStoredPathID`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/base_len_test.go#L182). **negative:** `unit/verify` [`TestWriteNLRI_AddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/base_len_test.go#L143). **positive:** `functional/verify` [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L16). **negative:** `functional/verify` [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L19) |
| `RFC7911-4-1` | Support for multiple AFI/SAFIs must be indicated in a single instance of the ADD-PATH Capability (Section 4) | MUST | 4 - ADD-PATH Capability | **positive:** `unit/verify` [`TestAddPathMultipleFamilies`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L347). **negative:** `unit/verify` [`TestAddPathCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L318) |
| `RFC7911-5-1` | To send multiple paths, speaker must advertise ADD-PATH Capability with Send/Receive set to 2 or 3 (Section 5) | MUST | 5 - Operation | **positive:** `unit/verify` [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/negotiated_test.go#L167). **positive:** `unit/verify` [`TestNegotiateAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L81). **negative:** `unit/verify` [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/negotiated_test.go#L168) |
| `RFC7911-5-2` | To send multiple paths, speaker must receive ADD-PATH Capability with Send/Receive set to 1 or 3 from peer (Section 5) | MUST | 5 - Operation | **positive:** `unit/verify` [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/negotiated_test.go#L169). **positive:** `unit/verify` [`TestNegotiateAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L82). **negative:** `unit/verify` [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/negotiated_test.go#L170) |
| `RFC7911-5-3` | Speaker must follow RFC 4271 procedures unless ADD-PATH is negotiated for both send and receive (Section 5) | MUST | 5 - Operation | **positive:** `unit/verify` [`TestForwardSplitSameContextKeepsRawSplit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body_test.go#L118). **positive:** `unit/verify` [`TestRFC7606Section54ReadsTypedNLRIUnderAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go#L88). **negative:** `unit/verify` [`TestForwardPathIDLeavesNonAddPathDestinationAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L221). **negative:** `unit/verify` [`TestForwardSplitConvertsAddPathContext`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body_test.go#L23) |
| `RFC7911-5-4` | When ADD-PATH is negotiated, speaker must generate route updates based on (address prefix, Path Identifier) combination using extended NLRI encodings (Section 5) | MUST | 5 - Operation | **positive:** `unit/verify` [`TestForwardPathIDWithdrawOfUnknownPathLeavesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_churn_test.go#L268). **positive:** `unit/verify` [`TestSplitUpdateAddPathEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_split_test.go#L304). **positive:** `unit/verify` [`TestWriteAnnounceUpdateWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L517). **negative:** `unit/verify` [`TestSplitUpdateEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_split_test.go#L254). **negative:** `unit/verify` [`TestWriteAnnounceUpdateWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L518) |
| `RFC7911-5-5` | Peer shall act accordingly in processing an UPDATE message related to a particular AFI/SAFI (Section 5) | SHALL | 5 - Operation | **positive:** `unit/verify` [`TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_addpath_test.go#L121). **positive:** `unit/verify` [`TestEnforceRFC7606_MPAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_addpath_test.go#L73). **negative:** `unit/verify` [`TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_addpath_test.go#L122). **negative:** `unit/verify` [`TestEnforceRFC7606_MPAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_addpath_test.go#L74) |
| `RFC7911-4-2` | Invalid Send/Receive value (not 1, 2, or 3) should be treated as capability not understood and ignored per RFC 5492 (Section 4) | SHOULD | 4 - ADD-PATH Capability | **positive:** no positive test. **negative:** no negative test |
| `RFC7911-5-6` | Withdraw with unknown Path Identifier should be silently ignored (Section 5) | SHOULD | 5 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7911-5-7` | Best route per RFC 4271 should be included when more than one path is advertised, unless path was received from that neighbor (Section 5) | SHOULD | 5 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7911-5-8` | Implementation should take special care that forwarding plane of Receiving Speaker is not affected during graceful restart (Section 5) | SHOULD | 5 - Operation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7911 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7911-2-1`](#rfc7911-2-1)

Path Identifier must be assigned so that (Prefix, Path Identifier) uniquely identifies a path advertised to a neighbor (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPeerRIB_AddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/peerrib_test.go#L187) | unit/verify | unproven |

### [`RFC7911-2-2`](#rfc7911-2-2)

A BGP speaker that re-advertises a route must generate its own Path Identifier (not reuse received) (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardPathIDStableAcrossUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_test.go#L85) | unit/verify | unproven |
| positive | [`TestForwardPathIDKeepsTheSourceOfARebuiltFrame`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_churn_test.go#L296) | unit/verify | unproven |
| positive | [`TestForwardPathIDWithdrawCarriesTheAnnouncedValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_churn_test.go#L232) | unit/verify | unproven |
| positive | [`TestForwardPathIDsFreedOnRelayedWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_churn_test.go#L196) | unit/verify | unproven |
| positive | [`TestForwardPathIDBoundaryReceivedValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L162) | unit/verify | unproven |
| positive | [`TestForwardPathIDDiffersForTwoSourcePeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L45) | unit/verify | unproven |
| positive | [`TestForwardPathIDIdenticalForEveryDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L192) | unit/verify | unproven |
| positive | [`TestForwardPathIDMatchesAnnounceAndWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L74) | unit/verify | unproven |
| positive | [`TestForwardPathIDReleaseReturnsValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L246) | unit/verify | unproven |
| positive | [`TestForwardPathIDSeparatesNonAddPathSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L133) | unit/verify | unproven |
| positive | [`TestForwardPathIDSurvivesAttributeChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L104) | unit/verify | unproven |
| positive | [`TestForwardPathIDsDifferForCollidingSources`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_test.go#L33) | unit/verify | unproven |
| positive | [`TestPathIDKeyFollowsWhatTheSourceFramed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/zz_pathid_growth_probe_test.go#L208) | unit/verify | unproven |
| positive | [`TestWithdrawOnlyUpdateFreesEveryIdentifierItBuys`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/zz_pathid_growth_probe_test.go#L89) | unit/verify | unproven |
| positive | [`checkAddPathReadvertiseCollision`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L16) | interop/nightly | unproven |
| positive | [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L23) | functional/verify | unproven |

### [`RFC7911-3-1`](#rfc7911-3-1)

NLRI encoding must be extended by prepending the 4-octet Path Identifier field (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWriteNLRI_AddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/base_len_test.go#L143) | unit/verify | unproven |
| negative | [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L19) | functional/verify | unproven |
| positive | [`TestWriteNLRI_WithStoredPathID`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/base_len_test.go#L182) | unit/verify | unproven |
| positive | [`TestINETWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/inet_test.go#L112) | unit/verify | unproven |
| positive | [`adj-rib-in-replay-addpath-source.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-addpath-source.ci#L16) | functional/verify | unproven |

### [`RFC7911-4-1`](#rfc7911-4-1)

Support for multiple AFI/SAFIs must be indicated in a single instance of the ADD-PATH Capability (Section 4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAddPathCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L318) | unit/verify | unproven |
| positive | [`TestAddPathMultipleFamilies`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L347) | unit/verify | unproven |

### [`RFC7911-5-1`](#rfc7911-5-1)

To send multiple paths, speaker must advertise ADD-PATH Capability with Send/Receive set to 2 or 3 (Section 5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/negotiated_test.go#L168) | unit/verify | unproven |
| positive | [`TestNegotiateAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L81) | unit/verify | unproven |
| positive | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/negotiated_test.go#L167) | unit/verify | unproven |

### [`RFC7911-5-2`](#rfc7911-5-2)

To send multiple paths, speaker must receive ADD-PATH Capability with Send/Receive set to 1 or 3 from peer (Section 5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/negotiated_test.go#L170) | unit/verify | unproven |
| positive | [`TestNegotiateAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L82) | unit/verify | unproven |
| positive | [`TestFromNegotiatedAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/context/negotiated_test.go#L169) | unit/verify | unproven |

### [`RFC7911-5-3`](#rfc7911-5-3)

Speaker must follow RFC 4271 procedures unless ADD-PATH is negotiated for both send and receive (Section 5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardSplitConvertsAddPathContext`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body_test.go#L23) | unit/verify | unproven |
| negative | [`TestForwardPathIDLeavesNonAddPathDestinationAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_gen_test.go#L221) | unit/verify | unproven |
| positive | [`TestForwardSplitSameContextKeepsRawSplit`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body_test.go#L118) | unit/verify | unproven |
| positive | [`TestRFC7606Section54ReadsTypedNLRIUnderAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go#L88) | unit/verify | unproven |

### [`RFC7911-5-4`](#rfc7911-5-4)

When ADD-PATH is negotiated, speaker must generate route updates based on (address prefix, Path Identifier) combination using extended NLRI encodings (Section 5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSplitUpdateEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_split_test.go#L254) | unit/verify | unproven |
| negative | [`TestWriteAnnounceUpdateWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L518) | unit/verify | unproven |
| positive | [`TestForwardPathIDWithdrawOfUnknownPathLeavesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_path_id_churn_test.go#L268) | unit/verify | unproven |
| positive | [`TestSplitUpdateAddPathEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_split_test.go#L304) | unit/verify | unproven |
| positive | [`TestWriteAnnounceUpdateWithAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L517) | unit/verify | unproven |

### [`RFC7911-5-5`](#rfc7911-5-5)

Peer shall act accordingly in processing an UPDATE message related to a particular AFI/SAFI (Section 5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_addpath_test.go#L122) | unit/verify | unproven |
| negative | [`TestEnforceRFC7606_MPAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_addpath_test.go#L74) | unit/verify | unproven |
| positive | [`TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_addpath_test.go#L121) | unit/verify | unproven |
| positive | [`TestEnforceRFC7606_MPAddPathLargePathIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_addpath_test.go#L73) | unit/verify | unproven |

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
