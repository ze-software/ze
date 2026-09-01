# RFC 5308 - Routing IPv6 with IS-IS

Experimental. Every requirement this repository extracted from RFC 5308, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 7 of 7 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 7 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 7 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 7 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 7 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 7 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 7 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 8 |
| Gated MUST-level | 7 |
| Obligations that bind Ze | 7 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5308.md` |
| Requirement shard | `rfc/requirements/rfc5308.md` |
| RFC text | `rfc/full/rfc5308.txt` |

## Enrolment

Enrolled: Routing IPv6 with IS-IS: seven MUST-level requirements, all met and test-bound with positive+negative tags. 2-1 (no link-local in IPv6 Reachability TLV 236) and 3-2 (no link-local in the LSP IPv6 address set) via internal/plugins/isis/lsdb origination tests; 3-1 (IPv6 Interface Address TLV 232 only for link-local in Hellos) and 4-1 (advertise IPv6 NLPID 0x8E in Protocols Supported) via internal/plugins/isis/circuit tests; 2-2 (do not route a metric above MaxV6PathMetric), 5-1 (up/down and level-aware preference), and 5-2 (clamp the path metric) via internal/plugins/isis/spf tests. Single-topology IS-IS: IPv6 rides the shared per-level SPF, so 5-1/5-2 reuse the IPv4 producers exercised for IPv6 via BuildRoutesV6.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

IPv6 reachability over the same IS-IS instance.

**What the ledger says remains:**

Same IS-IS experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC5308-2-1`](#rfc5308-2-1), [`RFC5308-2-2`](#rfc5308-2-2), [`RFC5308-3-1`](#rfc5308-3-1), [`RFC5308-3-2`](#rfc5308-3-2), [`RFC5308-4-1`](#rfc5308-4-1), [`RFC5308-5-1`](#rfc5308-5-1), [`RFC5308-5-2`](#rfc5308-5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5308-2-1` | Advertise link-local prefixes in TLV 236 (Section 2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L65). **negative:** `unit/verify` [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L66) |
| `RFC5308-2-2` | Consider a TLV 236 prefix with metric above MAX_V6_PATH_METRIC (0xFE000000) in normal SPF (Section 2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/ipv6_test.go#L207). **negative:** `unit/verify` [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/ipv6_test.go#L206) |
| `RFC5308-3-1` | Carry only link-local IPv6 addresses in TLV 232 in Hellos (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_ipv6_test.go#L29). **negative:** `unit/verify` [`TestISISIIHTLV232OmittedNoLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_ipv6_test.go#L102). **negative:** `unit/verify` [`TestISISIIHTLV232RejectsNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_ipv6_test.go#L129) |
| `RFC5308-3-2` | Carry only non-link-local IPv6 addresses in TLV 232 in LSPs (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L110). **negative:** `unit/verify` [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L111) |
| `RFC5308-4-1` | Advertise the IPv6 NLPID (142, 0x8E) in the NLPID TLV when supporting IPv6 (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_ipv6_test.go#L30). **positive:** `unit/verify` [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L148). **negative:** `unit/verify` [`TestISISIIHNoTLV232WhenIPv4Only`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_ipv6_test.go#L77). **negative:** `unit/verify` [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L149) |
| `RFC5308-5-1` | Apply the up/down-aware path preference order (Level 1 up, Level 2 up, Level 2 down, Level 1 down) (Section 5) | MUST | 5 | **positive:** `unit/verify` [`TestISISIPv6LevelArbitration`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/ipv6_test.go#L91). **positive:** `unit/verify` [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_test.go#L46). **negative:** `unit/verify` [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_test.go#L47) |
| `RFC5308-5-2` | Clamp an SPF path metric that would exceed MAX_V6_PATH_METRIC to MAX_V6_PATH_METRIC (Section 5) | SHALL | 5 | **positive:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L248). **negative:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L249) |
| `RFC5308-5-3` | Consider equal-best paths for equal-cost multi-path routing where supported (Section 5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5308 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5308-2-1`](#rfc5308-2-1)

Advertise link-local prefixes in TLV 236 (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L66) | unit/verify | unproven |
| positive | [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L65) | unit/verify | unproven |

### [`RFC5308-2-2`](#rfc5308-2-2)

Consider a TLV 236 prefix with metric above MAX_V6_PATH_METRIC (0xFE000000) in normal SPF (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/ipv6_test.go#L206) | unit/verify | unproven |
| positive | [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/ipv6_test.go#L207) | unit/verify | unproven |

### [`RFC5308-3-1`](#rfc5308-3-1)

Carry only link-local IPv6 addresses in TLV 232 in Hellos (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISIIHTLV232OmittedNoLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_ipv6_test.go#L102) | unit/verify | unproven |
| negative | [`TestISISIIHTLV232RejectsNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_ipv6_test.go#L129) | unit/verify | unproven |
| positive | [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_ipv6_test.go#L29) | unit/verify | unproven |

### [`RFC5308-3-2`](#rfc5308-3-2)

Carry only non-link-local IPv6 addresses in TLV 232 in LSPs (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L111) | unit/verify | unproven |
| positive | [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L110) | unit/verify | unproven |

### [`RFC5308-4-1`](#rfc5308-4-1)

Advertise the IPv6 NLPID (142, 0x8E) in the NLPID TLV when supporting IPv6 (Section 4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISIIHNoTLV232WhenIPv4Only`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_ipv6_test.go#L77) | unit/verify | unproven |
| negative | [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L149) | unit/verify | unproven |
| positive | [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_ipv6_test.go#L30) | unit/verify | unproven |
| positive | [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_ipv6_test.go#L148) | unit/verify | unproven |

### [`RFC5308-5-1`](#rfc5308-5-1)

Apply the up/down-aware path preference order (Level 1 up, Level 2 up, Level 2 down, Level 1 down) (Section 5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_test.go#L47) | unit/verify | unproven |
| positive | [`TestISISIPv6LevelArbitration`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/ipv6_test.go#L91) | unit/verify | unproven |
| positive | [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_test.go#L46) | unit/verify | unproven |

### [`RFC5308-5-2`](#rfc5308-5-2)

Clamp an SPF path metric that would exceed MAX_V6_PATH_METRIC to MAX_V6_PATH_METRIC (Section 5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L249) | unit/verify | unproven |
| positive | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L248) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5308, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5308, so its obligations are stated where they were written.
