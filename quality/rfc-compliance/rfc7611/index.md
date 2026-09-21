# RFC 7611 - BGP ACCEPT_OWN Community Attribute

No row in the public ledger. Every requirement this repository extracted from RFC 7611, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 5 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 5 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 5 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 20.0% | 1 of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 5 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 5 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 80.0% | 4 of 5 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 8 |
| Gated MUST-level | 5 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Gated with no test | 4 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7611.md` |
| Requirement shard | `rfc/requirements/rfc7611.md` |
| RFC text | `rfc/full/rfc7611.txt` |

## Enrolment

Enrolled: BGP ACCEPT_OWN Community Attribute: five MUST-level rows after the 2026-09-21 extraction walk. One is annotated: RFC7611-3-1, the §3 extra decision-process step, is {not-applicable} because Ze does not implement the ACCEPT_OWN mechanism -- it recognizes the community value only for text display (internal/core/bgp/attribute/community.go:59-61,112 CommunityAcceptOwn = 0xFFFF0001) and has no L3VPN VRF-import code path that would honor it. The four rows this reason used to name (3-2 honor only on iBGP, 3-3 MUST NOT accept from eBGP, 3-4 remove before re-advertising to eBGP, 3-5 route reflectors propagate unchanged) state obligations RFC 7611 does not contain, and the walk deleted them; RFC7611-3-1 itself was re-texted, because the "all conditions MUST hold" row it used to carry is a MAY in §2.1 ("A router MAY accept a route whose ORIGINATOR_ID or NEXT_HOP value matches that of the receiving speaker if all of the following are true"). The walk added four MUST-level rows from the document's own text, none of them tested: 2.1-1 (a route MUST NOT ever be accepted back into its source VRF), 2.2-1 (discard the community when the address family does not allow the source VRF to be looked up), 2.3-1 (if controlled by configuration it MUST default to disabled) and 2.3-2 (when disabled, MUST NOT apply the §2.1 acceptance rules). The 2-1 and 5-1 SHOULDs and the 2.2-2 SHOULD NOT are not gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7611.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 4 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (1):** [`RFC7611-3-1`](#rfc7611-3-1)

**No test and no annotation (4):** [`RFC7611-2.1-1`](#rfc7611-2.1-1), [`RFC7611-2.2-1`](#rfc7611-2.2-1), [`RFC7611-2.3-1`](#rfc7611-2.3-1), [`RFC7611-2.3-2`](#rfc7611-2.3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7611-2.1-1` | A route MUST NOT ever be accepted back into its source VRF, even if it carries one or more RTs that match that VRF (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7611-2.2-1` | If a route carrying the ACCEPT_OWN community is received in an address family that does not allow the source VRF to be looked up, the ACCEPT_OWN community MUST be discarded (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7611-2.2-2` | When propagating routes into a VRF, the ACCEPT_OWN community SHOULD NOT be propagated, and it SHOULD NOT be attached to any routes that cannot be associated with a source VRF (§2.2) | SHOULD NOT | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7611-2.3-1` | ACCEPT_OWN handling SHOULD be controlled by configuration, and if controlled by configuration, it MUST default to being disabled (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7611-2.3-2` | When ACCEPT_OWN is disabled by configuration, either explicitly or by default, the router MUST NOT apply the special route acceptance rules detailed in Section 2.1 (§2.3) | MUST NOT | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7611-3-1` | The extra decision-process step MUST only be invoked during the best path selection process of VPN-IP routes, and MUST NOT be invoked for the best path selection of imported IP routes in a VRF (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 7611 ACCEPT_OWN mechanism and has no VPN-IP decision-process step to invoke. It recognizes the ACCEPT_OWN community value only for text display (internal/core/bgp/attribute/community.go:59-61,112 CommunityAcceptOwn = 0xFFFF0001), and no L3VPN VRF-import path exists that would insert the extra step after the LOCAL_PREF comparison. |
| `RFC7611-2-1` | Processing of the ACCEPT_OWN community SHOULD be controlled by configuration, and the functionality SHOULD default to being disabled (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7611-5-1` | Where a BGP speaker maintains multiple routing contexts by an approach other than RFC 4364, "VRF" should be understood to mean whatever construct provides a routing context, "Route Distinguisher" whatever construct allows a route's originator to associate that route with its source context, and "Route Target" whatever construct allows a route to be targeted for import into a context other than its source (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7611-2.1-1`](#rfc7611-2.1-1) A route MUST NOT ever be accepted back into its source VRF, even if it carries one or more RTs that match that VRF (§2.1) | no test | no test carries this requirement id |
| [`RFC7611-2.2-1`](#rfc7611-2.2-1) If a route carrying the ACCEPT_OWN community is received in an address family that does not allow the source VRF to be looked up, the ACCEPT_OWN community MUST be discarded (§2.2) | no test | no test carries this requirement id |
| [`RFC7611-2.3-1`](#rfc7611-2.3-1) ACCEPT_OWN handling SHOULD be controlled by configuration, and if controlled by configuration, it MUST default to being disabled (§2.3) | no test | no test carries this requirement id |
| [`RFC7611-2.3-2`](#rfc7611-2.3-2) When ACCEPT_OWN is disabled by configuration, either explicitly or by default, the router MUST NOT apply the special route acceptance rules detailed in Section 2.1 (§2.3) | no test | no test carries this requirement id |
| [`RFC7611-3-1`](#rfc7611-3-1) The extra decision-process step MUST only be invoked during the best path selection process of VPN-IP routes, and MUST NOT be invoked for the best path selection of imported IP routes in a VRF (§3) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 7611 ACCEPT_OWN mechanism and has no VPN-IP decision-process step to invoke. It recognizes the ACCEPT_OWN community value only for text display (internal/core/bgp/attribute/community.go:59-61,112 CommunityAcceptOwn = 0xFFFF0001), and no L3VPN VRF-import path exists that would insert the extra step after the LOCAL_PREF comparison. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7611-2.1-1`](#rfc7611-2.1-1)

A route MUST NOT ever be accepted back into its source VRF, even if it carries one or more RTs that match that VRF (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7611-2.1-1, so no unit is bound to it.

### [`RFC7611-2.2-1`](#rfc7611-2.2-1)

If a route carrying the ACCEPT_OWN community is received in an address family that does not allow the source VRF to be looked up, the ACCEPT_OWN community MUST be discarded (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7611-2.2-1, so no unit is bound to it.

### [`RFC7611-2.3-1`](#rfc7611-2.3-1)

ACCEPT_OWN handling SHOULD be controlled by configuration, and if controlled by configuration, it MUST default to being disabled (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7611-2.3-1, so no unit is bound to it.

### [`RFC7611-2.3-2`](#rfc7611-2.3-2)

When ACCEPT_OWN is disabled by configuration, either explicitly or by default, the router MUST NOT apply the special route acceptance rules detailed in Section 2.1 (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7611-2.3-2, so no unit is bound to it.

### [`RFC7611-3-1`](#rfc7611-3-1)

The extra decision-process step MUST only be invoked during the best path selection process of VPN-IP routes, and MUST NOT be invoked for the best path selection of imported IP routes in a VRF (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7611-3-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7611.txt |
| Source fingerprint | 63f038bbea1c9b4f |
| Record | rfc/extraction/rfc7611.json |
| Mapped sentences | 5 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 1 | walked | not stated |
| `2.2` | not stated | 1 | walked | not stated |
| `2.3` | not stated | 2 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 7611 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 7611, so its obligations are stated where they were written.
