# DRAFT-IETF-SIDROPS-ASPA-VERIFICATION - BGP AS_PATH Verification Based on Autonomous System Provider Authorization (ASPA) Objects

Partial. Every requirement this repository extracted from DRAFT-IETF-SIDROPS-ASPA-VERIFICATION, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 87.5% | 7 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 8 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 77.8% | 14 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 18 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 12.5% | 1 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 18 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 5 |
| Discrimination records | 14 |
| Summary | `rfc/short/draft-ietf-sidrops-aspa-verification.md` |
| Requirement shard | `rfc/requirements/draft-ietf-sidrops-aspa-verification.md` |
| RFC text | `rfc/drafts/draft-ietf-sidrops-aspa-verification.txt` |

## Enrolment

Enrolled: Router-verification requirements follow the cached revision 28. ASPA registration and AS-migration duties remain separate from router verification.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Sections 5.5 and 5.6 upstream/downstream verification, consecutive-prepend compression, Empty/AS_SET Invalid outcomes, IPv4/IPv6 unicast scope, role-based algorithm selection, and cache-change re-evaluation. `aspa_verify.go` produces path verdicts
- `rpki_config.go` selects the algorithm and defaults Invalid policy to reject
- `rpki.go` combines origin and path policy and dispatches eligibility decisions. This describes the source, not a fresh test or discrimination result. Section 5.1 prerequisite-order coverage and proof for the restored permanent IDs remain unverified. Extraction and discrimination provenance migration remains pending
- the former neighbor-check SHOULD and mismatch SHALL have unresolved source-level identities in revision 28. Current validation and discrimination of the implemented paths, Section 5.1 prerequisite ordering, and final migration of the retired revision-27 source identities remain unresolved. The two retired IDs are reserved, not reassigned to different obligations.


**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1`](#draft-ietf-sidrops-aspa-verification-4-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1`](#draft-ietf-sidrops-aspa-verification-5.1-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1`](#draft-ietf-sidrops-aspa-verification-5.4-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2`](#draft-ietf-sidrops-aspa-verification-5.4-2), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1`](#draft-ietf-sidrops-aspa-verification-5.6-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1`](#draft-ietf-sidrops-aspa-verification-6.2-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2`](#draft-ietf-sidrops-aspa-verification-6.2-2)

**Annotated (including scoped evidence) (1):** [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2`](#draft-ietf-sidrops-aspa-verification-5.1-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1` | Normally, a SPAS (see Section 3) is not expected to contain both an AS 0 and other Provider ASes, but an unexpected presence of AS 0 has no influence on the AS_PATH verification procedures (see Section 5.3, Section 5). (§4) | MUST | 4 | **positive:** `unit/verify` [`TestASPAZeroBesideProvidersIsNoWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_zero_test.go#L28). **positive:** `unit/verify` [`TestASPAZeroDoesNotAuthorizeOrInvalidate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L212). **negative:** `unit/verify` [`TestASPAZeroBesideProvidersIsNoWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_zero_test.go#L34). **negative:** `unit/verify` [`TestASPAZeroDoesNotAuthorizeOrInvalidate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L213) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2` | If the aforementioned AS_PATH checks and error handling are implemented, they MUST be applied prior to ASPA verification. (Section 5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** entrypoint coverage for this newly explicit ordering obligation is unverified; Session.processMessage applies existing checks before semantic UPDATE delivery, but the prior neighbor-AS tests have not been established as ordering proof |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1` | Let the sequence COMPRESSED_AS_PATH {AS(N), AS(N-1),..., AS(2), AS(1)} represent the AS_PATH after removing consecutive duplicate ASNs, where AS(1) is the origin AS, and AS(N) is the most recently added neighbor AS of the receiving/verifying AS. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestASPACompressedUpdatePrependsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_compressed_aspa_test.go#L97). **negative:** `unit/verify` [`TestASPACompressedUpdatePreservesHops`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_compressed_aspa_test.go#L149) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1` | The upstream verification algorithm described here is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS. (§5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_role_mode_test.go#L32). **negative:** `unit/verify` [`TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_role_mode_test.go#L33) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2` | If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid" (Section 5.5; Section 5.6) | MUST | 5.5 | **positive:** `unit/verify` [`TestASPAVerifyASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L56). **negative:** `unit/verify` [`TestASPAStateForPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L180) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1` | If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (see Section 3) and MUST be kept in the Adj-RIB-In for potential future re-evaluation (see [RFC9324]). (§5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestASPARetainedPathReplayRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L59). **positive:** `unit/verify` [`TestASPARetentionReceiveRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L183). **negative:** `unit/verify` [`TestASPARetainedPathReplayRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L62). **negative:** `unit/verify` [`TestASPARetentionReplacementWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L232) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1` | The verification procedures described in this document MUST be applied to BGP routes with {AFI, SAFI} combinations {AFI 1 (IPv4), SAFI 1} and {AFI 2 (IPv6), SAFI 1} [IANA-AF] [IANA-SAF]. (Section 6.2) | MUST | 6.2 - The two uppercase MUST sites retain their AFI/SAFI IDs | **positive:** `unit/verify` [`TestASPAAppliesToIPv6Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L110). **negative:** `unit/verify` [`TestASPAAppliesToIPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L132) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2` | The procedures MUST NOT be applied to other address families by default (Section 6.2) | MUST NOT | 6.2 - The two uppercase MUST sites retain their AFI/SAFI IDs | **positive:** `unit/verify` [`TestASPAAppliesToIPv6Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L111). **negative:** `unit/verify` [`TestASPANotAppliedToOtherFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L165) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-2` | If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (Section 5.7) | SHOULD | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-3` | When a route is evaluated as Unknown (using ASPA- based AS_PATH verification), it SHOULD be treated at the same preference level as a route evaluated as Valid. (§5.7) | SHOULD | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-4` | The specific configuration of a mitigation policy based on AS_PATH verification using ASPA is at the discretion of the network operator. However, the following mitigation policy is RECOMMENDED. (§5.7) | RECOMMENDED | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-3` | However, the procedures are NOT RECOMMENDED for use on internal BGP (iBGP) sessions or eBGP sessions internal to an AS Confederation. (§6.2) | NOT RECOMMENDED | 6.2 - The two uppercase MUST sites retain their AFI/SAFI IDs | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.3-1` | The BGP Role configuration parameter and its cross-check in BGP OPEN message as specified in [RFC9234] are RECOMMENDED. (§6.3) | RECOMMENDED | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.3-2` | The configured BGP Roles SHOULD be used to automate the use of the above-described AS_PATH verification procedures helping to distinguish whether upstream or downstream procedures should be applied. (§6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.4-1` | If multiple eBGP sessions can segregate the Complex peering relationship into eBGP sessions with normal peering relationships, the receiving/verifying AS SHOULD select the algorithm (per Section 5.5 or Section 5.6) for each of the normal sessions based on its peering relation type (Section 6.4) | SHOULD | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.6-1` | For any route with an Invalid AS_PATH, the cause of the Invalid state SHOULD be logged for monitoring and diagnostic purposes (Section 6.6) | SHOULD | 6.6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8.4-1` | The implementation of the procedures utilizing the OTC Attribute is RECOMMENDED to complement the ASPA-based AS_PATH verification (Section 8.4) | RECOMMENDED | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.4-2` | If a Complex peering relation cannot be segregated (i.e., when a Complex BGP relationship occurs within one single BGP session), an operator may want to achieve an equivalent outcome by applying an appropriate algorithm (Section 5.5 or Section 5.6) on a per-prefix basis corresponding to the peering relation for the prefix. If this option is not feasible, then an operator MAY apply the algorithm for downstream paths (Section 5.6) to avoid false positive outcomes. (§6.4) | MAY | 6.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2`](#draft-ietf-sidrops-aspa-verification-5.1-2) If the aforementioned AS_PATH checks and error handling are implemented, they MUST be applied prior to ASPA verification. (Section 5.1) | {gap}, no test | entrypoint coverage for this newly explicit ordering obligation is unverified; Session.processMessage applies existing checks before semantic UPDATE delivery, but the prior neighbor-AS tests have not been established as ordering proof |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1`](#draft-ietf-sidrops-aspa-verification-4-1)

Normally, a SPAS (see Section 3) is not expected to contain both an AS 0 and other Provider ASes, but an unexpected presence of AS 0 has no influence on the AS_PATH verification procedures (see Section 5.3, Section 5). (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: AS 0 beside other providers changing the verification outcome. TestASPAZeroDoesNotAuthorizeOrInvalidate: {0,200} Valid, {0} alone Invalid. TestASPAZeroBesideProvidersIsNoWildcard: {0,200} = {200} = Valid (AS 0 does not invalidate) and {0,999} = {999} = Invalid for a path whose provider 200 is unlisted (AS 0 is no wildcard).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPAZeroDoesNotAuthorizeOrInvalidate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L213) | unit/verify | revert, verified |
| negative | [`TestASPAZeroBesideProvidersIsNoWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_zero_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestASPAZeroDoesNotAuthorizeOrInvalidate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L212) | unit/verify | revert, verified |
| positive | [`TestASPAZeroBesideProvidersIsNoWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_zero_test.go#L28) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2`](#draft-ietf-sidrops-aspa-verification-5.1-2)

If the aforementioned AS_PATH checks and error handling are implemented, they MUST be applied prior to ASPA verification. (Section 5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1`](#draft-ietf-sidrops-aspa-verification-5.1-1)

Let the sequence COMPRESSED_AS_PATH {AS(N), AS(N-1),..., AS(2), AS(1)} represent the AS_PATH after removing consecutive duplicate ASNs, where AS(1) is the origin AS, and AS(N) is the most recently added neighbor AS of the receiving/verifying AS. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a COMPRESSED_AS_PATH that keeps consecutive duplicates, removes non-consecutive ones, or reverses origin and neighbor. TestASPACompressedUpdatePrependsValid asserts state valid and accepted for prepends within a segment, across a segment boundary and on a single AS, red if a duplicate survives as an unauthorized self-hop; with 100 as neighbor and 300 as origin, a reversed order also turns it Invalid. TestASPACompressedUpdatePreservesHops asserts invalid and rejected for the separated repeat [100 200 300 200], red if non-consecutive ASNs are dropped.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPACompressedUpdatePreservesHops`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_compressed_aspa_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestASPACompressedUpdatePrependsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_compressed_aspa_test.go#L97) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1`](#draft-ietf-sidrops-aspa-verification-5.4-1)

The upstream verification algorithm described here is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS. (§5.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a route from a Customer or Peer, by an RS from an RS-client, or by an RS-client from an RS verified other than by the upstream algorithm. TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes asserts configuredASPAMode == aspaUpstream for local roles provider, peer, rs and rs-client (one per listed case) and that the down-ramp path is Invalid under each, red if any maps to downstream; local role customer maps to aspaDownstream and verifies the same path Valid.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_role_mode_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_role_mode_test.go#L32) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2`](#draft-ietf-sidrops-aspa-verification-5.4-2)

If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid" (Section 5.5; Section 5.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPAStateForPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L180) | unit/verify | revert, verified |
| positive | [`TestASPAVerifyASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L56) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1`](#draft-ietf-sidrops-aspa-verification-5.6-1)

If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (see Section 3) and MUST be kept in the Adj-RIB-In for potential future re-evaluation (see [RFC9324]). (§5.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an Invalid route selected, or an Invalid route dropped from the Adj-RIB-In. TestASPARetentionReceiveRecovery asserts after an i decision that the stored route is Ineligible with its full attribute, NLRI and next-hop bytes (red if dropped) and validationSelection(false): not in Loc-RIB and not in best-path replay (red if selectable), then selectable after an a decision without another UPDATE. TestASPARetentionReplacementWithdrawal asserts a rejected replacement removes the old selection and retains the new bytes. adj_rib_in TestASPARetainedPathReplayRecovery covers retention on the Adj-RIB-In plugin.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPARetainedPathReplayRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L62) | unit/verify | unproven |
| negative | [`TestASPARetentionReplacementWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L232) | unit/verify | unproven |
| positive | [`TestASPARetainedPathReplayRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L59) | unit/verify | unproven |
| positive | [`TestASPARetentionReceiveRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L183) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1`](#draft-ietf-sidrops-aspa-verification-6.2-1)

The verification procedures described in this document MUST be applied to BGP routes with {AFI, SAFI} combinations {AFI 1 (IPv4), SAFI 1} and {AFI 2 (IPv6), SAFI 1} [IANA-AF] [IANA-SAF]. (Section 6.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an IPv4 unicast or IPv6 unicast route left unverified. TestASPAAppliesToIPv6Unicast asserts family ipv6/unicast reaches the decision path with aspaState ASPAInvalid and is tracked, red if MP_REACH IPv6 unicast is skipped. TestASPAAppliesToIPv4Unicast asserts ipv4/unicast carries ASPAInvalid, red if the plain-NLRI branch is skipped.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPAAppliesToIPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestASPAAppliesToIPv6Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L110) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2`](#draft-ietf-sidrops-aspa-verification-6.2-2)

The procedures MUST NOT be applied to other address families by default (Section 6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPANotAppliedToOtherFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestASPAAppliesToIPv6Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L111) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | Main integration; independent source-mapping review: SourceIdentityReview |
| Signed off | 2026-09-22 |
| Register | rfc2119 |
| Source | rfc/drafts/draft-ietf-sidrops-aspa-verification.txt |
| Source fingerprint | b90c021fcc40be45 |
| Record | rfc/extraction/draft-ietf-sidrops-aspa-verification.json |
| Mapped sentences | 4 |
| Declined as scope | 6 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Abstract, draft status, copyright and table of contents; no router-verification obligation. |
| `1` | not stated | 0 | walked | Introduction describes threats and the purpose of the procedures; it adds no distinct checklist obligation. |
| `2` | not stated | 0 | walked | BCP 14 keyword interpretation boilerplate; no operational requirement. |
| `3` | not stated | 0 | walked | Terminology defines route ineligibility, CAS, PAS, SPAS and local peering relationships used by the later algorithms. |
| `4` | not stated | 4 | walked | The indicative verification rule retained under 4-1 is: "Normally, a SPAS (see Section 3) is not expected to contain both an AS 0 and other Provider ASes, but an unexpected presence of AS 0 has no influence on the AS_PATH verification procedures (see Section 5.3, Section 5)." It carries no uppercase MUST, so it is declared here rather than mapped to a registration site. The uppercase registration sites bind the separate registrar. The SHOULDs to register an ASPA, use a single object, enforce that practice in CA hosting software and keep registries identical, plus the RECOMMENDED advance inclusion of standby providers, likewise address registration/CA operations. Revision 28 omits revision 27's AS0-registration MUST, so old site 4:5 has no current counterpart. |
| `5` | not stated | 0 | walked | The procedures apply to four-octet-compatible speakers and use the reconstructed AS_PATH when AS_PATH and AS4_PATH arrive together. This input definition is retained in the summary's prerequisite discussion. Former section-5 neighbor-check SHOULD and mismatch SHALL sentences are absent; their allocated IDs are source-level conflicts documented in aspa-metadata-migration.json, not assigned to unrelated current sites. |
| `5.1` | not stated | 2 | walked | The AS_SET treat-as-withdraw MUST is attributed to RFC 9774/RFC 7606. The draft's own MUST is: "If the aforementioned AS_PATH checks and error handling are implemented, they MUST be applied prior to ASPA verification." This is DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2. The neighbor-AS check is described with the transparent route-server exception and attributed to RFC 4271/RFC 7606. The following sentence describes ASPA Invalid if the prerequisites are not enforced; it does not restate the former unconditional SHALL. |
| `5.2` | not stated | 0 | walked | The compression definition retains 5.1-1: "Let the sequence COMPRESSED_AS_PATH {AS(N), AS(N-1),..., AS(2), AS(1)} represent the AS_PATH after removing consecutive duplicate ASNs, where AS(1) is the origin AS, and AS(N) is the most recently added neighbor AS of the receiving/verifying AS." The definition is indicative prose. The section also defines the ramp topology and apex separation used by the formal algorithms. |
| `5.3` | not stated | 0 | walked | Defines the union of cryptographically valid ASPA provider sets and the Provider+, Not Provider+ and No Attestation results. The summary retains the function table and the restriction that No Attestation means no retrieved or cryptographically valid ASPA. |
| `5.4` | not stated | 0 | walked | Defines maximum/minimum up-ramp and down-ramp lengths and their Invalid/Unknown/Valid bounds; the summary's ramp table and algorithm steps retain these indicative rules. This is not the current source of the permanently allocated 5.4-1 and 5.4-2 IDs. |
| `5.5` | not stated | 0 | walked | The role obligation retaining 5.4-1 reads: "The upstream verification algorithm described here is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS." Step 3 retains 5.4-2: "If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid"." Both are indicative algorithm rules. Empty path, neighbor mismatch with the RS-client exception, ramp denial, missing attestation and the Valid fallback are retained in the summary's upstream algorithm. |
| `5.6` | not stated | 0 | walked | Downstream applies to a route from a Provider and uses both ramps. Step 3 repeats the AS_SET Invalid obligation already recorded as DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2 under section 5.5; no duplicate requirement is allocated. The summary retains all downstream steps. This is not the source of the old mitigation IDs. |
| `5.7` | not stated | 1 | walked | The MUST-bearing Invalid sentence maps to retained ID 5.6-1, and also says the route "SHOULD be considered ineligible for route selection" (5.6-2). The same paragraph is one inventory site, so the SHOULD has no separate mapping. The other advisory rows come from: "When a route is evaluated as Unknown (using ASPA-based AS_PATH verification), it SHOULD be treated at the same preference level as a route evaluated as Valid." (5.6-3), and "However, the following mitigation policy is RECOMMENDED." (5.6-4), preceded by operator discretion over the specific policy configuration. |
| `6` | Introduces deployment recommendations | 0 | walked | Introduces deployment recommendations. |
| `6.1` | not stated | 0 | walked | Links illustrative verification examples; no additional normative requirement. |
| `6.2` | The two uppercase MUST sites retain their AFI/SAFI IDs | 2 | walked | The two uppercase MUST sites retain their AFI/SAFI IDs. The advisory row reads: "However, the procedures are NOT RECOMMENDED for use on internal BGP (iBGP) sessions or eBGP sessions internal to an AS Confederation." The ingress placement and external Confederation boundary context are retained. |
| `6.3` | not stated | 0 | walked | 6.3-1: "The BGP Role configuration parameter and its cross-check in BGP OPEN message as specified in [RFC9234] are RECOMMENDED." 6.3-2: "The configured BGP Roles SHOULD be used to automate the use of the above-described AS_PATH verification procedures helping to distinguish whether upstream or downstream procedures should be applied." Both are advisory and outside the uppercase MUST inventory. |
| `6.4` | not stated | 0 | walked | 6.4-1: "If multiple eBGP sessions can segregate the Complex peering relationship into eBGP sessions with normal peering relationships, the receiving/verifying AS SHOULD select the algorithm (per Section 5.5 or Section 5.6) for each of the normal sessions based on its peering relation type." For a relation that cannot be segregated and where the described per-prefix option is infeasible, 6.4-2 reads: "If this option is not feasible, then an operator MAY apply the algorithm for downstream paths (Section 5.6) to avoid false positive outcomes." |
| `6.5` | not stated | 1 | walked | The migration notification MUST binds the AS operator's customer-notification process, as classified at 6.5:1. |
| `6.6` | not stated | 0 | walked | The advisory requirement reads: "For any route with an Invalid AS_PATH, the cause of the Invalid state SHOULD be logged for monitoring and diagnostic purposes." The following prose suggests listing Not Provider+ hops and warns that the logging router cannot necessarily identify the leak's author. |
| `7` | not stated | 0 | walked | Security considerations heading; substantive text follows in subsections. |
| `7.1` | not stated | 0 | walked | Explains the cross-family permissiveness caused by a single union of IPv4 and IPv6 providers; retained in the summary. |
| `7.2` | not stated | 0 | walked | Operational advice to maintain correct ASPAs and monitor customer SPAS membership; explains reduced detection or false Invalid caused by erroneous provider sets. The lowercase modals add no uppercase requirement site. |
| `7.3` | not stated | 0 | walked | Describes provider-origin/path manipulation that ASPA may not detect and its operational consequences; no new verification rule. |
| `7.4` | not stated | 0 | walked | ASPA cannot detect added or removed prepends, but that change alone does not affect route-leak detection. It does not authorise removing non-consecutive repeats. |
| `8` | Relation-to-other-technologies heading | 0 | walked | Relation-to-other-technologies heading. |
| `8.1` | Explains complementary origin verification with ROAs | 0 | walked | Explains complementary origin verification with ROAs. |
| `8.2` | not stated | 0 | walked | Explains complementary cryptographic path protection with BGPsec; no ASPA requirement added. |
| `8.3` | not stated | 0 | walked | Compares ASPA with Peerlock; descriptive text adds no distinct algorithm obligation. |
| `8.4` | not stated | 0 | walked | The advisory requirement reads: "The implementation of the procedures utilizing the OTC Attribute is RECOMMENDED to complement the ASPA-based AS_PATH verification." It follows the limitations on preventing locally initiated leaks and detecting leaks through Complex relationships. |
| `9` | not stated | 0 | skipped (iana) | The section states that the document includes no request to IANA. |
| `10` | not stated | 0 | walked | Describes third-party implementation reports under RFC 7942 and expressly disclaims independent verification. These reports supply no Ze execution evidence. |
| `11` | Reference-list heading | 0 | skipped (references) | Reference-list heading. |
| `11.1` | not stated | 0 | skipped (references) | Normative bibliography; titles and dates are not router requirements. |
| `11.2` | Informative bibliography | 0 | skipped (references) | Informative bibliography. |
| `A` | Acknowledges contributors | 0 | skipped (acknowledgements) | Acknowledges contributors. |
| `B` | not stated | 0 | walked | Explains route-leak, forged-origin and forged-segment detection properties and assumptions during partial deployment; introduces no new procedure. Authors' addresses follow without a numbered heading. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 4 says: "An AS MUST list in its SPAS the union of all its Provider AS(es) and non-transparent RS AS(es) at which it is an RS-client." This action belongs to the resource holder's ASPA registration service. internal/component/bgp/plugins/rpki/aspa_cache.go::ApplyDelta stores customer/provider records received through RTR; internal/component/bgp/plugins/rpki/aspa_cache.go::checkPair and internal/component/bgp/plugins/rpki/rpki.go::handleStructuredUpdate consume them for verification. These functions neither choose the resource holder's published SPAS nor register signed objects. The registrar acts for the resource holder independently of this router's verification role; a cache supplying verified records does not register the router operator's ASPA on its behalf. | An AS MUST list in its SPAS the union of all its Provider AS(es) and non-transparent RS AS(es) at which it is an RS-client. |
| `4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 4 says: "An AS MUST include a Provider AS in its SPAS regardless of whether it provides connectivity for only IPv4 or only IPv6 or both." The resource holder's ASPA registration service chooses that SPAS. internal/component/bgp/plugins/rpki/aspa_cache.go::ApplyDelta receives a provider set with no address-family field, and internal/component/bgp/plugins/rpki/aspa_cache.go::checkPair reads it. Receiving the union does not perform the independent resource holder's duty to publish it, and no layer used by these router-verification producers selects or registers that holder's providers. | An AS MUST include a Provider AS in its SPAS regardless of whether it provides connectivity for only IPv4 or only IPv6 or both. |
| `4:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 4 says: "In the Complex relationship case (Section 3 and [RFC9234]), an AS MUST include the neighbor AS in its SPAS if the neighbor plays the Provider role for all or a subset of received or sent prefixes." The resource holder's ASPA registration service publishes those provider relationships. internal/component/bgp/plugins/rpki/aspa_cache.go::ApplyDelta stores received sets and internal/component/bgp/plugins/rpki/rpki_config.go::configuredASPAMode selects the router's verification algorithm; neither publishes ASPAs. A Complex verification relationship does not turn those consumers, or the cache supplying them, into the resource holder's registrar. | In the Complex relationship case (Section 3 and [RFC9234]), an AS MUST include the neighbor AS in its SPAS if the neighbor plays the Provider role for all or a subset of received or sent prefixes. |
| `4:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 4 says: "The ASes on the boundary of an AS Confederation MUST register ASPAs using the Confederation's global AS number (ASN) as the CAS." The Confederation resource holder's ASPA registration service chooses the CAS of the published object. internal/component/bgp/plugins/rpki/aspa_cache.go::ApplyDelta indexes the CAS supplied through RTR and does not register it. Operating a boundary router does not make its verification consumer or its supplying cache the registrant for that resource holder. | The ASes on the boundary of an AS Confederation MUST register ASPAs using the Confederation's global AS number (ASN) as the CAS. |
| `5.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence explicitly attributes the obligation elsewhere: "[RFC9774] specifies that "treat-as-withdraw" error handling [RFC7606] MUST be applied to any route containing an AS_SET in the AS_PATH." This attribution does not exclude ASPA's own AS_SET Invalid result: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2 retains that distinct algorithm obligation from sections 5.5 and 5.6. The conditional requirement that implemented checks run before ASPA is separately mapped at 5.1:2. | [RFC9774] specifies that "treat-as-withdraw" error handling [RFC7606] MUST be applied to any route containing an AS_SET in the AS_PATH. |
| `6.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 6.5 says: "The AS operator MUST notify its customer ASes and advise them to update ASPA records to include both the globally configured ASN and the legacy ASN in their SPAS." The migrating AS operator's out-of-band customer notification process performs this communication. internal/component/bgp/plugins/rpki/aspa_cache.go::ApplyDelta receives provider sets and internal/component/bgp/plugins/rpki/rpki.go::handleStructuredUpdate verifies received routes; neither contacts the operator's customers or changes their registrations. A cache delivering verified ASPAs does not fulfil this human operational notification on the router's behalf. | The AS operator MUST notify its customer ASes and advise them to update ASPA records to include both the globally configured ASN and the legacy ASN in their SPAS. |

## Superseded

No document obsoletes DRAFT-IETF-SIDROPS-ASPA-VERIFICATION, so its obligations are stated where they were written.
