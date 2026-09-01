# RFC 8097 - BGP Prefix Origin Validation State Extended Community

No row in the public ledger. Every requirement this repository extracted from RFC 8097, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 5 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
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
| Requirements | 10 |
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
| Summary | `rfc/short/rfc8097.md` |
| Requirement shard | `rfc/requirements/rfc8097.md` |
| RFC text | `rfc/full/rfc8097.txt` |

## Enrolment

Enrolled: BGP Prefix Origin Validation State Extended Community: five MUST-level requirements, all {not-applicable} to Ze. Ze performs local RFC 6811 origin validation (internal/component/bgp/plugins/rpki/validate.go) but does not implement the RFC 8097 origin-validation-state extended community -- it has no encoder/decoder for the type-0x43 community and treats every extended community as a generic opaque 8-octet value (internal/core/bgp/attribute/community.go:231 ExtendedCommunity [8]byte, community.go:275 ParseExtendedCommunities). RFC8097-2-1 (Reserved field 0 on transmission) has no encoder; RFC8097-2-2 (ignore Reserved on receipt) and RFC8097-2-5 (RFC 7606 discard if state > 2) have no field/state decoding; RFC8097-2-3 (disregard all but greatest validation state) has no multi-instance selection; RFC8097-2-4 (drop from eBGP without processing) has no OV-state recognition. The 2-6..2-10 SHOULD/SHOULD-NOT clauses are not gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 8097.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (5):** [`RFC8097-2-1`](#rfc8097-2-1), [`RFC8097-2-2`](#rfc8097-2-2), [`RFC8097-2-3`](#rfc8097-2-3), [`RFC8097-2-4`](#rfc8097-2-4), [`RFC8097-2-5`](#rfc8097-2-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8097-2-1` | Reserved field (bytes 2-6) must be set to 0 on transmission (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 8097 BGP Prefix Origin Validation State Extended Community. It performs local RFC 6811 origin validation (internal/component/bgp/plugins/rpki/validate.go) but has no encoder for the origin-validation-state extended community (type 0x43); its extended-community codec is a generic opaque 8-octet carrier (internal/core/bgp/attribute/community.go:231 ExtendedCommunity [8]byte), so Ze never originates this community and has no Reserved field to zero. |
| `RFC8097-2-2` | Reserved field must be ignored upon receipt (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not decode the origin-validation-state extended community; ParseExtendedCommunities (internal/core/bgp/attribute/community.go:275) retains every extended community as an opaque 8-octet value and never interprets bytes 2-6 as a reserved field, so there is no RFC 8097 Reserved field for Ze to ignore. |
| `RFC8097-2-3` | Disregard all instances other than the one with the numerically greatest validation state value when multiple instances received (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not process the origin-validation-state extended community (no type-0x43 decoder; community.go:275 keeps ext-communities opaque), so there is no validation-state value to compare across instances and no greatest-state selection code path. |
| `RFC8097-2-4` | Drop the origin validation state extended community if received from an EBGP peer, without processing it further (default behavior) (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not recognize or process the origin-validation-state extended community (no type-0x43 handling; community.go:275), so it applies no RFC 8097 semantics -- there is no origin-validation-state community for Ze to drop-vs-process based on the eBGP/iBGP peer type. |
| `RFC8097-2-5` | Apply attribute discard strategy per RFC 7606 if validation state value > 2, discarding the erroneous community and logging the error (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not decode the origin-validation-state extended community's validation-state octet (community.go:275 keeps the 8-octet value opaque and never reads it as a validation state), so it has no code path that could observe a state value > 2 to apply the RFC 7606 discard against. |
| `RFC8097-2-6` | Attach the origin validation state extended community to BGP UPDATE messages sent to iBGP peers when configured to support this extension (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-7` | Derive validation state from the extended community in absence of locally-computed validation state (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-8` | Send more than one instance of the origin validation state extended community (§2) | SHOULD NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-9` | Send the community to EBGP peers by default (§2) | SHOULD NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-10` | Be configurable to send or accept the community to/from EBGP peers when warranted (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8097-2-1`](#rfc8097-2-1) Reserved field (bytes 2-6) must be set to 0 on transmission (§2) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 8097 BGP Prefix Origin Validation State Extended Community. It performs local RFC 6811 origin validation (internal/component/bgp/plugins/rpki/validate.go) but has no encoder for the origin-validation-state extended community (type 0x43); its extended-community codec is a generic opaque 8-octet carrier (internal/core/bgp/attribute/community.go:231 ExtendedCommunity [8]byte), so Ze never originates this community and has no Reserved field to zero. |
| [`RFC8097-2-2`](#rfc8097-2-2) Reserved field must be ignored upon receipt (§2) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not decode the origin-validation-state extended community; ParseExtendedCommunities (internal/core/bgp/attribute/community.go:275) retains every extended community as an opaque 8-octet value and never interprets bytes 2-6 as a reserved field, so there is no RFC 8097 Reserved field for Ze to ignore. |
| [`RFC8097-2-3`](#rfc8097-2-3) Disregard all instances other than the one with the numerically greatest validation state value when multiple instances received (§2) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not process the origin-validation-state extended community (no type-0x43 decoder; community.go:275 keeps ext-communities opaque), so there is no validation-state value to compare across instances and no greatest-state selection code path. |
| [`RFC8097-2-4`](#rfc8097-2-4) Drop the origin validation state extended community if received from an EBGP peer, without processing it further (default behavior) (§2) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not recognize or process the origin-validation-state extended community (no type-0x43 handling; community.go:275), so it applies no RFC 8097 semantics -- there is no origin-validation-state community for Ze to drop-vs-process based on the eBGP/iBGP peer type. |
| [`RFC8097-2-5`](#rfc8097-2-5) Apply attribute discard strategy per RFC 7606 if validation state value > 2, discarding the erroneous community and logging the error (§2) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not decode the origin-validation-state extended community's validation-state octet (community.go:275 keeps the 8-octet value opaque and never reads it as a validation state), so it has no code path that could observe a state value > 2 to apply the RFC 7606 discard against. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8097-2-1`](#rfc8097-2-1)

Reserved field (bytes 2-6) must be set to 0 on transmission (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8097-2-1, so no unit is bound to it.

### [`RFC8097-2-2`](#rfc8097-2-2)

Reserved field must be ignored upon receipt (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8097-2-2, so no unit is bound to it.

### [`RFC8097-2-3`](#rfc8097-2-3)

Disregard all instances other than the one with the numerically greatest validation state value when multiple instances received (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8097-2-3, so no unit is bound to it.

### [`RFC8097-2-4`](#rfc8097-2-4)

Drop the origin validation state extended community if received from an EBGP peer, without processing it further (default behavior) (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8097-2-4, so no unit is bound to it.

### [`RFC8097-2-5`](#rfc8097-2-5)

Apply attribute discard strategy per RFC 7606 if validation state value > 2, discarding the erroneous community and logging the error (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8097-2-5, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 8097, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8097, so its obligations are stated where they were written.
