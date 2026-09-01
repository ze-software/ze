# DRAFT-IETF-SIDROPS-ASPA-VERIFICATION - Verification of AS_PATH Using the Resource Certificate PKI and Autonomous System Provider Authorization

Partial. Every requirement this repository extracted from DRAFT-IETF-SIDROPS-ASPA-VERIFICATION, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 4 of 8 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 25.0% | 2 of 8 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 11 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 25.0% | 2 of 8 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 8 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 8 |
| Obligations that bind Ze | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 11 |
| Tagged units | 11 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/draft-ietf-sidrops-aspa-verification.md` |
| Requirement shard | `rfc/requirements/draft-ietf-sidrops-aspa-verification.md` |
| RFC text | `rfc/drafts/draft-ietf-sidrops-aspa-verification.txt` |

## Enrolment

Enrolled: ASPA AS_PATH verification: 4 MET + 2 single-polarity (6-1, 7-2) + 2 gap (6-4 per-AFI records, 8-1 Invalid-not-preferred)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Section 6 verification algorithm (upstream/downstream), AS_SET to Unknown, prepend collapse, AS0-in-provider rejection, RTR ASPA PDU (Type 11) consumption, and re-validation on cache change. Two MUSTs unmet: per-AFI ASPA records (6-4, the AFI flag is parsed then discarded and the cache is keyed by customer AS alone, so per-AFI records overwrite each other)
- Invalid-not-preferred (8-1, ASPA state drives only reject/keep with default LogOnly, so an accepted Invalid route can outrank a Valid one for the same prefix).


**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-2`](#draft-ietf-sidrops-aspa-verification-6-2), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-3`](#draft-ietf-sidrops-aspa-verification-6-3), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-x-1`](#draft-ietf-sidrops-aspa-verification-x-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-1`](#draft-ietf-sidrops-aspa-verification-7-1)

**Annotated instead of tested (4):** [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-1`](#draft-ietf-sidrops-aspa-verification-6-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-4`](#draft-ietf-sidrops-aspa-verification-6-4), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-1`](#draft-ietf-sidrops-aspa-verification-8-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-2`](#draft-ietf-sidrops-aspa-verification-7-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-1` | Apply upstream verification to routes received from customers and lateral peers (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestASPAStateForPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L223). **negative:** no negative test. **{single-polarity}:** verifyASPA runs on every received UPDATE carrying an AS_PATH whenever ASPA is enabled (a superset that includes customer and peer routes), and there is no required case where such a route must NOT be verified (internal/component/bgp/plugins/rpki/rpki.go:338) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-2` | AS_SET in path must result in Unknown validation state (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestASPAStateForPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L245). **positive:** `unit/verify` [`TestASPAVerifyASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L62). **negative:** `unit/verify` [`TestASPAVerifyValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L16) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-3` | Prepend removal must only collapse consecutive duplicates (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestASPANormalizePrepends`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L100). **negative:** `unit/verify` [`TestASPANormalizePrepends`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L102) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-4` | Support per-AFI ASPA records if provided by cache (Section 6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the ASPA PDU AFI-flags byte is read only to reject unknown AFI and is then discarded; ASPARecord carries no AFI field and the cache is keyed by customer AS alone, so per-AFI records for one customer overwrite each other and one AS_PATH state is applied to both IPv4 and IPv6 NLRIs (internal/component/bgp/plugins/rpki/aspa_cache.go:10-13, rpki.go:344) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-x-1` | AS0 in ASPA provider set must be ignored (Pitfalls) | MUST | x | **positive:** `unit/verify` [`TestParseASPAPDUReservedProviderAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L384). **negative:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L228) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-1` | Invalid routes must not be preferred over Valid or Unknown routes (Section 8) | MUST NOT | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ASPA state drives only a binary reject/keep decision with no local-pref demotion or best-path tiebreak, and the default Invalid action is LogOnly (retain), so an accepted ASPA-Invalid route competes on ordinary BGP attributes and can outrank an ASPA-Valid route for the same prefix (internal/component/bgp/plugins/rpki/rpki.go:92-100, rpki_config.go:110) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-1` | Re-run verification when ASPA data changes (Section 7) | MUST | 7 | **positive:** `unit/verify` [`TestASPATrackerRevalidate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_tracker_test.go#L54). **negative:** `unit/verify` [`TestASPATrackerRevalidateNoChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_tracker_test.go#L117) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-2` | Use the most recent ASPA data available (Section 7) | MUST | 7 | **positive:** `unit/verify` [`TestASPAApplyDeltaMostRecent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_cache_test.go#L171). **negative:** no negative test. **{single-polarity}:** ApplyDelta atomically replaces cache entries at each End of Data and verifyASPA reads the live cache under lock, so verification always reflects the applied delta; there is no stale-data mode to test negatively (internal/component/bgp/plugins/rpki/aspa_cache.go:110-129) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-2` | Reject Invalid routes by default (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-3` | Accept Unknown routes as unverified (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-4` | Log Invalid results for operational visibility (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-5` | Assign local-pref based on ASPA validation state (Section 8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-5` | Skip verification for routes from upstream providers (Section 6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-6` | Apply verification to IBGP-learned routes (Section 6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-4`](#draft-ietf-sidrops-aspa-verification-6-4) Support per-AFI ASPA records if provided by cache (Section 6) | {gap}, no test | the ASPA PDU AFI-flags byte is read only to reject unknown AFI and is then discarded; ASPARecord carries no AFI field and the cache is keyed by customer AS alone, so per-AFI records for one customer overwrite each other and one AS_PATH state is applied to both IPv4 and IPv6 NLRIs (internal/component/bgp/plugins/rpki/aspa_cache.go:10-13, rpki.go:344) |
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-1`](#draft-ietf-sidrops-aspa-verification-8-1) Invalid routes must not be preferred over Valid or Unknown routes (Section 8) | {gap}, no test | ASPA state drives only a binary reject/keep decision with no local-pref demotion or best-path tiebreak, and the default Invalid action is LogOnly (retain), so an accepted ASPA-Invalid route competes on ordinary BGP attributes and can outrank an ASPA-Valid route for the same prefix (internal/component/bgp/plugins/rpki/rpki.go:92-100, rpki_config.go:110) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-1`](#draft-ietf-sidrops-aspa-verification-6-1)

Apply upstream verification to routes received from customers and lateral peers (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestASPAStateForPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L223) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-2`](#draft-ietf-sidrops-aspa-verification-6-2)

AS_SET in path must result in Unknown validation state (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPAVerifyValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L16) | unit/verify | unproven |
| positive | [`TestASPAStateForPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L245) | unit/verify | unproven |
| positive | [`TestASPAVerifyASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L62) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-3`](#draft-ietf-sidrops-aspa-verification-6-3)

Prepend removal must only collapse consecutive duplicates (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPANormalizePrepends`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L102) | unit/verify | unproven |
| positive | [`TestASPANormalizePrepends`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_verify_test.go#L100) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-4`](#draft-ietf-sidrops-aspa-verification-6-4)

Support per-AFI ASPA records if provided by cache (Section 6)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-4, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-x-1`](#draft-ietf-sidrops-aspa-verification-x-1)

AS0 in ASPA provider set must be ignored (Pitfalls)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L228) | unit/verify | unproven |
| positive | [`TestParseASPAPDUReservedProviderAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L384) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-1`](#draft-ietf-sidrops-aspa-verification-8-1)

Invalid routes must not be preferred over Valid or Unknown routes (Section 8)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-1, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-1`](#draft-ietf-sidrops-aspa-verification-7-1)

Re-run verification when ASPA data changes (Section 7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPATrackerRevalidateNoChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_tracker_test.go#L117) | unit/verify | unproven |
| positive | [`TestASPATrackerRevalidate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_tracker_test.go#L54) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-2`](#draft-ietf-sidrops-aspa-verification-7-2)

Use the most recent ASPA data available (Section 7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestASPAApplyDeltaMostRecent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/aspa_cache_test.go#L171) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for DRAFT-IETF-SIDROPS-ASPA-VERIFICATION, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes DRAFT-IETF-SIDROPS-ASPA-VERIFICATION, so its obligations are stated where they were written.
