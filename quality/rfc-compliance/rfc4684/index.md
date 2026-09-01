# RFC 4684 - Constrained Route Distribution for Border Gateway Protocol/MultiProtocol Label Switching (BGP/MPLS) Internet Protocol (IP) Virtual Private Networks (VPNs)

Partial. Every requirement this repository extracted from RFC 4684, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 4 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 8 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4684.md` |
| Requirement shard | `rfc/requirements/rfc4684.md` |
| RFC text | `rfc/full/rfc4684.txt` |

## Enrolment

Enrolled: Constrained Route Distribution for BGP/MPLS VPNs (Route Target Constraint): four MUST-level requirements, all {gap}. Ze implements RTC as a DECODE-ONLY NLRI codec for display/analysis (internal/component/bgp/plugins/nlri/rtc/rtc.go DecodeNLRIHex/RunDecode) with no encode/origination path and no RT-membership distribution. RFC4684-3.2-1 (Originator/Next-hop when advertising RT membership NLRI) and RFC4684-3.2-2 (best-path/client-path advertisement selection): Ze never advertises RT membership NLRI. RFC4684-3.2-3 (consider all iBGP paths for the outbound route filter): Ze builds no ORF from RT membership. RFC4684-6-1 (bound the End-of-RIB delay for VPN route advertisement): Ze gates no VPN route advertisement on RT-membership state. Disclosed in the docs/features/rfc-status.md RFC 4684 row (Partial). The 6-2/6-3/8-1 SHOULDs and 5-1 MAY are not gated.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

RTC NLRI decode for display/analysis (`ze bgp decode`). Tests bound per requirement in [`rfc/requirements/rfc4684.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc4684.md).

**What the ledger says remains**

Decode-only: no encode/origination path and no RT-membership distribution. Four MUST gaps gated in [`rfc/short/rfc4684.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4684.md): Ze does not advertise RT membership NLRI (so no §3.2 Originator/Next-hop or best-path/client-path selection), builds no outbound route filter from RT membership (§3.2), and gates no VPN route advertisement on RT-membership End-of-RIB (§6).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (4):** [`RFC4684-3.2-1`](#rfc4684-3.2-1), [`RFC4684-3.2-2`](#rfc4684-3.2-2), [`RFC4684-3.2-3`](#rfc4684-3.2-3), [`RFC4684-6-1`](#rfc4684-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4684-3.2-1` | When advertising RT membership NLRI to a route-reflector client, the Originator attribute shall be set to the router-id of the advertiser, and the Next-hop attribute shall be set to the local address for that session (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze implements RTC (Route Target Constraint) as a DECODE-ONLY NLRI codec for display/analysis (internal/component/bgp/plugins/nlri/rtc/rtc.go DecodeNLRIHex/RunDecode; there is no encode/origination path). It never advertises RT membership NLRI, so it implements none of the Section 3.2 advertisement Originator/Next-hop procedure. Disclosed in docs/features/rfc-status.md. |
| `RFC4684-3.2-2` | When advertising RT membership NLRI to a non-client peer, if best path is from a non-client peer and an alternative client path exists, advertise the client path attributes (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not advertise RT membership NLRI at all (its RTC support is a decode-only NLRI codec, internal/component/bgp/plugins/nlri/rtc/rtc.go, with no origination path), so it implements none of the Section 3.2 best-path/client-path advertisement selection. Disclosed in docs/features/rfc-status.md. |
| `RFC4684-3.2-3` | When processing RT membership NLRIs from internal iBGP peers, consider all available iBGP paths for a given RT prefix for building the outbound route filter, not just the best path (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze builds no outbound route filter from RT membership -- its RTC support is a decode-only NLRI codec (internal/component/bgp/plugins/nlri/rtc/rtc.go) with no ORF construction or RT-membership-driven VPN route filtering. Disclosed in docs/features/rfc-status.md. |
| `RFC4684-6-1` | If delaying VPN route advertisement until End-of-RIB marker is received, MUST limit that delay to an upper bound (Section 6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not gate VPN route advertisement on RT-membership state -- it has no RTC-driven VPN route distribution (the RTC codec is decode-only, internal/component/bgp/plugins/nlri/rtc/rtc.go), so there is no such End-of-RIB delay for Ze to bound. Disclosed in docs/features/rfc-status.md. |
| `RFC4684-6-2` | Implementations SHOULD generate an End-of-RIB marker for Route Target membership (AFI=1, SAFI=132) regardless of whether graceful-restart is enabled (Section 6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4684-6-3` | A BGP speaker should generate the minimum set of BGP VPN route updates necessary to transition between the previous and current state of the route distribution graph (Section 6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4684-8-1` | Implementations SHOULD provide means to filter RT membership information (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4684-5-1` | A BGP speaker MAY participate in distribution of Route Target information without using it for VPN NLRI output route filtering (Section 5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4684-3.2-1`](#rfc4684-3.2-1) When advertising RT membership NLRI to a route-reflector client, the Originator attribute shall be set to the router-id of the advertiser, and the Next-hop attribute shall be set to the local address for that session (Section 3.2) | {gap}, no test | Ze implements RTC (Route Target Constraint) as a DECODE-ONLY NLRI codec for display/analysis (internal/component/bgp/plugins/nlri/rtc/rtc.go DecodeNLRIHex/RunDecode; there is no encode/origination path). It never advertises RT membership NLRI, so it implements none of the Section 3.2 advertisement Originator/Next-hop procedure. Disclosed in docs/features/rfc-status.md. |
| [`RFC4684-3.2-2`](#rfc4684-3.2-2) When advertising RT membership NLRI to a non-client peer, if best path is from a non-client peer and an alternative client path exists, advertise the client path attributes (Section 3.2) | {gap}, no test | Ze does not advertise RT membership NLRI at all (its RTC support is a decode-only NLRI codec, internal/component/bgp/plugins/nlri/rtc/rtc.go, with no origination path), so it implements none of the Section 3.2 best-path/client-path advertisement selection. Disclosed in docs/features/rfc-status.md. |
| [`RFC4684-3.2-3`](#rfc4684-3.2-3) When processing RT membership NLRIs from internal iBGP peers, consider all available iBGP paths for a given RT prefix for building the outbound route filter, not just the best path (Section 3.2) | {gap}, no test | Ze builds no outbound route filter from RT membership -- its RTC support is a decode-only NLRI codec (internal/component/bgp/plugins/nlri/rtc/rtc.go) with no ORF construction or RT-membership-driven VPN route filtering. Disclosed in docs/features/rfc-status.md. |
| [`RFC4684-6-1`](#rfc4684-6-1) If delaying VPN route advertisement until End-of-RIB marker is received, MUST limit that delay to an upper bound (Section 6) | {gap}, no test | Ze does not gate VPN route advertisement on RT-membership state -- it has no RTC-driven VPN route distribution (the RTC codec is decode-only, internal/component/bgp/plugins/nlri/rtc/rtc.go), so there is no such End-of-RIB delay for Ze to bound. Disclosed in docs/features/rfc-status.md. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4684-3.2-1`](#rfc4684-3.2-1)

When advertising RT membership NLRI to a route-reflector client, the Originator attribute shall be set to the router-id of the advertiser, and the Next-hop attribute shall be set to the local address for that session (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4684-3.2-1, so no unit is bound to it.

### [`RFC4684-3.2-2`](#rfc4684-3.2-2)

When advertising RT membership NLRI to a non-client peer, if best path is from a non-client peer and an alternative client path exists, advertise the client path attributes (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4684-3.2-2, so no unit is bound to it.

### [`RFC4684-3.2-3`](#rfc4684-3.2-3)

When processing RT membership NLRIs from internal iBGP peers, consider all available iBGP paths for a given RT prefix for building the outbound route filter, not just the best path (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4684-3.2-3, so no unit is bound to it.

### [`RFC4684-6-1`](#rfc4684-6-1)

If delaying VPN route advertisement until End-of-RIB marker is received, MUST limit that delay to an upper bound (Section 6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4684-6-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 4684, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4684, so its obligations are stated where they were written.
