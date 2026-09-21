# DRAFT-IETF-SIDROPS-ASPA-VERIFICATION - Verification of AS_PATH Using the Resource Certificate PKI and Autonomous System Provider Authorization

Partial. Every requirement this repository extracted from DRAFT-IETF-SIDROPS-ASPA-VERIFICATION, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.5% | 1 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 19 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 87.5% | 7 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
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
| Requirements | 19 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 7 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/draft-ietf-sidrops-aspa-verification.md` |
| Requirement shard | `rfc/requirements/draft-ietf-sidrops-aspa-verification.md` |
| RFC text | `rfc/drafts/draft-ietf-sidrops-aspa-verification.txt` |

## Enrolment

Enrolled: ASPA AS_PATH verification: 4 tested rows (4-1, 5.1-1, 5.4-2, 5.6-1) + 1 single-polarity positive (5.4-1 upstream application) + 1 gap (5.6-2 Invalid route not made ineligible for route selection) + 3 untested MUST rows added by the 2026-09-21 extraction walk (5-1 treat-as-withdraw on neighbor mismatch, 6.2-1 and 6.2-2 AFI/SAFI scope). That walk also re-anchored every id to the section it cites: the rows had been written against an earlier draft revision, so they named sections 6, 7 and 8, which in draft-24 are Deployment Recommendations, Security Considerations and Relation to Other Technologies. The section 4 registration MUSTs and the section 6.5 operator-notification MUST bind the ASPA registrant and are excluded in rfc/extraction/.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Section 5 verification algorithm (upstream/downstream), prepend compression, AS0-in-provider rejection, RTR ASPA PDU (Type 11) consumption, and re-validation on cache change. Three defects against draft-24 remain. (1) AS_SET: sections 5.4 and 5.5 step 3 halt with "Invalid"
- ze returns Unknown, and three tagged tests in [`internal/component/bgp/plugins/rpki/aspa_verify_test.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go) assert the Unknown outcome (row 5.4-2). (2) Row 5.6-2: the default Invalid action is LogOnly, so an ASPA-Invalid route is never made ineligible for route selection. (3) Rows 5-1, 6.2-1 and 6.2-2 carry no test.


**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 7 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (1):** [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1`](#draft-ietf-sidrops-aspa-verification-5.4-1)

**No test and no annotation (7):** [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1`](#draft-ietf-sidrops-aspa-verification-4-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5-1`](#draft-ietf-sidrops-aspa-verification-5-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1`](#draft-ietf-sidrops-aspa-verification-5.1-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2`](#draft-ietf-sidrops-aspa-verification-5.4-2), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1`](#draft-ietf-sidrops-aspa-verification-5.6-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1`](#draft-ietf-sidrops-aspa-verification-6.2-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2`](#draft-ietf-sidrops-aspa-verification-6.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1` | An unexpected presence of AS 0 in a SPAS has no influence on the AS_PATH verification procedures (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5-1` | If the check matching the most recently added AS in the AS_PATH to the BGP neighbor's ASN fails, then the AS_PATH is considered semantically invalid, and the UPDATE SHALL be handled using the approach of "treat-as-withdraw" [RFC7606] (Section 5) | SHALL | 5 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1` | The COMPRESSED_AS_PATH is the AS_PATH after removing consecutive duplicate ASNs (Section 5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1` | The upstream verification algorithm is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS (Section 5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{single-polarity}:** verifyASPA runs on every received UPDATE carrying an AS_PATH whenever ASPA is enabled (a superset that includes customer and peer routes), and there is no required case where such a route must NOT be verified (internal/component/bgp/plugins/rpki/rpki.go:338) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2` | If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid" (Section 5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1` | A route whose AS_PATH is determined to be Invalid MUST be kept in the Adj-RIB-In for potential future re-evaluation (Section 5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1` | The verification procedures described in this document MUST be applied to BGP routes with {AFI 1 (IPv4), SAFI 1} and {AFI 2 (IPv6), SAFI 1} (Section 6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2` | The procedures MUST NOT be applied to other address families by default (Section 6.2) | MUST NOT | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5-2` | The check matching the most recently added AS in the AS_PATH to the BGP neighbor's ASN SHOULD be performed as specified in Section 6.3 of [RFC4271] with the exception when a route is received from a transparent Internet Exchange (IX) (Section 5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-2` | If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (Section 5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the default Invalid action is LogOnly (retain) and ASPA state drives only a binary reject/keep decision, so an ASPA-Invalid route that is retained stays eligible for best-path selection (internal/component/bgp/plugins/rpki/rpki.go:92-100, rpki_config.go:110) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-3` | When a route is evaluated as Unknown, it SHOULD be treated at the same preference level as a route evaluated as Valid (Section 5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-4` | The specific configuration of a mitigation policy based on AS_PATH verification using ASPA is at the discretion of the network operator; however, the mitigation policy of Section 5.6 is RECOMMENDED (Section 5.6) | RECOMMENDED | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-3` | The procedures are NOT RECOMMENDED for use on internal BGP (iBGP) sessions or eBGP sessions internal to an AS Confederation (Section 6.2) | NOT RECOMMENDED | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.3-1` | The BGP Role configuration parameter and its cross-check in the BGP OPEN message as specified in [RFC9234] are RECOMMENDED (Section 6.3) | RECOMMENDED | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.3-2` | The configured BGP Roles SHOULD be used to automate the use of the AS_PATH verification procedures, helping to distinguish whether upstream or downstream procedures should be applied (Section 6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.4-1` | If multiple eBGP sessions can segregate the Complex peering relationship into eBGP sessions with normal peering relationships, the receiving/verifying AS SHOULD select the algorithm (per Section 5.4 or Section 5.5) for each of the normal sessions based on its peering relation type (Section 6.4) | SHOULD | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.6-1` | For any route with an Invalid AS_PATH, the cause of the Invalid state SHOULD be logged for monitoring and diagnostic purposes (Section 6.6) | SHOULD | 6.6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8.4-1` | The implementation of the procedures utilizing the OTC Attribute is RECOMMENDED to complement the ASPA-based AS_PATH verification (Section 8.4) | RECOMMENDED | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.4-2` | If a Complex peering relation cannot be segregated and per-prefix application is not feasible, then an operator MAY apply the algorithm for downstream paths (Section 5.5) to avoid false positive outcomes (Section 6.4) | MAY | 6.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1`](#draft-ietf-sidrops-aspa-verification-4-1) An unexpected presence of AS 0 in a SPAS has no influence on the AS_PATH verification procedures (Section 4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5-1`](#draft-ietf-sidrops-aspa-verification-5-1) If the check matching the most recently added AS in the AS_PATH to the BGP neighbor's ASN fails, then the AS_PATH is considered semantically invalid, and the UPDATE SHALL be handled using the approach of "treat-as-withdraw" [RFC7606] (Section 5) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1`](#draft-ietf-sidrops-aspa-verification-5.1-1) The COMPRESSED_AS_PATH is the AS_PATH after removing consecutive duplicate ASNs (Section 5.1) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1`](#draft-ietf-sidrops-aspa-verification-5.4-1) The upstream verification algorithm is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS (Section 5.4) | no test | no test carries this requirement id; annotated {single-polarity}: verifyASPA runs on every received UPDATE carrying an AS_PATH whenever ASPA is enabled (a superset that includes customer and peer routes), and there is no required case where such a route must NOT be verified (internal/component/bgp/plugins/rpki/rpki.go:338) |
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2`](#draft-ietf-sidrops-aspa-verification-5.4-2) If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid" (Section 5.4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1`](#draft-ietf-sidrops-aspa-verification-5.6-1) A route whose AS_PATH is determined to be Invalid MUST be kept in the Adj-RIB-In for potential future re-evaluation (Section 5.6) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1`](#draft-ietf-sidrops-aspa-verification-6.2-1) The verification procedures described in this document MUST be applied to BGP routes with {AFI 1 (IPv4), SAFI 1} and {AFI 2 (IPv6), SAFI 1} (Section 6.2) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2`](#draft-ietf-sidrops-aspa-verification-6.2-2) The procedures MUST NOT be applied to other address families by default (Section 6.2) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-2`](#draft-ietf-sidrops-aspa-verification-5.6-2) If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (Section 5.6) | {gap} | the default Invalid action is LogOnly (retain) and ASPA state drives only a binary reject/keep decision, so an ASPA-Invalid route that is retained stays eligible for best-path selection (internal/component/bgp/plugins/rpki/rpki.go:92-100, rpki_config.go:110) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1`](#draft-ietf-sidrops-aspa-verification-4-1)

An unexpected presence of AS 0 in a SPAS has no influence on the AS_PATH verification procedures (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5-1`](#draft-ietf-sidrops-aspa-verification-5-1)

If the check matching the most recently added AS in the AS_PATH to the BGP neighbor's ASN fails, then the AS_PATH is considered semantically invalid, and the UPDATE SHALL be handled using the approach of "treat-as-withdraw" [RFC7606] (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5-1, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1`](#draft-ietf-sidrops-aspa-verification-5.1-1)

The COMPRESSED_AS_PATH is the AS_PATH after removing consecutive duplicate ASNs (Section 5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1`](#draft-ietf-sidrops-aspa-verification-5.4-1)

The upstream verification algorithm is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS (Section 5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2`](#draft-ietf-sidrops-aspa-verification-5.4-2)

If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid" (Section 5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1`](#draft-ietf-sidrops-aspa-verification-5.6-1)

A route whose AS_PATH is determined to be Invalid MUST be kept in the Adj-RIB-In for potential future re-evaluation (Section 5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1`](#draft-ietf-sidrops-aspa-verification-6.2-1)

The verification procedures described in this document MUST be applied to BGP routes with {AFI 1 (IPv4), SAFI 1} and {AFI 2 (IPv6), SAFI 1} (Section 6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2`](#draft-ietf-sidrops-aspa-verification-6.2-2)

The procedures MUST NOT be applied to other address families by default (Section 6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/drafts/draft-ietf-sidrops-aspa-verification.txt |
| Source fingerprint | d11e36f2253bad3f |
| Record | rfc/extraction/draft-ietf-sidrops-aspa-verification.json |
| Mapped sentences | 4 |
| Declined as scope | 7 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 5 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.4` | not stated | 0 | walked | not stated |
| `5.5` | not stated | 0 | walked | not stated |
| `5.6` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 2 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 1 | walked | not stated |
| `6.6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.4` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `11.1` | not stated | 0 | walked | not stated |
| `11.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | An AS MUST list its Provider ASes and non-transparent RS ASes in its SPAS. The obligation binds the AS that REGISTERS the ASPA object in the RPKI, not the BGP speaker that verifies an AS_PATH with it. A SPAS is a field of a signed ASPA object published by the resource holder's certification authority. Ze's entire ASPA surface is an RTR client: producer internal/component/bgp/plugins/rpki/aspa_cache.go consumes ASPA PDUs (Type 11) into a read-only cache, and internal/component/bgp/plugins/rpki/rpki.go reads that cache during verification. Ze holds no certification authority, no ASPA object signing and no RPKI publication path, so no producer in the tree can list a Provider AS in a SPAS. Producer: the RPKI ASPA issuer: the resource holder's certification authority that signs and publishes the ASPA object carrying the SPAS; ze holds no code for this role. | An AS MUST list in its SPAS the union of all its Provider AS(es) and non-transparent RS AS(es) at which it is an RS-client. |
| `4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | An AS MUST include a Provider AS in its SPAS whatever address families that provider serves. The obligation binds the AS that REGISTERS the ASPA object in the RPKI, not the BGP speaker that verifies an AS_PATH with it. A SPAS is a field of a signed ASPA object published by the resource holder's certification authority. Ze's entire ASPA surface is an RTR client: producer internal/component/bgp/plugins/rpki/aspa_cache.go consumes ASPA PDUs (Type 11) into a read-only cache, and internal/component/bgp/plugins/rpki/rpki.go reads that cache during verification. Ze holds no certification authority, no ASPA object signing and no RPKI publication path, so no producer in the tree can list a Provider AS in a SPAS. Producer: the RPKI ASPA issuer: the resource holder's certification authority that signs and publishes the ASPA object carrying the SPAS; ze holds no code for this role. | An AS MUST include a Provider AS in its SPAS regardless of whether it provides connectivity for only IPv4 or only IPv6 or both. |
| `4:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | In the Complex relationship case an AS MUST include the neighbor AS in its SPAS. The obligation binds the AS that REGISTERS the ASPA object in the RPKI, not the BGP speaker that verifies an AS_PATH with it. A SPAS is a field of a signed ASPA object published by the resource holder's certification authority. Ze's entire ASPA surface is an RTR client: producer internal/component/bgp/plugins/rpki/aspa_cache.go consumes ASPA PDUs (Type 11) into a read-only cache, and internal/component/bgp/plugins/rpki/rpki.go reads that cache during verification. Ze holds no certification authority, no ASPA object signing and no RPKI publication path, so no producer in the tree can list a Provider AS in a SPAS. Producer: the RPKI ASPA issuer: the resource holder's certification authority that signs and publishes the ASPA object carrying the SPAS; ze holds no code for this role. | In the Complex relationship case (Section 3 and [RFC9234]), an AS MUST include the neighbor AS in its SPAS if the neighbor plays the Provider role for all or a subset of received or sent prefixes. |
| `4:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The ASes on the boundary of an AS Confederation MUST register ASPAs under the Confederation's global ASN. The obligation binds the AS that REGISTERS the ASPA object in the RPKI, not the BGP speaker that verifies an AS_PATH with it. A SPAS is a field of a signed ASPA object published by the resource holder's certification authority. Ze's entire ASPA surface is an RTR client: producer internal/component/bgp/plugins/rpki/aspa_cache.go consumes ASPA PDUs (Type 11) into a read-only cache, and internal/component/bgp/plugins/rpki/rpki.go reads that cache during verification. Ze holds no certification authority, no ASPA object signing and no RPKI publication path, so no producer in the tree can list a Provider AS in a SPAS. Producer: the RPKI ASPA issuer: the resource holder's certification authority that signs and publishes the ASPA object carrying the SPAS; ze holds no code for this role. | The ASes on the boundary of an AS Confederation MUST register ASPAs using the Confederation's global AS number (ASN) as the CAS. |
| `4:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | An AS with no transit providers MUST register an AS0 ASPA. The obligation binds the AS that REGISTERS the ASPA object in the RPKI, not the BGP speaker that verifies an AS_PATH with it. A SPAS is a field of a signed ASPA object published by the resource holder's certification authority. Ze's entire ASPA surface is an RTR client: producer internal/component/bgp/plugins/rpki/aspa_cache.go consumes ASPA PDUs (Type 11) into a read-only cache, and internal/component/bgp/plugins/rpki/rpki.go reads that cache during verification. Ze holds no certification authority, no ASPA object signing and no RPKI publication path, so no producer in the tree can list a Provider AS in a SPAS. Producer: the RPKI ASPA issuer: the resource holder's certification authority that signs and publishes the ASPA object carrying the SPAS; ze holds no code for this role. | If that statement is true, then the AS MUST register an AS0 ASPA. |
| `5:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence attributes the obligation to the document it cites: "[RFC9774] specifies that 'treat-as-withdraw' error handling [RFC7606] MUST be applied to routes with AS_SET in the AS_PATH." This draft's own directive on AS_SET is the next sentence, "routes with AS_SET are given Invalid evaluation in the AS_PATH verification procedures (Section 5.4 and Section 5.5)", which is carried by DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2. | [RFC9774] specifies that "treat-as-withdraw" error handling [RFC7606] MUST be applied to routes with AS_SET in the AS_PATH. |
| `6.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | "The AS operator MUST notify its customer ASes and advise them to update ASPA records to include both the globally configured ASN and the legacy ASN in their SPAS." The role is the AS operator conducting an out-of-band notification of its customers during AS migration, and the action it demands is performed by the customers' RPKI registrations, not by any router. Ze's ASPA surface is the RTR client at internal/component/bgp/plugins/rpki/aspa_cache.go, which consumes ASPA records and neither issues them nor addresses customers; no producer in the tree could send such a notification. Producer: the AS operator's out-of-band notification of its customer ASes during AS migration; ze holds no code for this role. | The AS operator MUST notify its customer ASes and advise them to update ASPA records to include both the globally configured ASN and the legacy ASN in their SPAS. |

## Superseded

No document obsoletes DRAFT-IETF-SIDROPS-ASPA-VERIFICATION, so its obligations are stated where they were written.
