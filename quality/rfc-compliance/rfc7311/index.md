# RFC 7311 - The Accumulated IGP Metric Attribute for BGP

Partial. Every requirement this repository extracted from RFC 7311, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.0% | 4 of 5 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 5 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 40.0% | 4 of 10 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 5 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 5 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 5 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.0% | 1 of 5 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 5 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 9 |
| Gated MUST-level | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tagged units | 10 |
| Recorded audit verdicts | 0 |
| Discrimination records | 4 |
| Summary | `rfc/short/rfc7311.md` |
| Requirement shard | `rfc/requirements/rfc7311.md` |
| RFC text | `rfc/full/rfc7311.txt` |

## Enrolment

Enrolled: The Accumulated IGP Metric Attribute for BGP (AIGP, code 26): four MUST-level requirements. Three over the codec (internal/core/bgp/attribute/aigp.go) are tested with both polarities in aigp_test.go: RFC7311-3-1 (type-1 TLV MUST have length 11) via TestParseAIGP (valid length-11 metric accepted) and TestParseAIGPMalformedMetricWrongLength (length 8 rejected, aigp.go:118); RFC7311-3-2 (total attribute length consistent with contained TLVs) via TestParseAIGPMultipleTLVs (two TLVs summing to the total) and TestParseAIGPMalformedTruncatedValue (declared TLV overruns the buffer -> error, aigp.go:111); RFC7311-3-3 (unknown TLV types preserved not discarded) via TestParseAIGPMultipleTLVs (unknown type-2 retained with exact data while type-1 still interpreted) and TestAIGPWriteToMultipleTLVs (unknown TLV survives the WriteTo re-encode round-trip). RFC7311-3.2-4 (a received AIGP with the transitive bit set is malformed and is discarded) is gated with both polarities by TestRFC7606AIGPTransitiveIsDiscarded and TestRFC7606AIGPNonTransitiveIsKept (internal/component/bgp/message/rfc7606_aigp_test.go) at validateAttributeFlags (internal/component/bgp/message/rfc7606.go), which answers attribute discard on attribute code 26 inside the RFC 7606 walk, and again by TestRFC7311AIGPTransitiveDiscardedOnReceive and TestRFC7311AIGPNonTransitiveKeptOnReceive (internal/component/bgp/reactor/rfc7311_aigp_receive_test.go) over enforceRFC7606, the receive path a peer's UPDATE takes, where the malformed AIGP leaves one ATTR_TOMBSTONE and the route's own attributes ride on. RFC7311-3.2-1 (MUST NOT propagate AIGP to a different-administrative-domain eBGP peer) is {gap}: Ze forwards received attributes verbatim (forward_context.go:11) with no AIGP admin-domain strip and the AIGP plugin is a stub, so received AIGP leaks to eBGP; disclosed in the docs/features/rfc-status.md RFC 7311 row (Partial). The 3-4/3.2-2/3.2-3 SHOULDs and 3.1-1 MAY are not gated.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- AIGP wire encoding, decoding, JSON, and set/increment/decrement filters
- TLV validation (type-1 length 11, total-length consistency, unknown-TLV preservation) gated per requirement in [`rfc/requirements/rfc7311.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7311.md). Ze sends the attribute optional and non-transitive (flags 0x80), and discards a received AIGP whose transitive bit is set (§3.2) with the rest of the UPDATE processed.


**What the ledger says remains**

AIGP is not consumed by best-path selection, and Ze does not strip AIGP at the eBGP administrative-domain boundary (§3.2 MUST NOT): received AIGP is forwarded verbatim, removable only by explicit operator policy. Gated as a gap in [`rfc/short/rfc7311.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7311.md).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7311-3-1`](#rfc7311-3-1), [`RFC7311-3-2`](#rfc7311-3-2), [`RFC7311-3-3`](#rfc7311-3-3), [`RFC7311-3.2-4`](#rfc7311-3.2-4)

**Annotated instead of tested (1):** [`RFC7311-3.2-1`](#rfc7311-3.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7311-3-1` | AIGP TLV type 1 MUST have length 11 (3 header + 8 metric) (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGP`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L20). **negative:** `unit/verify` [`TestParseAIGPMalformedMetricWrongLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L101) |
| `RFC7311-3-2` | Total attribute length must be consistent with contained TLVs (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L52). **negative:** `unit/verify` [`TestParseAIGPMalformedTruncatedValue`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L89) |
| `RFC7311-3-3` | Unknown TLV types MUST be preserved and not discarded (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L57). **negative:** `unit/verify` [`TestAIGPWriteToMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L142) |
| `RFC7311-3.2-4` | A received path attribute carrying the AIGP codepoint with the transitive bit set is a malformed AIGP attribute and MUST be discarded (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7311AIGPNonTransitiveKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L103). **positive:** `unit/verify` [`TestRFC7606AIGPNonTransitiveIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_aigp_test.go#L104). **negative:** `unit/verify` [`TestRFC7311AIGPTransitiveDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L60). **negative:** `unit/verify` [`TestRFC7606AIGPTransitiveIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_aigp_test.go#L68) |
| `RFC7311-3.2-1` | AIGP attribute MUST NOT be attached to or propagated to a route advertised to a peer in a different administrative domain (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not strip the AIGP attribute at the eBGP administrative-domain boundary. AIGP is an optional NON-transitive attribute (RFC 7311 Section 3; internal/core/bgp/attribute/aigp.go, AIGP.Flags) and Ze forwards received route attributes largely verbatim (internal/component/bgp/reactor/forward_context.go:11 reuses the source context with only AS_PATH/AGGREGATOR ASN-width rewrite and the eBGP local-AS prepend), with no AIGP-specific removal for a peer in a different administrative domain. The AIGP plugin is an explicit stub (internal/component/bgp/plugins/aigp/aigp.go:7-8 "Full AIGP processing will be added when the spec-aigp work is implemented"). AIGP can be removed only by an explicit operator remove-attribute policy, not automatically at the domain boundary, so a received AIGP would leak to a different-domain eBGP peer. Disclosed in docs/features/rfc-status.md |
| `RFC7311-3-4` | Unknown TLV types SHOULD be preserved and propagated unchanged (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.2-2` | A BGP speaker that receives an AIGP attribute SHOULD propagate it to IBGP peers and to eBGP peers within the same administrative domain (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.2-3` | When propagating, the accumulated metric SHOULD be updated by adding the IGP metric to the next hop (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.1-1` | A BGP speaker MAY attach an AIGP attribute to a connected route, a route learned from an IGP, or a route already carrying AIGP (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7311-3.2-1`](#rfc7311-3.2-1) AIGP attribute MUST NOT be attached to or propagated to a route advertised to a peer in a different administrative domain (§3.2) | {gap}, no test | Ze does not strip the AIGP attribute at the eBGP administrative-domain boundary. AIGP is an optional NON-transitive attribute (RFC 7311 Section 3; internal/core/bgp/attribute/aigp.go, AIGP.Flags) and Ze forwards received route attributes largely verbatim (internal/component/bgp/reactor/forward_context.go:11 reuses the source context with only AS_PATH/AGGREGATOR ASN-width rewrite and the eBGP local-AS prepend), with no AIGP-specific removal for a peer in a different administrative domain. The AIGP plugin is an explicit stub (internal/component/bgp/plugins/aigp/aigp.go:7-8 "Full AIGP processing will be added when the spec-aigp work is implemented"). AIGP can be removed only by an explicit operator remove-attribute policy, not automatically at the domain boundary, so a received AIGP would leak to a different-domain eBGP peer. Disclosed in docs/features/rfc-status.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7311-3-1`](#rfc7311-3-1)

AIGP TLV type 1 MUST have length 11 (3 header + 8 metric) (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseAIGPMalformedMetricWrongLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L101) | unit/verify | unproven |
| positive | [`TestParseAIGP`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L20) | unit/verify | unproven |

### [`RFC7311-3-2`](#rfc7311-3-2)

Total attribute length must be consistent with contained TLVs (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseAIGPMalformedTruncatedValue`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L89) | unit/verify | unproven |
| positive | [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L52) | unit/verify | unproven |

### [`RFC7311-3-3`](#rfc7311-3-3)

Unknown TLV types MUST be preserved and not discarded (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPWriteToMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L142) | unit/verify | unproven |
| positive | [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L57) | unit/verify | unproven |

### [`RFC7311-3.2-4`](#rfc7311-3.2-4)

A received path attribute carrying the AIGP codepoint with the transitive bit set is a malformed AIGP attribute and MUST be discarded (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606AIGPTransitiveIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_aigp_test.go#L68) | unit/verify | revert, verified |
| negative | [`TestRFC7311AIGPTransitiveDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC7606AIGPNonTransitiveIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_aigp_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC7311AIGPNonTransitiveKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L103) | unit/verify | revert, verified |

### [`RFC7311-3.2-1`](#rfc7311-3.2-1)

AIGP attribute MUST NOT be attached to or propagated to a route advertised to a peer in a different administrative domain (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.2-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7311, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7311, so its obligations are stated where they were written.
