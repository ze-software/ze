# RFC 7611 - BGP ACCEPT_OWN Community Attribute

No row in the public ledger. Every requirement this repository extracted from RFC 7611, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 5 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Requirements | 6 |
| Gated MUST-level | 5 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7611.md` |
| Requirement shard | `rfc/requirements/rfc7611.md` |
| RFC text | `rfc/full/rfc7611.txt` |

## Enrolment

Enrolled: BGP ACCEPT_OWN Community Attribute: five MUST-level requirements, all {not-applicable} to Ze. Ze does not implement the RFC 7611 ACCEPT_OWN mechanism -- it recognizes the ACCEPT_OWN community value only for text display (internal/core/bgp/attribute/community.go:59-61,112 CommunityAcceptOwn = 0xFFFF0001) and has no L3VPN VRF-import code path that would honor it. RFC7611-3-1 (all honoring conditions), RFC7611-3-2 (honor only on iBGP), RFC7611-3-3 (MUST NOT accept from eBGP): no honoring/VRF-import code path exists. RFC7611-3-4 (remove before re-advertising to eBGP): Ze neither sets nor honors ACCEPT_OWN, so it does not participate in the mechanism that requires stripping. RFC7611-3-5 (RR propagates unchanged): Ze RR forwards communities verbatim (generic transparency already gated under RFC 4456 / RFC 7947), with no ACCEPT_OWN-specific handling. The 5-1 SHOULD is not gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7611.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (5):** [`RFC7611-3-1`](#rfc7611-3-1), [`RFC7611-3-2`](#rfc7611-3-2), [`RFC7611-3-3`](#rfc7611-3-3), [`RFC7611-3-4`](#rfc7611-3-4), [`RFC7611-3-5`](#rfc7611-3-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7611-3-1` | All conditions MUST hold for ACCEPT_OWN to be honored: community present, received from iBGP, own AS in AS_PATH, VPN route, targets different VRF (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 7611 ACCEPT_OWN mechanism. It recognizes the ACCEPT_OWN community value only for text display (internal/core/bgp/attribute/community.go:59-61,112 CommunityAcceptOwn = 0xFFFF0001) and has no L3VPN VRF-import code path that would honor it (no import of a self-originated VPN route into a different VRF; grep across internal/component/bgp/ and internal/core/bgp/ finds only the name definition), so none of the honoring conditions is ever evaluated. |
| `RFC7611-3-2` | ACCEPT_OWN MUST only be honored on routes received from iBGP peers (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** The "honored only on iBGP routes" restriction governs a router that honors ACCEPT_OWN via VRF import, which Ze does not implement (community.go:59-61 defines the value for display only; there is no VRF-import honoring path). |
| `RFC7611-3-3` | ACCEPT_OWN MUST NOT be accepted from eBGP peers (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze never honors ACCEPT_OWN from any peer (it has no VRF-import mechanism), so the "MUST NOT accept from eBGP" restriction has no applicable honoring code path -- the community is recognized only as a named value for display (community.go:112). |
| `RFC7611-3-4` | ACCEPT_OWN community MUST be removed before re-advertising to eBGP peers (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not participate in the ACCEPT_OWN VPN mechanism -- it neither sets ACCEPT_OWN on VRF export nor honors it on import (community.go:59-61 is display-only) -- so the "remove before re-advertising to eBGP" obligation, which prevents a participating router from leaking the VPN-internal ACCEPT_OWN signal, has no applicable code path. |
| `RFC7611-3-5` | Route reflectors MUST propagate the ACCEPT_OWN community unchanged (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze's route reflector forwards route communities verbatim and does not special-case ACCEPT_OWN, so a received ACCEPT_OWN would be reflected unchanged as a side effect of the generic community-transparent forwarding already gated under RFC 4456 / RFC 7947; Ze implements no ACCEPT_OWN VPN mechanism (community.go:59-61 is display-only), so this RR requirement has no ACCEPT_OWN-specific code path. |
| `RFC7611-5-1` | ACCEPT_OWN SHOULD be used with explicit configuration, not enabled by default (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7611-3-1`](#rfc7611-3-1) All conditions MUST hold for ACCEPT_OWN to be honored: community present, received from iBGP, own AS in AS_PATH, VPN route, targets different VRF (§3) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 7611 ACCEPT_OWN mechanism. It recognizes the ACCEPT_OWN community value only for text display (internal/core/bgp/attribute/community.go:59-61,112 CommunityAcceptOwn = 0xFFFF0001) and has no L3VPN VRF-import code path that would honor it (no import of a self-originated VPN route into a different VRF; grep across internal/component/bgp/ and internal/core/bgp/ finds only the name definition), so none of the honoring conditions is ever evaluated. |
| [`RFC7611-3-2`](#rfc7611-3-2) ACCEPT_OWN MUST only be honored on routes received from iBGP peers (§3) | no test | no test carries this requirement id; annotated {not-applicable}: The "honored only on iBGP routes" restriction governs a router that honors ACCEPT_OWN via VRF import, which Ze does not implement (community.go:59-61 defines the value for display only; there is no VRF-import honoring path). |
| [`RFC7611-3-3`](#rfc7611-3-3) ACCEPT_OWN MUST NOT be accepted from eBGP peers (§3) | no test | no test carries this requirement id; annotated {not-applicable}: Ze never honors ACCEPT_OWN from any peer (it has no VRF-import mechanism), so the "MUST NOT accept from eBGP" restriction has no applicable honoring code path -- the community is recognized only as a named value for display (community.go:112). |
| [`RFC7611-3-4`](#rfc7611-3-4) ACCEPT_OWN community MUST be removed before re-advertising to eBGP peers (§3) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not participate in the ACCEPT_OWN VPN mechanism -- it neither sets ACCEPT_OWN on VRF export nor honors it on import (community.go:59-61 is display-only) -- so the "remove before re-advertising to eBGP" obligation, which prevents a participating router from leaking the VPN-internal ACCEPT_OWN signal, has no applicable code path. |
| [`RFC7611-3-5`](#rfc7611-3-5) Route reflectors MUST propagate the ACCEPT_OWN community unchanged (§3) | no test | no test carries this requirement id; annotated {not-applicable}: Ze's route reflector forwards route communities verbatim and does not special-case ACCEPT_OWN, so a received ACCEPT_OWN would be reflected unchanged as a side effect of the generic community-transparent forwarding already gated under RFC 4456 / RFC 7947; Ze implements no ACCEPT_OWN VPN mechanism (community.go:59-61 is display-only), so this RR requirement has no ACCEPT_OWN-specific code path. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7611-3-1`](#rfc7611-3-1)

All conditions MUST hold for ACCEPT_OWN to be honored: community present, received from iBGP, own AS in AS_PATH, VPN route, targets different VRF (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7611-3-1, so no unit is bound to it.

### [`RFC7611-3-2`](#rfc7611-3-2)

ACCEPT_OWN MUST only be honored on routes received from iBGP peers (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7611-3-2, so no unit is bound to it.

### [`RFC7611-3-3`](#rfc7611-3-3)

ACCEPT_OWN MUST NOT be accepted from eBGP peers (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7611-3-3, so no unit is bound to it.

### [`RFC7611-3-4`](#rfc7611-3-4)

ACCEPT_OWN community MUST be removed before re-advertising to eBGP peers (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7611-3-4, so no unit is bound to it.

### [`RFC7611-3-5`](#rfc7611-3-5)

Route reflectors MUST propagate the ACCEPT_OWN community unchanged (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7611-3-5, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7611, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7611, so its obligations are stated where they were written.
