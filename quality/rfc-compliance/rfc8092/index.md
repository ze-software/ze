# RFC 8092 - BGP Large Communities Attribute

Supported. Every requirement this repository extracted from RFC 8092, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 57.1% | 4 of 7 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 42.9% | 3 of 7 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 7 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 7 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 11 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 7 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 11 |
| Gated MUST-level | 7 |
| Obligations that bind Ze | 7 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 11 |
| Tagged units | 11 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8092.md` |
| Requirement shard | `rfc/requirements/rfc8092.md` |
| RFC text | `rfc/full/rfc8092.txt` |

## Enrolment

Enrolled: BGP Large Communities: seven MUST-level requirements over the LARGE_COMMUNITY codec (internal/core/bgp/attribute/community.go) and its RFC 7606 validator (internal/component/bgp/message/rfc7606.go:620). Both polarities: RFC8092-3-1 (duplicates not transmitted) via WriteTo/unique, RFC8092-3-2 (receiver silently removes redundant) via ParseLargeCommunities/deduplicate, RFC8092-6-2 (malformed if length not a non-zero multiple of 12) and RFC8092-6-4 (malformed -> treat-as-withdraw) via validateLargeCommunityAttr driven from ValidateUpdateRFC7606. Three {single-polarity: positive}: RFC8092-5-1 (canonical, no leading zeros) since strconv.AppendUint never emits them; RFC8092-6-1 (reserved ASN not malformed) since the validator inspects length only; RFC8092-6-3 (duplicates not malformed) since the parser deduplicates rather than rejecting. The 3-3/5-2/4-1 SHOULDs and 3-4 NOT RECOMMENDED are not gated.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

LARGE_COMMUNITY parsing, validation, duplicate removal, JSON, and RFC 7606 length checks.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC8092-3-1`](#rfc8092-3-1), [`RFC8092-3-2`](#rfc8092-3-2), [`RFC8092-6-2`](#rfc8092-6-2), [`RFC8092-6-4`](#rfc8092-6-4)

**Annotated instead of tested (3):** [`RFC8092-5-1`](#rfc8092-5-1), [`RFC8092-6-1`](#rfc8092-6-1), [`RFC8092-6-3`](#rfc8092-6-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8092-3-1` | Duplicate BGP Large Community values must not be transmitted (Section 3) | MUST NOT | 3 - BGP Large Communities Attribute | **positive:** `unit/verify` [`TestLargeCommunitiesWriteToNoDuplicates`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L430). **negative:** `unit/verify` [`TestLargeCommunities`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L221) |
| `RFC8092-3-2` | A receiving speaker must silently remove redundant BGP Large Community values (Section 3) | MUST | 3 - BGP Large Communities Attribute | **positive:** `unit/verify` [`TestLargeCommunitiesDeduplication`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L403). **negative:** `unit/verify` [`TestLargeCommunitiesParse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L245) |
| `RFC8092-5-1` | Canonical representation numbers must not contain leading zeros; zero must be represented as a single "0" (Section 5) | MUST NOT | 5 - Canonical Representation | **positive:** `unit/verify` [`TestAppendText_LargeCommunityElement`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/text_append_test.go#L39). **negative:** no negative test. **{single-polarity}:** the canonical rendering LargeCommunity.AppendText (internal/core/bgp/attribute/community.go:316) uses strconv.AppendUint, which never emits leading zeros and renders 0 as a single "0", so every value is canonical and there is no non-canonical rendering to assert as a negative |
| `RFC8092-6-1` | BGP Large Communities attribute must not be considered malformed if Global Administrator contains an unallocated, unassigned, or reserved ASN (Section 6) | MUST NOT | 6 - Error Handling | **positive:** `unit/verify` [`TestRFC8092LargeCommunityValidReservedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8092_test.go#L41). **negative:** no negative test. **{single-polarity}:** the malformed check validateLargeCommunityAttr (internal/component/bgp/message/rfc7606.go:620) inspects the attribute length only and never the Global Administrator value, so a reserved or unallocated ASN is never treated as malformed and there is no ASN-based rejection to assert as a negative |
| `RFC8092-6-2` | Attribute shall be considered malformed if length is not a non-zero multiple of 12 octets (Section 6) | SHALL | 6 - Error Handling | **positive:** `unit/verify` [`TestRFC8092LargeCommunityMalformedLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8092_test.go#L17). **negative:** `unit/verify` [`TestRFC8092LargeCommunityValidReservedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8092_test.go#L37) |
| `RFC8092-6-3` | Attribute shall not be considered malformed due to presence of duplicate Large Community values (Section 6) | SHALL NOT | 6 - Error Handling | **positive:** `unit/verify` [`TestLargeCommunitiesDeduplication`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L406). **negative:** no negative test. **{single-polarity}:** ParseLargeCommunities silently deduplicates and returns no error on duplicates (internal/core/bgp/attribute/community.go:422), and validateLargeCommunityAttr checks only length, so a duplicate is never treated as malformed and there is no duplicate-based rejection to assert as a negative |
| `RFC8092-6-4` | A BGP UPDATE with malformed BGP Large Communities attribute shall be handled using "treat-as-withdraw" per RFC 7606 (Section 6) | SHALL | 6 - Error Handling | **positive:** `unit/verify` [`TestRFC8092LargeCommunityMalformedLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8092_test.go#L19). **negative:** `unit/verify` [`TestRFC8092LargeCommunityValidReservedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8092_test.go#L39) |
| `RFC8092-3-3` | Global Administrator field should be an ASN (Section 3) | SHOULD | 3 - BGP Large Communities Attribute | **positive:** no positive test. **negative:** no negative test |
| `RFC8092-5-2` | BGP Large Communities should be represented in the canonical representation (Section 5) | SHOULD | 5 - Canonical Representation | **positive:** no positive test. **negative:** no negative test |
| `RFC8092-4-1` | Aggregated routes should contain the union of all BGP Large Communities from all aggregated routes (Section 4) | SHOULD | 4 - Aggregation | **positive:** no positive test. **negative:** no negative test |
| `RFC8092-3-4` | Use of reserved ASNs (0, 65535, 4294967295) in Global Administrator is not recommended (Section 3) | NOT RECOMMENDED | 3 - BGP Large Communities Attribute | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8092 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8092-3-1`](#rfc8092-3-1)

Duplicate BGP Large Community values must not be transmitted (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLargeCommunities`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L221) | unit/verify | unproven |
| positive | [`TestLargeCommunitiesWriteToNoDuplicates`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L430) | unit/verify | unproven |

### [`RFC8092-3-2`](#rfc8092-3-2)

A receiving speaker must silently remove redundant BGP Large Community values (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLargeCommunitiesParse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L245) | unit/verify | unproven |
| positive | [`TestLargeCommunitiesDeduplication`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L403) | unit/verify | unproven |

### [`RFC8092-5-1`](#rfc8092-5-1)

Canonical representation numbers must not contain leading zeros; zero must be represented as a single "0" (Section 5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestAppendText_LargeCommunityElement`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/text_append_test.go#L39) | unit/verify | unproven |

### [`RFC8092-6-1`](#rfc8092-6-1)

BGP Large Communities attribute must not be considered malformed if Global Administrator contains an unallocated, unassigned, or reserved ASN (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8092LargeCommunityValidReservedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8092_test.go#L41) | unit/verify | unproven |

### [`RFC8092-6-2`](#rfc8092-6-2)

Attribute shall be considered malformed if length is not a non-zero multiple of 12 octets (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8092LargeCommunityValidReservedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8092_test.go#L37) | unit/verify | unproven |
| positive | [`TestRFC8092LargeCommunityMalformedLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8092_test.go#L17) | unit/verify | unproven |

### [`RFC8092-6-3`](#rfc8092-6-3)

Attribute shall not be considered malformed due to presence of duplicate Large Community values (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLargeCommunitiesDeduplication`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L406) | unit/verify | unproven |

### [`RFC8092-6-4`](#rfc8092-6-4)

A BGP UPDATE with malformed BGP Large Communities attribute shall be handled using "treat-as-withdraw" per RFC 7606 (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8092LargeCommunityValidReservedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8092_test.go#L39) | unit/verify | unproven |
| positive | [`TestRFC8092LargeCommunityMalformedLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8092_test.go#L19) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc8092 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc8092.txt |
| Source fingerprint | e374b17f52bc7261 |
| Record | rfc/extraction/rfc8092.json |
| Mapped sentences | 7 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates the Introduction: the attribute is an extension to BGP-4 that signals opaque information within separate namespaces, and it suits all ASNs including four-octet ASNs. The Copyright Notice's lowercase 'Code Components extracted from this document must include Simplified BSD License text' binds whoever reuses the document's code components, not a BGP speaker. |
| `1` | Introduction | 0 | walked | Introduction. Indicative throughout: network operators attach communities to routes, an RFC 1997 community is a four-octet value whose halves are read as a two-octet ASN and a locally defined value, four-octet ASNs (RFC 6793) no longer fit that encoding, and the six-octet RFC 4360 Extended Community value cannot hold a four-octet ASN in both the Global Administrator and the Local Administrator sub-fields. The closing paragraph states what this document defines: an unordered set of one or more twelve-octet values, each a four-octet Global Administrator and two four-octet operator-defined fields. No sentence directs a speaker. |
| `2` | Requirements Language | 0 | walked | Requirements Language. The RFC 2119 key-words paragraph, which tells a reader how to read the other sections and binds no speaker; the derivation excludes it from the site inventory. It is the RFC 2119 form alone: unlike a post-RFC 8174 document it does not add the 'only when, and in all capitals' sentence, so this walk reads a lowercase modal elsewhere in the document on its own merits rather than as a key word. |
| `3` | BGP Large Communities Attribute | 2 | walked | BGP Large Communities Attribute. Two MUST-level sites, 3:1 and 3:2, the transmit and receive halves of the duplicate rule, both mapped below. The rest of the section is definitional or advisory. Definitional: the attribute is an optional transitive path attribute of variable length, all routes carrying it belong to the communities it specifies, each value is a 12-octet quantity laid out as a four-octet Global Administrator followed by Local Data Part 1 and Local Data Part 2, and 'There is no significance to the order in which twelve-octet Large Community Attribute values are encoded'. Those are indicative statements carried by the Wire Formats and Encoding Rules of rfc/short/rfc8092.md; none is written as an obligation, so none earns a checklist row. Advisory: 'This field SHOULD be an ASN' and 'The use of Reserved ASNs (0 [RFC7607], 65535 and 4294967295 [RFC7300]) is NOT RECOMMENDED'. Both carry capitalised keywords that are not MUST-level, so the rfc2119 site inventory does not count them; they are declared RFC8092-3-3 and RFC8092-3-4 and are listed unsourced here. |
| `4` | Aggregation | 0 | walked | Aggregation. One sentence, and its modal is lowercase: 'If a range of routes is aggregated, then the resulting aggregate should have a BGP Large Communities attribute that contains all of the BGP Large Communities attributes from all of the aggregated routes.' Under the rfc2119 register the site scan counts capitalised MUST-level keywords only, so it sees nothing here. The summary declares the sentence as the advisory row RFC8092-4-1, listed unsourced below. |
| `5` | Canonical Representation | 1 | walked | Canonical Representation. Defines the three-decimal colon form, in the order Global Administrator, Local Data 1, Local Data 2, and gives 64496:4294967295:2 and 64496:0:0 as examples. Its one MUST-level site is 5:1, mapped below. The closing 'BGP Large Communities SHOULD be represented in the canonical representation' is a capitalised advisory keyword rather than a MUST-level one, so the site scan does not count it; it is declared RFC8092-5-2 and listed unsourced here. |
| `6` | Error Handling | 4 | walked | Error Handling. The densest section of the document and the only one whose every sentence is normative: three bulleted SHALL-level rules and a closing MUST NOT. All four are sites, 6:1 through 6:4, and all four are mapped below to the four rows the summary declares from this section. The section carries no other sentence beyond its one-line lead-in, 'The error handling of BGP Large Communities is as follows'. |
| `7` | Security Considerations | 0 | walked | Security Considerations. No capitalised keyword and no site. Its one strong modal is lowercase and binds a trust model rather than a speaker: 'an AS relying on the BGP Large Communities attribute carried in BGP must have trust in every other AS in the path, as any intermediate AS in the path may have added, deleted, or altered the BGP Large Communities attribute', and the next sentence puts the mechanism providing that trust beyond the document's scope. The remainder is advice to operators: the attribute does not protect the integrity of a community value, a speaker can alter values in an UPDATE, securing that is the broader BGP security problem, and administrators are pointed at Section 11 of RFC 7454. Nothing here directs encoding, decoding or propagation, so there is nothing for a daemon to implement. |
| `8` | IANA Considerations | 0 | walked | IANA Considerations. One indicative sentence: 'IANA has assigned the value 32 (LARGE_COMMUNITY) in the "BGP Path Attributes" subregistry under the "Border Gateway Protocol (BGP) Parameters" registry.' Walked rather than skipped under the iana kind because it records an assignment already made, which Ze depends on to encode and recognise the attribute, rather than an action left for the registry to take. The value is carried by the Constants table of rfc/short/rfc8092.md. No obligation, so no id is read from here. |
| `9` | References | 0 | skipped (references) | References. The heading alone; its two subsections carry the entries. |
| `9.1` | Normative References: RFC 2119, RFC 4271, RFC 7606 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 7606. |
| `9.2` | not stated | 0 | skipped (references) | Informative References: RFC 1997, RFC 4360, RFC 6793, RFC 7300, RFC 7454, RFC 7607. The derived section span runs to the end of the document, so it also holds Acknowledgments, Contributors and Authors' Addresses. No sentence in the span states an obligation on a BGP speaker. |

### Excluded sentences

The walk over RFC 8092 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 8092, so its obligations are stated where they were written.
