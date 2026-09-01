# RFC 4578 - Dynamic Host Configuration Protocol (DHCP) Options for the Intel Preboot eXecution Environment (PXE)

Supported. Every requirement this repository extracted from RFC 4578, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 1 of 1 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 1 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 1 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 1 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 1 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 8 |
| Gated MUST-level | 5 |
| Obligations that bind Ze | 1 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4578.md` |
| Requirement shard | `rfc/requirements/rfc4578.md` |
| RFC text | `rfc/full/rfc4578.txt` |

## Enrolment

Enrolled: RFC 4578 PXE DHCP options (93/94/97): five gated MUST requirements, one tested and four {not-applicable} (ze is a DHCP server, not a PXE client or Boot Server). RFC4578-2.1-2 (option 93 Client System Architecture Len MUST be an even number greater than zero) is MET and tested via TestParsePXEArch (internal/plugins/dhcpserver/handler_test.go): the producer parsePXEArch (internal/plugins/dhcpserver/handler.go:505) rejects a length < 2 or odd, positive case an even Len 2 is accepted and parsed, negative case an odd Len 3 is rejected and returns 0 (BIOS default). The other four are {not-applicable}: RFC4578-2.1-1 (option 93 present in sent packets) -- ze reads option 93 for bootfile selection but emits none in OFFER/ACK (appendPXEOptions handler.go:292-331 emits only 60/66/67/43); RFC4578-2.2-1 (option 94 Client Network Interface Identifier present) and RFC4578-2.3-1 (option 97 Client Machine Identifier present) -- ze has no option 94/97 code path; RFC4578-2.4-1 (PXE clients MUST request options 128-135) -- binds the PXE client role ze never plays. No SHOULD/MAY requirements are gated.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

PXE boot option injection for BIOS and UEFI bootfile selection.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC4578-2.1-2`](#rfc4578-2.1-2)

**Annotated instead of tested (4):** [`RFC4578-2.1-1`](#rfc4578-2.1-1), [`RFC4578-2.2-1`](#rfc4578-2.2-1), [`RFC4578-2.3-1`](#rfc4578-2.3-1), [`RFC4578-2.4-1`](#rfc4578-2.4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4578-2.1-1` | Option 93 (Client System Architecture Type) MUST be present in all DHCP and PXE packets sent by PXE-compliant clients and servers (Section 2.1) | MUST | 2.1 - Client System Architecture Type Option Definition, option 93 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a DHCP server, not a PXE client; it reads a received option 93 to select a bootfile (parsePXEArch, internal/plugins/dhcpserver/handler.go:505) but emits no option 93 in its OFFER/ACK replies (appendPXEOptions, internal/plugins/dhcpserver/handler.go:292) and ze has no PXE Boot Server Discovery echo code path -- it sets PXE_DISCOVERY_CONTROL to skip that exchange (internal/plugins/dhcpserver/handler.go:325) |
| `RFC4578-2.1-2` | Option 93 Len field MUST be an even number greater than zero (Section 2.1) | MUST | 2.1 - Client System Architecture Type Option Definition, option 93 | **positive:** `unit/verify` [`TestParsePXEArch`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L923). **negative:** `unit/verify` [`TestParsePXEArch`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L926) |
| `RFC4578-2.2-1` | Option 94 (Client Network Interface Identifier) MUST be present in all DHCP and PXE packets sent by PXE-compliant clients and servers (Section 2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a DHCP server, not a PXE client; ze has no option 94 code path -- it neither reads nor emits option 94 anywhere under internal/plugins/dhcpserver/ (grep for 94/UNDI in handler.go finds nothing) |
| `RFC4578-2.3-1` | Option 97 (Client Machine Identifier) MUST be present in all DHCP and PXE packets sent by PXE-compliant clients and servers (Section 2.3) | MUST | 2.3 - Client Machine Identifier Option Definition, option 97 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a DHCP server, not a PXE client; ze has no option 97 code path -- it neither reads nor emits option 97 (the client machine GUID) anywhere under internal/plugins/dhcpserver/ |
| `RFC4578-2.4-1` | All compliant PXE clients MUST include a request for DHCP options 128 through 135 in all DHCP and PXE packets (Section 2.4) | MUST | 2.4 - Options Requested by PXE Clients | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a DHCP server, not a PXE client; this requirement binds the PXE client to request options 128-135, a role ze never plays |
| `RFC4578-2.1-3` | Clients that support more than one architecture type MAY include a list of these types in their initial DHCP and PXE boot server packets (Section 2.1) | MAY | 2.1 - Client System Architecture Type Option Definition, option 93 | **positive:** no positive test. **negative:** no negative test |
| `RFC4578-2.1-4` | The list of supported architecture types MAY be reduced in any packet exchange between the client and server(s) (Section 2.1) | MAY | 2.1 - Client System Architecture Type Option Definition, option 93 | **positive:** no positive test. **negative:** no negative test |
| `RFC4578-2.4-2` | Options 128-135 MAY be present in the DHCP and PXE boot server replies (Section 2.4) | MAY | 2.4 - Options Requested by PXE Clients | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4578-2.1-1`](#rfc4578-2.1-1) Option 93 (Client System Architecture Type) MUST be present in all DHCP and PXE packets sent by PXE-compliant clients and servers (Section 2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a DHCP server, not a PXE client; it reads a received option 93 to select a bootfile (parsePXEArch, internal/plugins/dhcpserver/handler.go:505) but emits no option 93 in its OFFER/ACK replies (appendPXEOptions, internal/plugins/dhcpserver/handler.go:292) and ze has no PXE Boot Server Discovery echo code path -- it sets PXE_DISCOVERY_CONTROL to skip that exchange (internal/plugins/dhcpserver/handler.go:325) |
| [`RFC4578-2.2-1`](#rfc4578-2.2-1) Option 94 (Client Network Interface Identifier) MUST be present in all DHCP and PXE packets sent by PXE-compliant clients and servers (Section 2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a DHCP server, not a PXE client; ze has no option 94 code path -- it neither reads nor emits option 94 anywhere under internal/plugins/dhcpserver/ (grep for 94/UNDI in handler.go finds nothing) |
| [`RFC4578-2.3-1`](#rfc4578-2.3-1) Option 97 (Client Machine Identifier) MUST be present in all DHCP and PXE packets sent by PXE-compliant clients and servers (Section 2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a DHCP server, not a PXE client; ze has no option 97 code path -- it neither reads nor emits option 97 (the client machine GUID) anywhere under internal/plugins/dhcpserver/ |
| [`RFC4578-2.4-1`](#rfc4578-2.4-1) All compliant PXE clients MUST include a request for DHCP options 128 through 135 in all DHCP and PXE packets (Section 2.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a DHCP server, not a PXE client; this requirement binds the PXE client to request options 128-135, a role ze never plays |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4578-2.1-1`](#rfc4578-2.1-1)

Option 93 (Client System Architecture Type) MUST be present in all DHCP and PXE packets sent by PXE-compliant clients and servers (Section 2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4578-2.1-1, so no unit is bound to it.

### [`RFC4578-2.1-2`](#rfc4578-2.1-2)

Option 93 Len field MUST be an even number greater than zero (Section 2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParsePXEArch`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L926) | unit/verify | unproven |
| positive | [`TestParsePXEArch`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L923) | unit/verify | unproven |

### [`RFC4578-2.2-1`](#rfc4578-2.2-1)

Option 94 (Client Network Interface Identifier) MUST be present in all DHCP and PXE packets sent by PXE-compliant clients and servers (Section 2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4578-2.2-1, so no unit is bound to it.

### [`RFC4578-2.3-1`](#rfc4578-2.3-1)

Option 97 (Client Machine Identifier) MUST be present in all DHCP and PXE packets sent by PXE-compliant clients and servers (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4578-2.3-1, so no unit is bound to it.

### [`RFC4578-2.4-1`](#rfc4578-2.4-1)

All compliant PXE clients MUST include a request for DHCP options 128 through 135 in all DHCP and PXE packets (Section 2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4578-2.4-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc4578 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc4578.txt |
| Source fingerprint | 0c9586ab5658d5ee |
| Record | rfc/extraction/rfc4578.json |
| Mapped sentences | 5 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. The Status of This Memo states the document specifies no Internet standard, and the Abstract restates section 1. Neither directs a DHCP speaker. |
| `1` | Introduction | 0 | walked | Introduction. Explains why the MAC address in the chaddr field was deemed insufficient to identify a booting machine (shared docking stations reusing a MAC, one machine holding several interfaces) and says the three options were defined by Intel in the PXE and EFI specifications and are documented here for completeness. Entirely indicative. No gated requirement of rfc/short/rfc4578.md is read from this section. |
| `1.1` | Requirements Language: the RFC 2119 key-words paragraph | 0 | walked | Requirements Language: the RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Option Definitions | 0 | walked | Option Definitions. One sentence: 'There are three DHCP options [5] defined for use by PXE clients.' It counts the subsections that follow and directs nobody. |
| `2.1` | Client System Architecture Type Option Definition, option 93 | 2 | walked | Client System Architecture Type Option Definition, option 93. Holds two of the document's five sites, both mapped below: the Len constraint (2.1:1) and the presence obligation (2.1:2). The rest of the section is the option figure, the architecture type table (types 0 to 9), the statement that octets n1 and n2 encode a 16-bit architecture type identifier, and two MAY sentences that are advisory and never gate: a client supporting more than one architecture type MAY list them (RFC4578-2.1-3) and the list MAY be reduced in any packet exchange (RFC4578-2.1-4). Ze reads the first type from a received option 93 in parsePXEArch (internal/plugins/dhcpserver/handler.go), which is why a multi-type list is parsed rather than refused. |
| `2.2` | not stated | 1 | walked | Client Network Interface Identifier Option Definition, option 94. The figure, the statement that octet t encodes an interface type whose only supported value is 1 (UNDI), the M and m revision encoding, and the UNDI revision table are indicative. The one normative sentence is site 2.2:1, mapped below. |
| `2.3` | Client Machine Identifier Option Definition, option 97 | 1 | walked | Client Machine Identifier Option Definition, option 97. The figure, the statement that type 0 is the only defined value and describes the remaining octets as a 16-octet GUID, and the note that octet n is 17 for type 0 are indicative. The one normative sentence is site 2.3:1, mapped below. |
| `2.4` | Options Requested by PXE Clients | 1 | walked | Options Requested by PXE Clients. Its one MUST is site 2.4:1, mapped below. The rest states that the format and contents of options 128 to 135 are not defined by the PXE specification, that they MAY be present in replies (RFC4578-2.4-2, advisory and never gated), that they are not used by the PXE boot ROMs, and that because they were site-specific options before November 2004 their use for PXE may conflict with other uses on the same network. |
| `3` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. One sentence thanking Bernie Volz. |
| `4` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA updated the public DHCP option numbering space with references to this document for options 93, 94 and 97, and marked options 128 to 135 as used by PXE. Binds IANA, not a DHCP speaker, and is written in the past tense. |
| `5` | Security Considerations | 0 | walked | Security Considerations. Two observations and no countermeasure: a client specifying incorrect option values may reach code intended for another platform, and the options reveal a client's system architecture and pre-OS runtime environment to anyone listening to its DHCP messages. Neither sentence directs a speaker to do or refuse anything, so the section states no requirement. rfc/short/rfc4578.md carries both as its Security Considerations table. |
| `6` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 2131, the Intel PXE and EFI specifications, RFC 2132, RFC 3942, RFC 2939 and RFC 3679. The derivation folds the trailing matter under this heading as well -- the Authors' Addresses, the Full Copyright Statement, the Intellectual Property notice and the RFC Editor funding acknowledgement. None of it binds a DHCP speaker, and the site scan derives no site here. |

### Excluded sentences

The walk over RFC 4578 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 4578, so its obligations are stated where they were written.
