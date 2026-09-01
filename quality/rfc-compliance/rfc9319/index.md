# RFC 9319 - The Use of maxLength in the Resource Public Key Infrastructure (RPKI)

No row in the public ledger. Every requirement this repository extracted from RFC 9319, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 11 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9319.md` |
| Requirement shard | `rfc/requirements/rfc9319.md` |
| RFC text | `rfc/full/rfc9319.txt` |

## Enrolment

Enrolled: The Use of maxLength in the RPKI (BCP 185): four MUST-level requirements, all {not-applicable} to Ze. RFC 9319 is operational guidance directed at RPKI OPERATORS and ROA issuers (5-1 review existing ROAs for minimality, 5-2 replace published ROAs as necessary, 5-3 repeat the review on policy changes) and at operators PROVIDING RTDR/Route-Origin-Validation filtering as a service (6-1 MUST NOT require non-minimal ROAs). Ze is an RPKI Relying Party: it consumes Validated ROA Payloads over RTR (internal/component/bgp/plugins/rpki/roa_cache.go, aspa_cache.go) and validates route origins against the maxLength bound (internal/component/bgp/plugins/rpki/validate.go:45); it never issues, publishes, or reviews ROAs and provides no RTDR filtering service, so none of these operator-side obligations has an applicable code path. Ze re-validates its own routes on VRP change per RFC 6811 Section 4 (origin_tracker.go), which is governed by the already-enrolled RFC 6811, not this operator BCP. No SHOULD/MAY requirements are gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 9319.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (4):** [`RFC9319-5-1`](#rfc9319-5-1), [`RFC9319-5-2`](#rfc9319-5-2), [`RFC9319-5-3`](#rfc9319-5-3), [`RFC9319-6-1`](#rfc9319-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9319-5-1` | Operators adopting minimal ROAs MUST perform a review of existing ROAs, especially those using maxLength, to ensure the set of included prefixes is minimal with respect to current BGP origination and routing policies (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Directed at RPKI OPERATORS who create and manage ROAs. Ze is an RPKI Relying Party: it consumes Validated ROA Payloads received over RTR (internal/component/bgp/plugins/rpki/roa_cache.go, aspa_cache.go "received via RTR") and validates route origins against them, including the maxLength bound (internal/component/bgp/plugins/rpki/validate.go:45), but it never issues, publishes, or reviews ROAs. The ROA-review obligation has no applicable code path in Ze. |
| `RFC9319-5-2` | Published ROAs MUST be replaced as necessary (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Directed at the operator/CA that publishes ROAs. Ze issues and publishes no ROAs -- it maintains only a read-only VRP cache populated from RTR (internal/component/bgp/plugins/rpki/roa_cache.go). Replacing published ROAs is a ROA-issuer action that a Relying Party does not perform. |
| `RFC9319-5-3` | Review MUST be repeated whenever the operator makes changes to origination or routing policy (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** An operator review-cadence obligation for a ROA issuer. Ze issues no ROAs and holds no origination/routing-policy-driven ROA set to re-review; as a Relying Party it only re-validates its own received routes when the VRP cache changes (RFC 6811 Section 4, internal/component/bgp/plugins/rpki/origin_tracker.go), which is governed by RFC 6811, not this operator BCP. |
| `RFC9319-6-1` | Operators providing RTDR filtering MUST NOT make the creation of non-minimal ROAs a pre-requisite for its use (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Directed at operators PROVIDING Route Origin Validation (RTDR) filtering as a service. Ze provides no RTDR filtering service to third parties and gates no feature on the creation of non-minimal ROAs; it only validates its own received routes against the VRP set it consumes (RFC 6811, internal/component/bgp/plugins/rpki/validate.go). There is no code path in Ze that could impose a non-minimal-ROA pre-requisite. |
| `RFC9319-5-4` | Operators SHOULD use minimal ROAs whenever possible, containing only IP prefixes actually originated in BGP (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-5-5` | Operators SHOULD avoid using the maxLength attribute in ROAs (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-5.1-1` | ROAs for DDoS mitigation SHOULD only include prefixes that are always originated plus those sometimes originated (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-5.1-2` | ROAs for DDoS mitigation SHOULD NOT include any IP prefixes the operator knows will not be originated in BGP (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-5.1-3` | ROAs for DDoS mitigation SHOULD NOT make use of maxLength unless doing so has no impact on the set of included prefixes (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-6-2` | Operators SHOULD NOT create non-minimal ROAs for the purpose of advertising RTDR routes (§6) | SHOULD NOT | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-7-1` | User interface designers SHOULD provide warnings about risks of non-minimal ROAs and maxLength usage (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9319-5-1`](#rfc9319-5-1) Operators adopting minimal ROAs MUST perform a review of existing ROAs, especially those using maxLength, to ensure the set of included prefixes is minimal with respect to current BGP origination and routing policies (§5) | no test | no test carries this requirement id; annotated {not-applicable}: Directed at RPKI OPERATORS who create and manage ROAs. Ze is an RPKI Relying Party: it consumes Validated ROA Payloads received over RTR (internal/component/bgp/plugins/rpki/roa_cache.go, aspa_cache.go "received via RTR") and validates route origins against them, including the maxLength bound (internal/component/bgp/plugins/rpki/validate.go:45), but it never issues, publishes, or reviews ROAs. The ROA-review obligation has no applicable code path in Ze. |
| [`RFC9319-5-2`](#rfc9319-5-2) Published ROAs MUST be replaced as necessary (§5) | no test | no test carries this requirement id; annotated {not-applicable}: Directed at the operator/CA that publishes ROAs. Ze issues and publishes no ROAs -- it maintains only a read-only VRP cache populated from RTR (internal/component/bgp/plugins/rpki/roa_cache.go). Replacing published ROAs is a ROA-issuer action that a Relying Party does not perform. |
| [`RFC9319-5-3`](#rfc9319-5-3) Review MUST be repeated whenever the operator makes changes to origination or routing policy (§5) | no test | no test carries this requirement id; annotated {not-applicable}: An operator review-cadence obligation for a ROA issuer. Ze issues no ROAs and holds no origination/routing-policy-driven ROA set to re-review; as a Relying Party it only re-validates its own received routes when the VRP cache changes (RFC 6811 Section 4, internal/component/bgp/plugins/rpki/origin_tracker.go), which is governed by RFC 6811, not this operator BCP. |
| [`RFC9319-6-1`](#rfc9319-6-1) Operators providing RTDR filtering MUST NOT make the creation of non-minimal ROAs a pre-requisite for its use (§6) | no test | no test carries this requirement id; annotated {not-applicable}: Directed at operators PROVIDING Route Origin Validation (RTDR) filtering as a service. Ze provides no RTDR filtering service to third parties and gates no feature on the creation of non-minimal ROAs; it only validates its own received routes against the VRP set it consumes (RFC 6811, internal/component/bgp/plugins/rpki/validate.go). There is no code path in Ze that could impose a non-minimal-ROA pre-requisite. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9319-5-1`](#rfc9319-5-1)

Operators adopting minimal ROAs MUST perform a review of existing ROAs, especially those using maxLength, to ensure the set of included prefixes is minimal with respect to current BGP origination and routing policies (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9319-5-1, so no unit is bound to it.

### [`RFC9319-5-2`](#rfc9319-5-2)

Published ROAs MUST be replaced as necessary (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9319-5-2, so no unit is bound to it.

### [`RFC9319-5-3`](#rfc9319-5-3)

Review MUST be repeated whenever the operator makes changes to origination or routing policy (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9319-5-3, so no unit is bound to it.

### [`RFC9319-6-1`](#rfc9319-6-1)

Operators providing RTDR filtering MUST NOT make the creation of non-minimal ROAs a pre-requisite for its use (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9319-6-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 9319, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9319, so its obligations are stated where they were written.
