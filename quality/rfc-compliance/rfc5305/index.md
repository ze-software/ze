# RFC 5305 - IS-IS Extensions for Traffic Engineering

Experimental. Every requirement this repository extracted from RFC 5305, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.0% | 4 of 5 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 5 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 10 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.0% | 1 of 5 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 5 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 8 |
| Obligations that bind Ze | 5 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tagged units | 10 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5305.md` |
| Requirement shard | `rfc/requirements/rfc5305.md` |
| RFC text | `rfc/full/rfc5305.txt` |

## Enrolment

Enrolled: IS-IS Extensions for Traffic Engineering (TLV 22 extended IS reachability, TLV 135 extended IP reachability): eight MUST-level requirements. Four are met with positive+negative tags in internal/plugins/isis: 3-2 (SHALL clamp/saturate the wide path metric), 4-1 (do not install a route at or above MaxPathMetric), 4.1-1 (set the up/down bit correctly on a down-leak), 2-1 (retain unknown sub-TLVs without rejecting). 3.2-1, 3.2-2, 4.3-1 are {not-applicable}: ze implements no IS-IS TE sub-TLVs (6/8) or TLV 134. 3-1 (a maximum-metric link must not be used in normal SPF) is {gap}: ze relaxes a 2^24-1-metric link into SPF, excluding only metrics at or above MaxPathMetric (0xFE000000). Disclosed in the docs/features/rfc-status.md RFC 5305 row.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

- Extended IS reachability (TLV 22) and extended IPv4 reachability (TLV 135) with wide metrics and the up/down bit
- unknown sub-TLVs retained
- tests bound per requirement in [`rfc/requirements/rfc5305.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5305.md).


**What the ledger says remains**

[`RFC5305-3-1`](#rfc5305-3-1) gap: a link advertised with the maximum link metric (2^24-1 = 16777215) is still used in normal SPF (spf/graph.go builds the edge with the raw metric and spf/spf.go excludes only metrics at or above MaxPathMetric 0xFE000000), so the Section 3 maximum-link-metric exclusion is not implemented. TE sub-TLVs (6/8) and TLV 134 (TE Router ID) are not implemented (no IS-IS TE).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC5305-3-2`](#rfc5305-3-2), [`RFC5305-4-1`](#rfc5305-4-1), [`RFC5305-4.1-1`](#rfc5305-4.1-1), [`RFC5305-2-1`](#rfc5305-2-1)

**Annotated instead of tested (4):** [`RFC5305-3-1`](#rfc5305-3-1), [`RFC5305-3.2-1`](#rfc5305-3.2-1), [`RFC5305-3.2-2`](#rfc5305-3.2-2), [`RFC5305-4.3-1`](#rfc5305-4.3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5305-3-1` | Use a TLV 22 link advertised with metric 2^24 minus 1 in normal SPF (Section 3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze relaxes a maximum-metric (16777215) link into normal SPF -- internal/plugins/isis/spf/graph.go:192-194 builds the edge with the raw metric and spf/spf.go:208-211 excludes only metrics >= MaxPathMetric (0xFE000000), so the RFC 5305 Section 3 maximum-link-metric exclusion is not implemented |
| `RFC5305-3-2` | Clamp metrics at or above MAX_PATH_METRIC (0xFE000000) to MAX_PATH_METRIC (Section 3, Section 3.7) | SHALL | 3 | **positive:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L250). **negative:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L251) |
| `RFC5305-4-1` | Consider a TLV 135 prefix with metric above MAX_PATH_METRIC in normal SPF (Section 4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L293). **negative:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L298) |
| `RFC5305-4.1-1` | Set the TLV 135 up/down bit to 1 when advertising down the hierarchy or across same-level areas (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L255). **negative:** `unit/verify` [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L266). **negative:** `unit/verify` [`TestISISRedistConsumerConnected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/consumer_test.go#L169) |
| `RFC5305-3.2-1` | Inject sub-TLV 6 / sub-TLV 8 / Router-ID addresses as /32 routes (Section 3.2, Section 3.3, Section 4.3) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never decodes the RFC 5305 TE sub-TLVs (sub-TLV 6/8) or TLV 134 into an address; internal/plugins/isis/spf/graph.go:192-194 reads only edges and drops sub-TLVs, and route.go:155 installs only node.Prefixes, so there is no per-link /32 injection code path |
| `RFC5305-3.2-2` | Include sub-TLV 6 (and sub-TLV 8 on point-to-point) when implementing TE (Section 3.2, Section 3.3) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation is conditional on implementing IS-IS TE; ze originates no TLV 22 TE sub-TLVs (internal/plugins/isis/lsdb/encode.go:101-107 writes a zero sub-TLV length), so it does not implement the TE metric it would govern |
| `RFC5305-4.3-1` | Include the TE Router ID TLV (134) when implementing TE (Section 4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation is conditional on implementing IS-IS TE; TLV 134 (TE Router ID) is absent from ze's IS-IS codec type set (internal/plugins/isis/packet/tlv.go:17-32), so ze originates and consumes no TLV 134 |
| `RFC5305-2-1` | Ignore and skip unknown sub-TLVs on receipt (Section 2) | MUST | 2 | **positive:** `unit/verify` [`TestISISTLV22RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_core_test.go#L185). **positive:** `unit/verify` [`TestISISTLVIPv4RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L85). **negative:** `unit/verify` [`TestISISTLV22Truncated`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_core_test.go#L205) |
| `RFC5305-3.1-1` | Include each optional sub-TLV (3, 9, 10, 11, 18) at most once per TLV 22 (Section 3.1, Section 3.4 through Section 3.7) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-4.3-2` | Include the TE Router ID TLV more than once per LSP (Section 4.3) | SHOULD NOT | 4.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5305-3-1`](#rfc5305-3-1) Use a TLV 22 link advertised with metric 2^24 minus 1 in normal SPF (Section 3) | {gap}, no test | ze relaxes a maximum-metric (16777215) link into normal SPF -- internal/plugins/isis/spf/graph.go:192-194 builds the edge with the raw metric and spf/spf.go:208-211 excludes only metrics >= MaxPathMetric (0xFE000000), so the RFC 5305 Section 3 maximum-link-metric exclusion is not implemented |
| [`RFC5305-3.2-1`](#rfc5305-3.2-1) Inject sub-TLV 6 / sub-TLV 8 / Router-ID addresses as /32 routes (Section 3.2, Section 3.3, Section 4.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze never decodes the RFC 5305 TE sub-TLVs (sub-TLV 6/8) or TLV 134 into an address; internal/plugins/isis/spf/graph.go:192-194 reads only edges and drops sub-TLVs, and route.go:155 installs only node.Prefixes, so there is no per-link /32 injection code path |
| [`RFC5305-3.2-2`](#rfc5305-3.2-2) Include sub-TLV 6 (and sub-TLV 8 on point-to-point) when implementing TE (Section 3.2, Section 3.3) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation is conditional on implementing IS-IS TE; ze originates no TLV 22 TE sub-TLVs (internal/plugins/isis/lsdb/encode.go:101-107 writes a zero sub-TLV length), so it does not implement the TE metric it would govern |
| [`RFC5305-4.3-1`](#rfc5305-4.3-1) Include the TE Router ID TLV (134) when implementing TE (Section 4.3) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation is conditional on implementing IS-IS TE; TLV 134 (TE Router ID) is absent from ze's IS-IS codec type set (internal/plugins/isis/packet/tlv.go:17-32), so ze originates and consumes no TLV 134 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5305-3-1`](#rfc5305-3-1)

Use a TLV 22 link advertised with metric 2^24 minus 1 in normal SPF (Section 3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-3-1, so no unit is bound to it.

### [`RFC5305-3-2`](#rfc5305-3-2)

Clamp metrics at or above MAX_PATH_METRIC (0xFE000000) to MAX_PATH_METRIC (Section 3, Section 3.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L251) | unit/verify | unproven |
| positive | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L250) | unit/verify | unproven |

### [`RFC5305-4-1`](#rfc5305-4-1)

Consider a TLV 135 prefix with metric above MAX_PATH_METRIC in normal SPF (Section 4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L298) | unit/verify | unproven |
| positive | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L293) | unit/verify | unproven |

### [`RFC5305-4.1-1`](#rfc5305-4.1-1)

Set the TLV 135 up/down bit to 1 when advertising down the hierarchy or across same-level areas (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L266) | unit/verify | unproven |
| negative | [`TestISISRedistConsumerConnected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/consumer_test.go#L169) | unit/verify | unproven |
| positive | [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L255) | unit/verify | unproven |

### [`RFC5305-3.2-1`](#rfc5305-3.2-1)

Inject sub-TLV 6 / sub-TLV 8 / Router-ID addresses as /32 routes (Section 3.2, Section 3.3, Section 4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-3.2-1, so no unit is bound to it.

### [`RFC5305-3.2-2`](#rfc5305-3.2-2)

Include sub-TLV 6 (and sub-TLV 8 on point-to-point) when implementing TE (Section 3.2, Section 3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-3.2-2, so no unit is bound to it.

### [`RFC5305-4.3-1`](#rfc5305-4.3-1)

Include the TE Router ID TLV (134) when implementing TE (Section 4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-4.3-1, so no unit is bound to it.

### [`RFC5305-2-1`](#rfc5305-2-1)

Ignore and skip unknown sub-TLVs on receipt (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISTLV22Truncated`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_core_test.go#L205) | unit/verify | unproven |
| positive | [`TestISISTLV22RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_core_test.go#L185) | unit/verify | unproven |
| positive | [`TestISISTLVIPv4RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L85) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5305, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5305, so its obligations are stated where they were written.
