# RFC 4360 - BGP Extended Communities Attribute

Supported. Every requirement this repository extracted from RFC 4360, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 2 of 2 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 2 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 2 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 2 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 2 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 2 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4360.md` |
| Requirement shard | `rfc/requirements/rfc4360.md` |
| RFC text | `rfc/full/rfc4360.txt` |

## Enrolment

Enrolled: BGP Extended Communities: six MUST-level requirements. Two are tested with both polarities over the codec (internal/core/bgp/attribute/community.go): RFC4360-2-1 (equal only when all 8 octets match) via ExtendedCommunity [8]byte equality, RFC4360-x-1 (length a multiple of 8) via ParseExtendedCommunities (wired at wire.go:412). Four are {not-applicable}: RFC4360-6-1 (best-path/forwarding-loop) since ze's best-path (rib/bestpath.go) never reads Extended Communities; RFC4360-7-1 and RFC4360-7-2 are IANA type-registry allocation rules, not implementation behavior; RFC4360-x-2 governs a non-supporting speaker's pass-through, whereas ze supports the attribute. The 6-2 SHOULD, 6-3 SHOULD NOT and 6-4/6-5 MAYs are not gated.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Extended community attribute parsing, encoding, JSON, and policy use.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC4360-2-1`](#rfc4360-2-1), [`RFC4360-x-1`](#rfc4360-x-1)

**Annotated instead of tested (4):** [`RFC4360-6-1`](#rfc4360-6-1), [`RFC4360-7-1`](#rfc4360-7-1), [`RFC4360-7-2`](#rfc4360-7-2), [`RFC4360-x-2`](#rfc4360-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4360-6-1` | The Extended Communities attribute MUST NOT be used to modify the BGP best path selection algorithm in a way that leads to forwarding loops (§6) | MUST NOT | 6 - Operations | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's best-path decision (internal/component/bgp/plugins/rib/bestpath.go) selects on the RFC 4271 tie-breakers only -- LOCAL_PREF, AS_PATH length, ORIGIN, MED, ..., Router ID -- and never reads the Extended Communities attribute, so there is no ext-comm-driven path selection that could create a forwarding loop |
| `RFC4360-7-1` | The value allocated for a regular Type MUST NOT be reused as the high-order octet when allocating an extended Type (§7) | MUST NOT | 7 - IANA Considerations | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this constrains the IANA Extended Community type-code registry allocation, not a BGP implementation; ze consumes registry type codes and does not allocate them |
| `RFC4360-7-2` | The value of the high-order octet allocated for an extended Type MUST NOT be reused when allocating a regular Type (§7) | MUST NOT | 7 - IANA Considerations | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** same as 7-1 -- an IANA type-allocation rule on the registry, not implementation behavior; ze does not allocate Extended Community type codes |
| `RFC4360-2-1` | Two extended communities are equal only when all 8 octets are equal (§2) | MUST | 2 - BGP Extended Communities Attribute | **positive:** `unit/verify` [`TestExtendedCommunityEquality`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L297). **negative:** `unit/verify` [`TestExtendedCommunityEquality`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L299) |
| `RFC4360-x-1` | Attribute length MUST be a multiple of 8 octets (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestExtendedCommunitiesParse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L277). **negative:** `unit/verify` [`TestExtendedCommunitiesParseRejectsBadLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L287) |
| `RFC4360-x-2` | Non-supporting peers MUST pass the attribute unchanged (RFC 4271 behavior for optional transitive) (Compatibility) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this governs a speaker that does NOT support Extended Communities passing the optional-transitive attribute through unchanged (RFC 4271); ze supports the attribute -- it parses and re-encodes it, ExtendedCommunities.Flags is optional-transitive (internal/core/bgp/attribute/community.go:244) -- so ze is a supporting speaker, and the general RFC 4271 optional-transitive pass-through is tracked under RFC 4271/7606 |
| `RFC4360-6-2` | Non-transitive extended communities SHOULD be removed before advertising the route across the AS boundary (§6) | SHOULD | 6 - Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC4360-6-3` | Non-transitive extended communities SHOULD NOT be removed when advertising the route across the BGP Confederation boundary (§6) | SHOULD NOT | 6 - Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC4360-6-4` | A BGP speaker receiving a route without Extended Communities MAY append this attribute when propagating (§6) | MAY | 6 - Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC4360-6-5` | A BGP speaker receiving a route with Extended Communities MAY modify this attribute according to local policy (§6) | MAY | 6 - Operations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4360-6-1`](#rfc4360-6-1) The Extended Communities attribute MUST NOT be used to modify the BGP best path selection algorithm in a way that leads to forwarding loops (§6) | no test | no test carries this requirement id; annotated {not-applicable}: ze's best-path decision (internal/component/bgp/plugins/rib/bestpath.go) selects on the RFC 4271 tie-breakers only -- LOCAL_PREF, AS_PATH length, ORIGIN, MED, ..., Router ID -- and never reads the Extended Communities attribute, so there is no ext-comm-driven path selection that could create a forwarding loop |
| [`RFC4360-7-1`](#rfc4360-7-1) The value allocated for a regular Type MUST NOT be reused as the high-order octet when allocating an extended Type (§7) | no test | no test carries this requirement id; annotated {not-applicable}: this constrains the IANA Extended Community type-code registry allocation, not a BGP implementation; ze consumes registry type codes and does not allocate them |
| [`RFC4360-7-2`](#rfc4360-7-2) The value of the high-order octet allocated for an extended Type MUST NOT be reused when allocating a regular Type (§7) | no test | no test carries this requirement id; annotated {not-applicable}: same as 7-1 -- an IANA type-allocation rule on the registry, not implementation behavior; ze does not allocate Extended Community type codes |
| [`RFC4360-x-2`](#rfc4360-x-2) Non-supporting peers MUST pass the attribute unchanged (RFC 4271 behavior for optional transitive) (Compatibility) | no test | no test carries this requirement id; annotated {not-applicable}: this governs a speaker that does NOT support Extended Communities passing the optional-transitive attribute through unchanged (RFC 4271); ze supports the attribute -- it parses and re-encodes it, ExtendedCommunities.Flags is optional-transitive (internal/core/bgp/attribute/community.go:244) -- so ze is a supporting speaker, and the general RFC 4271 optional-transitive pass-through is tracked under RFC 4271/7606 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4360-6-1`](#rfc4360-6-1)

The Extended Communities attribute MUST NOT be used to modify the BGP best path selection algorithm in a way that leads to forwarding loops (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4360-6-1, so no unit is bound to it.

### [`RFC4360-7-1`](#rfc4360-7-1)

The value allocated for a regular Type MUST NOT be reused as the high-order octet when allocating an extended Type (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4360-7-1, so no unit is bound to it.

### [`RFC4360-7-2`](#rfc4360-7-2)

The value of the high-order octet allocated for an extended Type MUST NOT be reused when allocating a regular Type (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4360-7-2, so no unit is bound to it.

### [`RFC4360-2-1`](#rfc4360-2-1)

Two extended communities are equal only when all 8 octets are equal (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtendedCommunityEquality`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L299) | unit/verify | unproven |
| positive | [`TestExtendedCommunityEquality`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L297) | unit/verify | unproven |

### [`RFC4360-x-1`](#rfc4360-x-1)

Attribute length MUST be a multiple of 8 octets (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtendedCommunitiesParseRejectsBadLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L287) | unit/verify | unproven |
| positive | [`TestExtendedCommunitiesParse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L277) | unit/verify | unproven |

### [`RFC4360-x-2`](#rfc4360-x-2)

Non-supporting peers MUST pass the attribute unchanged (RFC 4271 behavior for optional transitive) (Compatibility)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4360-x-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc4360 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc4360.txt |
| Source fingerprint | 759f302ace542b9f |
| Record | rfc/extraction/rfc4360.json |
| Mapped sentences | 3 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice and Abstract. The Abstract restates section 1: the attribute labels information carried in BGP-4 and the labels can control its distribution. No sentence directs a speaker. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: the two enhancements over RFC 1997 (an extended range and a Type field), and what structure buys a policy writer. No directive. |
| `1.1` | Specification of Requirements | 0 | walked | Specification of Requirements. The RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. Unlike RFC 7313 section 2 it does not add the 'only when in all upper case' sentence, so this walk reads a lowercase modal on its own merits rather than as a key word. |
| `2` | BGP Extended Communities Attribute | 0 | walked | BGP Extended Communities Attribute. The document's wire-format section, written entirely in the indicative, so the site scan sees nothing in it and three declared MUST-level rows are read from here. It states the attribute is transitive optional with Type Code 16, that each extended community is an 8-octet quantity, that the Type Field is 1 octet for a Regular type and 2 for an Extended type, that the high-order octet carries the I (IANA authority) and T (transitive) bits with T=0 transitive and T=1 non-transitive across ASes, and that 'Two extended communities are declared equal only when all 8 octets of the community are equal.' The three unsourced ids below are those obligations. The remaining sentences are value definitions carried by the Wire Formats and Type Classification tables of rfc/short/rfc4360.md, and the closing 'The two members in the tuple <Type, Value> should be enumerated to specify any community value' is a lowercase modal describing how a value is written down, not a directive to a speaker. |
| `3` | Defined BGP Extended Community Types | 0 | walked | Defined BGP Extended Community Types. One paragraph naming what 3.1 to 3.3 define: templates identified by the high-order octet, with the low-order octet as sub-type. No directive. |
| `3.1` | Two-Octet AS Specific Extended Community | 0 | walked | Two-Octet AS Specific Extended Community. Value assignment: high-order octet 0x00 or 0x40, a 2-octet Global Administrator holding an IANA-assigned AS number and a 4-octet Local Administrator. Its one modal, 'The format and meaning of the value encoded in this sub-field should be defined by the sub-type of the community', is lowercase and binds whoever defines a future sub-type, not a speaker encoding one. The layout is carried by the Two-Octet AS Specific table of rfc/short/rfc4360.md. |
| `3.2` | IPv4 Address Specific Extended Community | 0 | walked | IPv4 Address Specific Extended Community. Value assignment: high-order octet 0x01 or 0x41, a 4-octet Global Administrator holding a registry-assigned IPv4 address and a 2-octet Local Administrator. Its lowercase 'should be defined by the sub-type' sentence binds a future sub-type definition, as in 3.1. The layout is carried by the IPv4 Address Specific table of rfc/short/rfc4360.md. |
| `3.3` | Opaque Extended Community | 0 | walked | Opaque Extended Community. Value assignment: high-order octet 0x03 or 0x43 and a 6-octet opaque Value. States that the sub-type defining the Value Field is to be assigned by IANA, which is an allocation fact rather than a directive to a speaker. The layout is carried by the Opaque Extended Community table of rfc/short/rfc4360.md. |
| `4` | Route Target Community | 0 | walked | Route Target Community. Value assignment: an extended type whose high-order octet is 0x00, 0x01 or 0x02 and whose low-order octet is 0x02, transitive across the AS boundary, with the Local Administrator drawn from the numbering space of the organization holding the AS number or IP address in the Global Administrator sub-field. Every sentence is indicative, and the use is deferred to RFC 4364. Carried by the Sub-Types table of rfc/short/rfc4360.md. |
| `5` | Route Origin Community | 0 | walked | Route Origin Community. The same shape as section 4 with low-order octet 0x03: an extended type, transitive across the AS boundary, Local Administrator drawn from the holder's numbering space, use deferred to RFC 4364. Indicative throughout. Carried by the Sub-Types table of rfc/short/rfc4360.md. |
| `6` | Operations | 1 | walked | Operations. The only section that directs a BGP speaker. Its one MUST-level site is 6:1, mapped below to RFC4360-6-1. Its four remaining directives are the two MAYs (append the attribute to a route that lacks it, modify the attribute per local policy) and the SHOULD and SHOULD NOT that bracket a non-transitive community at an AS boundary against a Confederation boundary; those are the four unsourced ids below, advisory and never gated. Two further sentences the site scan does not see are not obligations: the aggregation paragraph states a lowercase-'should' DEFAULT that the same paragraph says 'could be overridden via local configuration', and the closing paragraph is indicative division of labour, saying a route may carry both attributes and that the RFC 1997 one is handled per RFC 1997. |
| `7` | IANA Considerations | 3 | walked | IANA Considerations. Walked rather than skipped because two of its three sites are MUST-level sentences the summary declares as rows (RFC4360-7-1 and RFC4360-7-2), so an `iana` skip would hide two gated ids behind a skip kind. Both are mapped below, and both bind the registry: rfc/short/rfc4360.md records that as their {not-applicable} annotation, which is the summary's judgement and not this walk's. The rest of the section creates the 'BGP Extended Communities Type' registry and the three per-class registries, fixes the Standards Action, Early IANA Allocation, First Come First Served and Experimental ranges, and assigns 0x0002, 0x0003, 0x0102 and 0x0103. Those ranges and values are carried by the Type Classification and IANA Registered Type Values tables of rfc/short/rfc4360.md. |
| `8` | Security Considerations | 1 | walked | Security Considerations. States that the extension has security implications similar to RFC 1997 and changes no underlying security issue, then places one condition on the operator (site 8:1), and puts the mechanism providing that trust relationship out of scope. No countermeasure is directed at a speaker. |
| `9` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `10` | not stated | 0 | skipped (references) | Normative References: RFC 4271, RFC 1997, RFC 2119, RFC 2434, RFC 4020. |
| `11` | Informative References: RFC 4364 | 1 | skipped (references) | Informative References: RFC 4364. The derived section span runs to the end of the document, so it also holds Authors' Addresses, the Full Copyright Statement, the Intellectual Property boilerplate and the RFC Editor funding note. Site 11:1 is one sentence of that IPR boilerplate and is excluded below; nothing in the span states an obligation on a BGP speaker. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `7:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the party filing a future Type value assignment request with IANA, and IANA which records the answer: the sentence directs what such a REQUEST must specify, namely whether the value is for a transitive or a non-transitive Extended Community. Ze consumes allocated type codes and files no assignment request, so it never plays the requester role. The lowercase 'must' is why the site scan sees it only under the prose register. The role is a registry act by a person, so no producer could perform it. Ze CONSUMES the resulting assignment: the extended community codec reads the type octet it is given (`internal/component/bgp/attribute`). | Future requests for assignment of a Type value must specify whether the Type value is intended for a transitive or a non- transitive Extended Community. |
| `8:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the network operator: the sentence conditions an operator who RELIES on information carried in BGP on having a transitive trust relationship back to the source, and the next sentence puts the mechanism providing that relationship beyond the scope of the document. It directs no encoding, decoding or propagation behavior a BGP speaker performs, and there is nothing for a daemon to implement. The role is the AS operator, so no producer could act as it. Ze CONSUMES the operator's decision: the capability is negotiated per session in the reactor (`internal/component/bgp/reactor`), which advertises what it is configured to and decides no AS-wide policy. | Specifically, an operator who is relying on the information carried in BGP must have a transitive trust relationship back to the source of the information. |
| `11:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: this is the IETF's standard Intellectual Property boilerplate, which the derived section span carries into section 11 after the Informative References. The sentence invites interested parties to disclose patent rights to the IETF, and its 'may be required to implement this standard' describes what a patent might cover rather than requiring anything of an implementation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4360, so its obligations are stated where they were written.
