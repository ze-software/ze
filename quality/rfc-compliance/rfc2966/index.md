# RFC 2966 - Domain-wide Prefix Distribution with Two-Level IS-IS

Experimental. Every requirement this repository extracted from RFC 2966, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 3 of 3 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 3 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 3 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 6 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 3 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2966.md` |
| Requirement shard | `rfc/requirements/rfc2966.md` |
| RFC text | `rfc/full/rfc2966.txt` |

## Enrolment

Enrolled: Domain-wide Prefix Distribution with Two-Level IS-IS (the RFC 2966 up/down bit): four MUST-level requirements over the pure inter-level leak producer LeakPrefixes/leakInto (internal/plugins/isis/spf/leak.go). RFC2966-2-1 (up/down bit set for L2->L1 prefixes, clear otherwise) both polarities via TestISISLeakOriginationL1L2: an L2-derived prefix leaks DOWN into L1 with up/down=true, an L1-native prefix leaks UP into L2 with up/down=false. RFC2966-2-2 (MUST NOT re-advertise up/down-set L1-learned prefixes back into L2) both polarities via TestISISLeakOriginationL1L2: a prefix already carrying the down bit is skipped (leakInto skips p.UpDown) while a clear-bit L1 prefix is still leaked. RFC2966-2-3 (L1L2 routers never advertise L2->L1 routes back into L2) both polarities via TestISISLeakFixpoint: a re-originated down-bit prefix is not leaked back up, while the same prefix without the down bit does leak down. RFC2966-x-1 (ignore internal-reachability-with-external-metric-type on receipt) is {not-applicable}: Ze does not decode the old narrow TLV 128/130 (only the wide TLV 135/236), so the internal/external-metric-type conflict never arises. No SHOULD/MAY requirements are gated.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Up/down bit retention and redistribution behavior.

**What the ledger says remains:**

Same IS-IS experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC2966-2-1`](#rfc2966-2-1), [`RFC2966-2-2`](#rfc2966-2-2), [`RFC2966-2-3`](#rfc2966-2-3)

**Annotated instead of tested (1):** [`RFC2966-x-1`](#rfc2966-x-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2966-2-1` | Set the up/down bit to one for L2-derived prefixes advertised into L1 LSPs, zero otherwise (Section 2) | MUST | 2 | **positive:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L52). **negative:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L56) |
| `RFC2966-2-2` | Never advertise up/down-bit-set, L1-learned prefixes back into L2 (Section 2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L59). **negative:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L62) |
| `RFC2966-2-3` | L1L2 routers never advertise L2->L1 inter-area routes learned via L1 routing back into L2 (Section 2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L129). **negative:** `unit/verify` [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L133) |
| `RFC2966-x-1` | Ignore a prefix combining "IP Internal Reachability Information" with external metric-type on receipt (Sections 3.1, 3.3) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** This requirement concerns the OLD narrow-metric TLV 128 (IP Internal Reachability Information), whose per-prefix external-metric-type bit could conflict with internal reachability. Ze does not decode TLV 128 or TLV 130 (old narrow IP reachability) at all -- its codec recognizes only the wide-metric TLV 135 / TLV 236 (Extended IP/IPv6 Reachability, RFC 5305/5308), and TLV 135 has no internal/external-metric-type octet (internal/plugins/isis/packet/tlv.go recognized-type set: 1,2,6,8,9,10,22,129,132,135,137,232,236,240). A received TLV 128 is an unrecognized TLV, retained opaquely for re-flood but never interpreted for routing, so the internal-reachability-with-external-metric prefix RFC 2966 warns against is never acted upon in Ze. |
| `RFC2966-3.3-1` | Ignore the up/down bit in L2 LSPs and accept the prefixes regardless of its setting (Section 3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2966-x-2` | Default configuration does not advertise L2 routes into L1; require manual configuration to do so (Sections 3.3, 4) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2966-x-1`](#rfc2966-x-1) Ignore a prefix combining "IP Internal Reachability Information" with external metric-type on receipt (Sections 3.1, 3.3) | no test | no test carries this requirement id; annotated {not-applicable}: This requirement concerns the OLD narrow-metric TLV 128 (IP Internal Reachability Information), whose per-prefix external-metric-type bit could conflict with internal reachability. Ze does not decode TLV 128 or TLV 130 (old narrow IP reachability) at all -- its codec recognizes only the wide-metric TLV 135 / TLV 236 (Extended IP/IPv6 Reachability, RFC 5305/5308), and TLV 135 has no internal/external-metric-type octet (internal/plugins/isis/packet/tlv.go recognized-type set: 1,2,6,8,9,10,22,129,132,135,137,232,236,240). A received TLV 128 is an unrecognized TLV, retained opaquely for re-flood but never interpreted for routing, so the internal-reachability-with-external-metric prefix RFC 2966 warns against is never acted upon in Ze. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2966-2-1`](#rfc2966-2-1)

Set the up/down bit to one for L2-derived prefixes advertised into L1 LSPs, zero otherwise (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L56) | unit/verify | unproven |
| positive | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L52) | unit/verify | unproven |

### [`RFC2966-2-2`](#rfc2966-2-2)

Never advertise up/down-bit-set, L1-learned prefixes back into L2 (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L62) | unit/verify | unproven |
| positive | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L59) | unit/verify | unproven |

### [`RFC2966-2-3`](#rfc2966-2-3)

L1L2 routers never advertise L2->L1 inter-area routes learned via L1 routing back into L2 (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L133) | unit/verify | unproven |
| positive | [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L129) | unit/verify | unproven |

### [`RFC2966-x-1`](#rfc2966-x-1)

Ignore a prefix combining "IP Internal Reachability Information" with external metric-type on receipt (Sections 3.1, 3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2966-x-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 2966, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2966, so its obligations are stated where they were written.
