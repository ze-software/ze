# RFC 7311 - The Accumulated IGP Metric Attribute for BGP

Partial. Every requirement this repository extracted from RFC 7311, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 18.2% | 4 of 22 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 22 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 22 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 40.0% | 4 of 10 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 22 | of 26 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 22 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 22 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 22 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 22 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 81.8% | 18 of 22 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 22 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 26 |
| Gated MUST-level | 22 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 17 |
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
| No test and no annotation | 17 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **22** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7311-3-1`](#rfc7311-3-1), [`RFC7311-3-2`](#rfc7311-3-2), [`RFC7311-3-3`](#rfc7311-3-3), [`RFC7311-3.2-4`](#rfc7311-3.2-4)

**Annotated instead of tested (1):** [`RFC7311-3.2-1`](#rfc7311-3.2-1)

**No test and no annotation (17):** [`RFC7311-3.2-5`](#rfc7311-3.2-5), [`RFC7311-3.2-6`](#rfc7311-3.2-6), [`RFC7311-3.3-1`](#rfc7311-3.3-1), [`RFC7311-3.3-2`](#rfc7311-3.3-2), [`RFC7311-3.3-3`](#rfc7311-3.3-3), [`RFC7311-3.3-4`](#rfc7311-3.3-4), [`RFC7311-3.4.1-1`](#rfc7311-3.4.1-1), [`RFC7311-3.4.1-2`](#rfc7311-3.4.1-2), [`RFC7311-3.4.1-3`](#rfc7311-3.4.1-3), [`RFC7311-3.4.3-1`](#rfc7311-3.4.3-1), [`RFC7311-3.4.3-2`](#rfc7311-3.4.3-2), [`RFC7311-3.4.3-3`](#rfc7311-3.4.3-3), [`RFC7311-3.4.3-4`](#rfc7311-3.4.3-4), [`RFC7311-3.4.3-5`](#rfc7311-3.4.3-5), [`RFC7311-3.4.3-6`](#rfc7311-3.4.3-6), [`RFC7311-3.4.3-7`](#rfc7311-3.4.3-7), [`RFC7311-4.1-1`](#rfc7311-4.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7311-3-1` | AIGP TLV type 1 MUST have length 11 (3 header + 8 metric) (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGP`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L20). **negative:** `unit/verify` [`TestParseAIGPMalformedMetricWrongLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L101) |
| `RFC7311-3-2` | Total attribute length must be consistent with contained TLVs (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L52). **negative:** `unit/verify` [`TestParseAIGPMalformedTruncatedValue`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L89) |
| `RFC7311-3-3` | Unknown TLV types MUST be preserved and not discarded (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L57). **negative:** `unit/verify` [`TestAIGPWriteToMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L142) |
| `RFC7311-3.2-4` | A received path attribute carrying the AIGP codepoint with the transitive bit set is a malformed AIGP attribute and MUST be discarded (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7311AIGPNonTransitiveKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L103). **positive:** `unit/verify` [`TestRFC7606AIGPNonTransitiveIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_aigp_test.go#L104). **negative:** `unit/verify` [`TestRFC7311AIGPTransitiveDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L60). **negative:** `unit/verify` [`TestRFC7606AIGPTransitiveIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_aigp_test.go#L68) |
| `RFC7311-3.2-1` | AIGP attribute MUST NOT be attached to or propagated to a route advertised to a peer in a different administrative domain (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not strip the AIGP attribute at the eBGP administrative-domain boundary. AIGP is an optional NON-transitive attribute (RFC 7311 Section 3; internal/core/bgp/attribute/aigp.go, AIGP.Flags) and Ze forwards received route attributes largely verbatim (internal/component/bgp/reactor/forward_context.go:11 reuses the source context with only AS_PATH/AGGREGATOR ASN-width rewrite and the eBGP local-AS prepend), with no AIGP-specific removal for a peer in a different administrative domain. The AIGP plugin is an explicit stub (internal/component/bgp/plugins/aigp/aigp.go:7-8 "Full AIGP processing will be added when the spec-aigp work is implemented"). AIGP can be removed only by an explicit operator remove-attribute policy, not automatically at the domain boundary, so a received AIGP would leak to a different-domain eBGP peer. Disclosed in docs/features/rfc-status.md |
| `RFC7311-3.2-5` | When receiving a BGP Update message containing a malformed AIGP attribute, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute; that is, it "MUST be quietly ignored and not passed along to other BGP peers" (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.2-2` | If an AIGP attribute is received and its first AIGP TLV contains the maximum value 0xffffffffffffffff, the attribute SHOULD be considered to be malformed and SHOULD be discarded (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.2-6` | An AIGP attribute MUST NOT be considered to be malformed because it contains more than one TLV of a given type or because it contains TLVs of unknown types (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.3-1` | An implementation that supports the AIGP attribute MUST support a per-session configuration item, AIGP_SESSION, that indicates whether the attribute is enabled or disabled for use on that session (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.3-2` | For all External BGP (EBGP) sessions other than those between members of the same BGP Confederation, the default value of AIGP_SESSION MUST be "disabled" (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.3-3` | The AIGP attribute MUST NOT be sent on any BGP session for which AIGP_SESSION is disabled (§3.3) | MUST NOT | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.3-4` | If an AIGP attribute is received on a BGP session for which AIGP_SESSION is disabled, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.1-1` | An implementation that supports the AIGP attribute MUST support a configuration item, AIGP_ORIGINATE, that enables or disables its creation and attachment to routes (§3.4.1) | MUST | 3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.1-2` | The default value of AIGP_ORIGINATE MUST be "disabled" (§3.4.1) | MUST | 3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.1-3` | A BGP speaker R MUST NOT add the AIGP attribute to any route for which R does not set itself as the next hop (§3.4.1) | MUST NOT | 3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.3-1` | If R1 does not change the next hop of the route, then R1 MUST NOT change the AIGP attribute value of the route (§3.4.3) | MUST NOT | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.3-2` | In all the computations of §3.4.3, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.3-3` | Increasing the AIGP value MUST NOT cause the value to wrap around (§3.4.3) | MUST NOT | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.3-4` | If R1 changes the next hop to itself and R1's route to R2 is either an IGP-learned route or a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2 (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.3-5` | A MUST be increased by a non-zero amount (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.3-6` | When R1 and R2 are EBGP neighbors with a direct link on which no IGP is running, and R1 changes the next hop of a route from R2 to R1, the AIGP TLV value MUST be increased by a non-zero amount (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.3-7` | Any change in the AIGP TLV values of the next hops that are recursively resolved MUST trigger a new AIGP computation for that route (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-4.1-1` | Assuming that the BGP decision process invokes the tie-breaking procedures, the procedures of §4.1 MUST be executed BEFORE any of the tie-breaking procedures described in [BGP], Section 9.1.2.2 are executed (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.3-5` | For IBGP sessions, and for EBGP sessions between members of the same BGP Confederation, the default value of AIGP_SESSION SHOULD be "enabled" (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.1-1` | "This document only considers the use of the AIGP attribute in networks where each router uses tunneling of some sort to deliver a packet to its BGP next hop.  Use of the AIGP attribute in other scenarios is outside the scope of this document" (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.1-4` | The AIGP attribute may be added only to a static route not leading outside the AIGP administrative domain that is redistributed into BGP, an IGP route redistributed into BGP, an IBGP-learned route whose AS_PATH is empty, or an EBGP-learned route whose AS_PATH contains only ASes in the same AIGP administrative domain (§3.4.1) | MAY | 3.4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7311-3.2-1`](#rfc7311-3.2-1) AIGP attribute MUST NOT be attached to or propagated to a route advertised to a peer in a different administrative domain (§3.2) | {gap}, no test | Ze does not strip the AIGP attribute at the eBGP administrative-domain boundary. AIGP is an optional NON-transitive attribute (RFC 7311 Section 3; internal/core/bgp/attribute/aigp.go, AIGP.Flags) and Ze forwards received route attributes largely verbatim (internal/component/bgp/reactor/forward_context.go:11 reuses the source context with only AS_PATH/AGGREGATOR ASN-width rewrite and the eBGP local-AS prepend), with no AIGP-specific removal for a peer in a different administrative domain. The AIGP plugin is an explicit stub (internal/component/bgp/plugins/aigp/aigp.go:7-8 "Full AIGP processing will be added when the spec-aigp work is implemented"). AIGP can be removed only by an explicit operator remove-attribute policy, not automatically at the domain boundary, so a received AIGP would leak to a different-domain eBGP peer. Disclosed in docs/features/rfc-status.md |
| [`RFC7311-3.2-5`](#rfc7311-3.2-5) When receiving a BGP Update message containing a malformed AIGP attribute, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute; that is, it "MUST be quietly ignored and not passed along to other BGP peers" (§3.2) | no test | no test carries this requirement id |
| [`RFC7311-3.2-6`](#rfc7311-3.2-6) An AIGP attribute MUST NOT be considered to be malformed because it contains more than one TLV of a given type or because it contains TLVs of unknown types (§3.2) | no test | no test carries this requirement id |
| [`RFC7311-3.3-1`](#rfc7311-3.3-1) An implementation that supports the AIGP attribute MUST support a per-session configuration item, AIGP_SESSION, that indicates whether the attribute is enabled or disabled for use on that session (§3.3) | no test | no test carries this requirement id |
| [`RFC7311-3.3-2`](#rfc7311-3.3-2) For all External BGP (EBGP) sessions other than those between members of the same BGP Confederation, the default value of AIGP_SESSION MUST be "disabled" (§3.3) | no test | no test carries this requirement id |
| [`RFC7311-3.3-3`](#rfc7311-3.3-3) The AIGP attribute MUST NOT be sent on any BGP session for which AIGP_SESSION is disabled (§3.3) | no test | no test carries this requirement id |
| [`RFC7311-3.3-4`](#rfc7311-3.3-4) If an AIGP attribute is received on a BGP session for which AIGP_SESSION is disabled, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute (§3.3) | no test | no test carries this requirement id |
| [`RFC7311-3.4.1-1`](#rfc7311-3.4.1-1) An implementation that supports the AIGP attribute MUST support a configuration item, AIGP_ORIGINATE, that enables or disables its creation and attachment to routes (§3.4.1) | no test | no test carries this requirement id |
| [`RFC7311-3.4.1-2`](#rfc7311-3.4.1-2) The default value of AIGP_ORIGINATE MUST be "disabled" (§3.4.1) | no test | no test carries this requirement id |
| [`RFC7311-3.4.1-3`](#rfc7311-3.4.1-3) A BGP speaker R MUST NOT add the AIGP attribute to any route for which R does not set itself as the next hop (§3.4.1) | no test | no test carries this requirement id |
| [`RFC7311-3.4.3-1`](#rfc7311-3.4.3-1) If R1 does not change the next hop of the route, then R1 MUST NOT change the AIGP attribute value of the route (§3.4.3) | no test | no test carries this requirement id |
| [`RFC7311-3.4.3-2`](#rfc7311-3.4.3-2) In all the computations of §3.4.3, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff (§3.4.3) | no test | no test carries this requirement id |
| [`RFC7311-3.4.3-3`](#rfc7311-3.4.3-3) Increasing the AIGP value MUST NOT cause the value to wrap around (§3.4.3) | no test | no test carries this requirement id |
| [`RFC7311-3.4.3-4`](#rfc7311-3.4.3-4) If R1 changes the next hop to itself and R1's route to R2 is either an IGP-learned route or a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2 (§3.4.3) | no test | no test carries this requirement id |
| [`RFC7311-3.4.3-5`](#rfc7311-3.4.3-5) A MUST be increased by a non-zero amount (§3.4.3) | no test | no test carries this requirement id |
| [`RFC7311-3.4.3-6`](#rfc7311-3.4.3-6) When R1 and R2 are EBGP neighbors with a direct link on which no IGP is running, and R1 changes the next hop of a route from R2 to R1, the AIGP TLV value MUST be increased by a non-zero amount (§3.4.3) | no test | no test carries this requirement id |
| [`RFC7311-3.4.3-7`](#rfc7311-3.4.3-7) Any change in the AIGP TLV values of the next hops that are recursively resolved MUST trigger a new AIGP computation for that route (§3.4.3) | no test | no test carries this requirement id |
| [`RFC7311-4.1-1`](#rfc7311-4.1-1) Assuming that the BGP decision process invokes the tie-breaking procedures, the procedures of §4.1 MUST be executed BEFORE any of the tie-breaking procedures described in [BGP], Section 9.1.2.2 are executed (§4.1) | no test | no test carries this requirement id |

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

### [`RFC7311-3.2-5`](#rfc7311-3.2-5)

When receiving a BGP Update message containing a malformed AIGP attribute, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute; that is, it "MUST be quietly ignored and not passed along to other BGP peers" (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.2-5, so no unit is bound to it.

### [`RFC7311-3.2-6`](#rfc7311-3.2-6)

An AIGP attribute MUST NOT be considered to be malformed because it contains more than one TLV of a given type or because it contains TLVs of unknown types (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.2-6, so no unit is bound to it.

### [`RFC7311-3.3-1`](#rfc7311-3.3-1)

An implementation that supports the AIGP attribute MUST support a per-session configuration item, AIGP_SESSION, that indicates whether the attribute is enabled or disabled for use on that session (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.3-1, so no unit is bound to it.

### [`RFC7311-3.3-2`](#rfc7311-3.3-2)

For all External BGP (EBGP) sessions other than those between members of the same BGP Confederation, the default value of AIGP_SESSION MUST be "disabled" (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.3-2, so no unit is bound to it.

### [`RFC7311-3.3-3`](#rfc7311-3.3-3)

The AIGP attribute MUST NOT be sent on any BGP session for which AIGP_SESSION is disabled (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.3-3, so no unit is bound to it.

### [`RFC7311-3.3-4`](#rfc7311-3.3-4)

If an AIGP attribute is received on a BGP session for which AIGP_SESSION is disabled, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.3-4, so no unit is bound to it.

### [`RFC7311-3.4.1-1`](#rfc7311-3.4.1-1)

An implementation that supports the AIGP attribute MUST support a configuration item, AIGP_ORIGINATE, that enables or disables its creation and attachment to routes (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.4.1-1, so no unit is bound to it.

### [`RFC7311-3.4.1-2`](#rfc7311-3.4.1-2)

The default value of AIGP_ORIGINATE MUST be "disabled" (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.4.1-2, so no unit is bound to it.

### [`RFC7311-3.4.1-3`](#rfc7311-3.4.1-3)

A BGP speaker R MUST NOT add the AIGP attribute to any route for which R does not set itself as the next hop (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.4.1-3, so no unit is bound to it.

### [`RFC7311-3.4.3-1`](#rfc7311-3.4.3-1)

If R1 does not change the next hop of the route, then R1 MUST NOT change the AIGP attribute value of the route (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.4.3-1, so no unit is bound to it.

### [`RFC7311-3.4.3-2`](#rfc7311-3.4.3-2)

In all the computations of §3.4.3, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.4.3-2, so no unit is bound to it.

### [`RFC7311-3.4.3-3`](#rfc7311-3.4.3-3)

Increasing the AIGP value MUST NOT cause the value to wrap around (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.4.3-3, so no unit is bound to it.

### [`RFC7311-3.4.3-4`](#rfc7311-3.4.3-4)

If R1 changes the next hop to itself and R1's route to R2 is either an IGP-learned route or a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2 (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.4.3-4, so no unit is bound to it.

### [`RFC7311-3.4.3-5`](#rfc7311-3.4.3-5)

A MUST be increased by a non-zero amount (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.4.3-5, so no unit is bound to it.

### [`RFC7311-3.4.3-6`](#rfc7311-3.4.3-6)

When R1 and R2 are EBGP neighbors with a direct link on which no IGP is running, and R1 changes the next hop of a route from R2 to R1, the AIGP TLV value MUST be increased by a non-zero amount (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.4.3-6, so no unit is bound to it.

### [`RFC7311-3.4.3-7`](#rfc7311-3.4.3-7)

Any change in the AIGP TLV values of the next hops that are recursively resolved MUST trigger a new AIGP computation for that route (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-3.4.3-7, so no unit is bound to it.

### [`RFC7311-4.1-1`](#rfc7311-4.1-1)

Assuming that the BGP decision process invokes the tie-breaking procedures, the procedures of §4.1 MUST be executed BEFORE any of the tie-breaking procedures described in [BGP], Section 9.1.2.2 are executed (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7311-4.1-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7311.txt |
| Source fingerprint | cfedbb96c63b8a55 |
| Record | rfc/extraction/rfc7311.json |
| Mapped sentences | 20 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 4 | walked | not stated |
| `3.3` | not stated | 5 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.4.1` | not stated | 4 | walked | not stated |
| `3.4.2` | not stated | 0 | walked | not stated |
| `3.4.3` | not stated | 7 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence opens "That is," and restates the preceding MUST of the same paragraph, quoting [BGP] Section 5; site 3.2:1 already maps that obligation. | That is, it "MUST be quietly ignored and not passed along to other BGP peers" (see [BGP], Section 5). |
| `3.3:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence opens "That is," and restates the preceding MUST of the same paragraph, quoting [BGP] Section 5; site 3.3:4 already maps that obligation. | That is, it "MUST be quietly ignored and not passed along to other BGP peers" (see [BGP], Section 5). |

## Superseded

No document obsoletes RFC 7311, so its obligations are stated where they were written.
