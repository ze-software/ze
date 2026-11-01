# RFC 8097 - BGP Prefix Origin Validation State Extended Community

RPKI origin validation state extended community. Every requirement this repository extracted from RFC 8097, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 5 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 5 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 5 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 5 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 5 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 5 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 5 | of 5 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 5 gated MUSTs: they add up to 5. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | RPKI origin validation state extended community |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
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

Enrolled: BGP Prefix Origin Validation State Extended Community: five MUST-level requirements, all gaps. Ze performs local RFC 6811 origin validation (internal/component/bgp/plugins/rpki/validate.go) but does not implement the RFC 8097 origin validation state extended community: it has no encoder or decoder for the type-0x43 community and treats every extended community as an opaque 8-octet value (internal/core/bgp/attribute/community.go, ExtendedCommunity and ParseExtendedCommunities). An operator can still configure one in the 0x hex form with any Reserved value, and one received from an EBGP peer is readvertised unchanged rather than dropped (RFC8097-2-4). No owner decision declines the community, so each row is owed.

## What the public ledger says

**Area:** RPKI origin validation state extended community

**Proof:** 0 of 5 MUST proven

**Evidence:** -

**Unmet:** RFC8097-2-1, RFC8097-2-2, RFC8097-2-3, RFC8097-2-4, RFC8097-2-5


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (5):** [`RFC8097-2-1`](#rfc8097-2-1), [`RFC8097-2-2`](#rfc8097-2-2), [`RFC8097-2-3`](#rfc8097-2-3), [`RFC8097-2-4`](#rfc8097-2-4), [`RFC8097-2-5`](#rfc8097-2-5)

## Figures

- **MUST bar**: Ze correctness gap 5
- **Overview bar**: SHOULD not decided 5, Ze correctness gap 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 5 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 5 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc8097.json](../../../data/rfc/rfc8097.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8097-2-1` | The Reserved field MUST be set to 0 (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never builds the community itself, but an operator can configure the community in the 0x hex form with any value (internal/core/bgp/attribute/extcomm_hex.go::ParseExtendedCommunityHex interprets no type octets), so nothing refuses a non-zero Reserved field. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| `RFC8097-2-2` | The Reserved field MUST be set to 0 and ignored upon the receipt of this community. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no Reserved field of the community. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| `RFC8097-2-3` | However, if more than one instance is received, an implementation MUST disregard all instances other than the one with the numerically greatest validation state value. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze compares no validation state across instances. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| `RFC8097-2-4` | By default, implementations MUST drop the origin validation state extended community if received from an External BGP (EBGP) peer, without processing it further. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a community received from an EBGP peer is kept and readvertised unchanged, to IBGP peers included (internal/core/bgp/attribute/community.go::ExtendedCommunities.WriteTo copies it back out), so the default drop does not happen. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| `RFC8097-2-5` | If the value received is greater than the largest specified value (2), the implementation MUST apply a strategy similar to attribute discard [RFC7606] by discarding the erroneous community and logging the error for further analysis. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no validation state value, so a value above 2 is neither discarded nor logged. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| `RFC8097-2-6` | If the router is configured to support the extensions defined in this document, it SHOULD attach the origin validation state extended community to BGP UPDATE messages sent to IBGP peers by mapping the computed validation state in the last octet of the extended community. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-7` | Similarly, a receiving BGP speaker, in the absence of validation state set based on local data, SHOULD derive a validation state from the last octet of the extended community, if present. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-8` | An implementation SHOULD NOT send more than one instance of the origin validation state extended community. (§2) | SHOULD NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-9` | Similarly, by default, an implementation SHOULD NOT send the community to EBGP peers. (§2) | SHOULD NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-10` | However, it SHOULD be possible to configure an implementation to send or accept the community when warranted. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8097-2-1`](#rfc8097-2-1) The Reserved field MUST be set to 0 (§2) | {gap}, no test | ze never builds the community itself, but an operator can configure the community in the 0x hex form with any value (internal/core/bgp/attribute/extcomm_hex.go::ParseExtendedCommunityHex interprets no type octets), so nothing refuses a non-zero Reserved field. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| [`RFC8097-2-2`](#rfc8097-2-2) The Reserved field MUST be set to 0 and ignored upon the receipt of this community. (§2) | {gap}, no test | ze reads no Reserved field of the community. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| [`RFC8097-2-3`](#rfc8097-2-3) However, if more than one instance is received, an implementation MUST disregard all instances other than the one with the numerically greatest validation state value. (§2) | {gap}, no test | ze compares no validation state across instances. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| [`RFC8097-2-4`](#rfc8097-2-4) By default, implementations MUST drop the origin validation state extended community if received from an External BGP (EBGP) peer, without processing it further. (§2) | {gap}, no test | a community received from an EBGP peer is kept and readvertised unchanged, to IBGP peers included (internal/core/bgp/attribute/community.go::ExtendedCommunities.WriteTo copies it back out), so the default drop does not happen. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| [`RFC8097-2-5`](#rfc8097-2-5) If the value received is greater than the largest specified value (2), the implementation MUST apply a strategy similar to attribute discard [RFC7606] by discarding the erroneous community and logging the error for further analysis. (§2) | {gap}, no test | ze reads no validation state value, so a value above 2 is neither discarded nor logged. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8097-2-1`](#rfc8097-2-1)

The Reserved field MUST be set to 0 (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8097-2-1, so no unit is bound to it.

### [`RFC8097-2-2`](#rfc8097-2-2)

The Reserved field MUST be set to 0 and ignored upon the receipt of this community. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8097-2-2, so no unit is bound to it.

### [`RFC8097-2-3`](#rfc8097-2-3)

However, if more than one instance is received, an implementation MUST disregard all instances other than the one with the numerically greatest validation state value. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8097-2-3, so no unit is bound to it.

### [`RFC8097-2-4`](#rfc8097-2-4)

By default, implementations MUST drop the origin validation state extended community if received from an External BGP (EBGP) peer, without processing it further. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8097-2-4, so no unit is bound to it.

### [`RFC8097-2-5`](#rfc8097-2-5)

If the value received is greater than the largest specified value (2), the implementation MUST apply a strategy similar to attribute discard [RFC7606] by discarding the erroneous community and logging the error for further analysis. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8097-2-5, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc8097.txt |
| Source fingerprint | 0e52119a0d75abc0 |
| Record | rfc/extraction/rfc8097.json |
| Mapped sentences | 4 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 4 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust copyright boilerplate in the Status of This Memo section: it binds the extraction of code components from the document, not a BGP speaker, and the keyword is lowercase 'must'. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |

## Superseded

No document obsoletes RFC 8097, so its obligations are stated where they were written.
