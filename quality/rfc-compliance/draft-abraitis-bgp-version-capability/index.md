# DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY - Software Version Capability for BGP

Supported. Every requirement this repository extracted from DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 11 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 11 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 11 of 11 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 11 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 15 |
| Gated MUST-level | 11 |
| Obligations that bind Ze | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 11 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/draft-abraitis-bgp-version-capability.md` |
| Requirement shard | `rfc/requirements/draft-abraitis-bgp-version-capability.md` |
| RFC text | `rfc/drafts/draft-abraitis-bgp-version-capability.txt` |

## Enrolment

Enrolled: Software Version capability for BGP (code 75): eleven MUST-level requirements. Send side is implemented and gated by config -- encodeValue writes one length octet plus the constant ZeVersion and extractSoftverCapabilities declares code 75 only for a peer or group whose config carries a software-version key that is not disable or refuse (internal/component/bgp/plugins/softver/softver.go), which is 3-1, 3-2, 3-3, 3-6, 3-8 and 4-2. Receive side: parseCapability (internal/core/bgp/capability/capability.go) has no case for code 75, so a received capability becomes an Unknown nothing reads, which is how 3-4, 3-5, 3-7 and 4-1 are met. 3.1-1 requires RFC 9072, which is enrolled separately and Partial. No requirement carries a tagged test yet; the coverage rollup names them.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

- Send side only, and the draft makes both halves optional: "Implementations are not required to advertise the version nor to process received advertisements" (Abstract). `encodeValue` ([`internal/component/bgp/plugins/softver/softver.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/softver.go)) writes one length octet plus the constant `ZeVersion`, and `extractSoftverCapabilities` (same file) declares code 75 only for a peer or group whose config carries a `software-version` capability key that is not `disable` or `refuse`, so the default is disabled. Receive side: `parseCapability` ([`internal/core/bgp/capability/capability.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability.go)) has no case for code 75, so a received capability becomes an `Unknown` that nothing reads. That is the RFC 5492 Section 3 outcome the draft points at, and it is why the three receiver obligations are met by ignoring rather than by parsing. `ze bgp decode capability 75 <hex>` decodes a payload an operator supplies
- that path is a diagnostic tool and is not on the session receive path. Carried in this table as `draft-ietf-idr-software-version` until 2026-09-01. No IETF document of that name exists: the datatracker knows only `draft-abraitis-bgp-version-capability`, revision 18, "Software Version Capability for BGP", which IANA names as the reference for code 75. [`docs/architecture/wire/capabilities.md`](https://github.com/ze-software/ze/blob/main/docs/architecture/wire/capabilities.md) had always spelled the real one.


**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 11 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (11):** [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1`](#draft-abraitis-bgp-version-capability-3-1), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2`](#draft-abraitis-bgp-version-capability-3-2), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3`](#draft-abraitis-bgp-version-capability-3-3), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4`](#draft-abraitis-bgp-version-capability-3-4), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5`](#draft-abraitis-bgp-version-capability-3-5), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6`](#draft-abraitis-bgp-version-capability-3-6), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7`](#draft-abraitis-bgp-version-capability-3-7), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8`](#draft-abraitis-bgp-version-capability-3-8), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1`](#draft-abraitis-bgp-version-capability-3.1-1), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1`](#draft-abraitis-bgp-version-capability-4-1), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2`](#draft-abraitis-bgp-version-capability-4-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1` | "If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use, and MUST default to disabled" -- the configuration option half (§3) | MUST | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2` | "If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use, and MUST default to disabled" -- the default-to-disabled half (§3) | MUST | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3` | "The Capability Length for the Software Version Capability MUST be greater than zero" (§3) | MUST | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4` | "A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored" -- the encoding-error half (§3) | SHALL | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5` | "A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored" -- the ignore half (§3) | MUST | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6` | "The Version field MUST be encoded using UTF-8" (§3) | MUST | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7` | "A receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences" (§3) | MUST NOT | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8` | "A sender SHOULD limit generated product identifiers to what is necessary to identify the product; a sender MUST NOT generate advertising or other nonessential information within the product identifier" -- the MUST NOT half (§3) | MUST NOT | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-9` | "The Capability Length SHOULD be no greater than 64" (§3) | SHOULD | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-10` | "A sender SHOULD limit generated product identifiers to what is necessary to identify the product" (§3) | SHOULD | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-11` | "A sender SHOULD NOT generate information in product-version that is not a version identifier" (§3) | SHOULD NOT | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-12` | "It is NOT RECOMMENDED for use outside a single Autonomous System, or a set of Autonomous Systems under a common administration" (§3) | NOT RECOMMENDED | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1` | "Implementations of this specification are REQUIRED Extended Optional Parameters Length for BGP OPEN Message support as defined in [RFC9072]" (§3.1) | REQUIRED | 3.1 - Capabilities Length Overflow | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1` | "The Software Version Capability MUST only be used for displaying the version of a BGP speaker's router daemon to make troubleshooting easier" (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2` | "Enabling (i.e., turning on) this capability requires bouncing all existing BGP sessions and the feature MUST be explicitly configured before an implementation advertizes the Software Version Capability" (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1`](#draft-abraitis-bgp-version-capability-3-1) "If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use, and MUST default to disabled" -- the configuration option half (§3) | no test | no test carries this requirement id |
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2`](#draft-abraitis-bgp-version-capability-3-2) "If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use, and MUST default to disabled" -- the default-to-disabled half (§3) | no test | no test carries this requirement id |
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3`](#draft-abraitis-bgp-version-capability-3-3) "The Capability Length for the Software Version Capability MUST be greater than zero" (§3) | no test | no test carries this requirement id |
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4`](#draft-abraitis-bgp-version-capability-3-4) "A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored" -- the encoding-error half (§3) | no test | no test carries this requirement id |
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5`](#draft-abraitis-bgp-version-capability-3-5) "A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored" -- the ignore half (§3) | no test | no test carries this requirement id |
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6`](#draft-abraitis-bgp-version-capability-3-6) "The Version field MUST be encoded using UTF-8" (§3) | no test | no test carries this requirement id |
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7`](#draft-abraitis-bgp-version-capability-3-7) "A receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences" (§3) | no test | no test carries this requirement id |
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8`](#draft-abraitis-bgp-version-capability-3-8) "A sender SHOULD limit generated product identifiers to what is necessary to identify the product; a sender MUST NOT generate advertising or other nonessential information within the product identifier" -- the MUST NOT half (§3) | no test | no test carries this requirement id |
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1`](#draft-abraitis-bgp-version-capability-3.1-1) "Implementations of this specification are REQUIRED Extended Optional Parameters Length for BGP OPEN Message support as defined in [RFC9072]" (§3.1) | no test | no test carries this requirement id |
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1`](#draft-abraitis-bgp-version-capability-4-1) "The Software Version Capability MUST only be used for displaying the version of a BGP speaker's router daemon to make troubleshooting easier" (§4) | no test | no test carries this requirement id |
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2`](#draft-abraitis-bgp-version-capability-4-2) "Enabling (i.e., turning on) this capability requires bouncing all existing BGP sessions and the feature MUST be explicitly configured before an implementation advertizes the Software Version Capability" (§4) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1`](#draft-abraitis-bgp-version-capability-3-1)

"If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use, and MUST default to disabled" -- the configuration option half (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1, so no unit is bound to it.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2`](#draft-abraitis-bgp-version-capability-3-2)

"If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use, and MUST default to disabled" -- the default-to-disabled half (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2, so no unit is bound to it.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3`](#draft-abraitis-bgp-version-capability-3-3)

"The Capability Length for the Software Version Capability MUST be greater than zero" (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3, so no unit is bound to it.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4`](#draft-abraitis-bgp-version-capability-3-4)

"A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored" -- the encoding-error half (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4, so no unit is bound to it.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5`](#draft-abraitis-bgp-version-capability-3-5)

"A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored" -- the ignore half (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5, so no unit is bound to it.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6`](#draft-abraitis-bgp-version-capability-3-6)

"The Version field MUST be encoded using UTF-8" (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6, so no unit is bound to it.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7`](#draft-abraitis-bgp-version-capability-3-7)

"A receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences" (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7, so no unit is bound to it.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8`](#draft-abraitis-bgp-version-capability-3-8)

"A sender SHOULD limit generated product identifiers to what is necessary to identify the product; a sender MUST NOT generate advertising or other nonessential information within the product identifier" -- the MUST NOT half (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8, so no unit is bound to it.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1`](#draft-abraitis-bgp-version-capability-3.1-1)

"Implementations of this specification are REQUIRED Extended Optional Parameters Length for BGP OPEN Message support as defined in [RFC9072]" (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1, so no unit is bound to it.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1`](#draft-abraitis-bgp-version-capability-4-1)

"The Software Version Capability MUST only be used for displaying the version of a BGP speaker's router daemon to make troubleshooting easier" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1, so no unit is bound to it.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2`](#draft-abraitis-bgp-version-capability-4-2)

"Enabling (i.e., turning on) this capability requires bouncing all existing BGP sessions and the feature MUST be explicitly configured before an implementation advertizes the Software Version Capability" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 draft walk, draft-abraitis-bgp-version-capability |
| Signed off | 2026-09-01 |
| Register | prose |
| Source | rfc/drafts/draft-abraitis-bgp-version-capability.txt |
| Source fingerprint | 472ca7a757b75dda |
| Record | rfc/extraction/draft-abraitis-bgp-version-capability.json |
| Mapped sentences | 9 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | skipped (front-matter) | Title block, Abstract, Status of This Memo and Copyright Notice. The Abstract carries the sentence that makes the whole mechanism optional and one boilerplate site; both are excluded below. |
| `1` | Introduction | 1 | walked | Introduction. Says data-center routers run different routing-software versions, that knowing them helps root-cause a fault, and that LLDP and CDP are awkward in containerized environments. Its one site restates the Abstract's optionality and is excluded below. No obligation. |
| `2` | Specification of Requirements | 0 | walked | Specification of Requirements. The BCP 14 key-words paragraph. It tells a reader how to read the rest and binds nobody, which is why the derivation raises no site here. |
| `3` | Software Version Capability | 7 | walked | Software Version Capability. The document's main normative section: seven sites, five mapped to DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1, 3-3, 3-4, 3-6, 3-7 and 3-8, and two excluded. It also assigns Capability Code 75 and defines the Capability Length and Capability Value fields, which are value definitions carried by the Wire Format section of rfc/short/draft-abraitis-bgp-version-capability.md. Six ids are declared unsourced here. Two are the second obligation of a sentence already mapped: 3-2, the MUST to default to disabled, shares site 3:1 with 3-1, and 3-5, the MUST to ignore a zero-length capability, shares site 3:4 with 3-4. The other four are advisory levels the capitalised MUST-level scan does not raise: 3-9 (the SHOULD that Capability Length be no greater than 64), 3-10 (the SHOULD to limit the product identifier, the first clause of site 3:7), 3-11 (the SHOULD NOT to put non-version information in product-version) and 3-12 (the NOT RECOMMENDED against use outside one Autonomous System). The opening sentence, that the document is not Standards Track but uses BCP 14 terminology, and the OPTIONAL sentence 'The inclusion of the Software Version Capability is OPTIONAL' state no obligation on a speaker. |
| `3.1` | Capabilities Length Overflow | 2 | walked | Capabilities Length Overflow. Two sites: 3.1:1 is mapped to DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1 and 3.1:2 is excluded below. The rest recounts that RFC 5492 caps the BGP Capabilities Optional Parameter at 255 bytes and that exceeding it leaves no room for more important capabilities. |
| `4` | Operation | 2 | walked | Operation. Two sites, both mapped, to DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1 and DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2. The remaining paragraph is the worked example of an operator correlating a bug with a routing-daemon version across upstream nodes. |
| `5` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA has assigned capability number 75 in the BGP Capability Codes registry, with the one-row table naming it. Binds IANA, not a speaker. |
| `6` | Security Considerations | 0 | walked | Security Considerations. States that the version should be treated as sensitive, that an attacker who learns it has an easier time, that a forged or snooped value is possible without an integrity- or confidentiality-providing transport, and that GTSM or a firewall on TCP 179 limits the exposure. Every sentence uses a lowercase 'should' the document's own Section 2 gives no normative level, and the derivation raises no site. No obligation on a speaker. |
| `A` | Appendix A | 0 | skipped (appendix-non-normative) | Appendix A. Implementation Report. The RFC 7942 running-code list -- FRRouting, GoBGP, freeRouter and ExaBGP commits -- plus three figures of sample CLI output. No obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative, and permissive rather than obligatory: it states that an implementation need not advertise the version and need not process a received one. It directs no behavior, and it is the sentence that makes both halves of the mechanism optional. Ze offers the send half and declines the receive half, which is why the three receiver requirements are met by ignoring rather than by parsing. | Implementations are not required to advertise the version nor to process received advertisements. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate the splitter did not strip. The sentence binds a person who extracts Code Components from the document into other software and tells them to carry the Revised BSD License text. It directs no BGP speaker and describes no protocol behavior. Its 'must' is lowercase, so it is raised only by the case-insensitive modal scan the 'prose' register uses. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Introduction's restatement of the Abstract sentence at site front:1, in the same permissive shape: implementations 'are not required to' advertise or to process. Indicative, and it directs no behavior. It is not recorded as a duplicate-of, because duplicate-of names an id another site maps and no site maps this sentence: it states no obligation for an id to hold. | Implementations are not required to advertise their software version nor to process it on receipt. |
| `3:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 5492, which the sentence cites: a speaker that receives a capability it does not recognize or support must ignore it, RFC 5492 Section 3. The sentence is a back-reference telling a reader what already governs an unrecognized code 75, and its 'must' is lowercase. RFC 5492 is enrolled and that obligation is gated there as RFC5492-3-2. | An implementation that does not recognize or support the Software Version Capability but receives one must ignore it, as described in Section 3 of [RFC5492]. |
| `3.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative, and a security observation rather than a directive: it states that a rogue node can deny the advertisement of other capabilities by not excluding this one, and that the risk is not new to BGP. The words 'as required in Section 3.1' are a back-reference to the sentence at site 3.1:1, which this walk maps; they impose nothing of their own. | A rogue node can prevent the proper operation of a BGP session, or the advertisement of other Capabilities, by not excluding the Software Version Capability as required in Section 3.1. |

## Superseded

No document obsoletes DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY, so its obligations are stated where they were written.
