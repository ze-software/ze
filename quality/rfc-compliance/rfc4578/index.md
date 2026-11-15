# RFC 4578 - Dynamic Host Configuration Protocol (DHCP) Options for the Intel Preboot eXecution Environment (PXE)

DHCP PXE options. Every requirement listed for RFC 4578, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 5 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 5 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 5 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 4 | 4 of 4 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 3 | of 5 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 5 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 1 | of 5 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 5 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 5 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 5 | of 8 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 5 mandatory requirements: they add up to 5. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | DHCP PXE options |
| Automatic checks | Included |
| Requirements | 8 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tests linked to requirements | 4 |
| Recorded audit verdicts | 1 |
| Fault test records | 4 |
| Summary | `rfc/short/rfc4578.md` |
| Requirement table | `rfc/requirements/rfc4578.md` |
| RFC text | `rfc/full/rfc4578.txt` |

## Automatic checks

Included in automatic checks: RFC 4578 PXE DHCP options (93/94/97): five gated MUST requirements, one tested, three gaps and one not-applicable (ze is a DHCP server, not a PXE client). RFC4578-2.1-2 (option 93 Client System Architecture Len MUST be an even number greater than zero) is MET and tested via TestParsePXEArch (internal/plugins/dhcpserver/handler_test.go): the producer parsePXEArch (internal/plugins/dhcpserver/handler.go:505) rejects a length < 2 or odd, positive case an even Len 2 is accepted and parsed, negative case an odd Len 3 is rejected and returns 0 (BIOS default). Three are gaps and one is not-applicable: RFC4578-2.1-1 (option 93 present in sent packets) -- ze reads option 93 for bootfile selection but emits none in OFFER/ACK (appendPXEOptions handler.go:292-331 emits only 60/66/67/43); RFC4578-2.2-1 (option 94 Client Network Interface Identifier present) and RFC4578-2.3-1 (option 97 Client Machine Identifier present) -- ze has no option 94/97 code path; RFC4578-2.4-1 (PXE clients MUST request options 128-135) -- binds the PXE client role ze never plays. No SHOULD/MAY requirements are gated.

## Published support status

**Area:** DHCP PXE options

**Proof:** 1 of 5 MUST proven

**Evidence:** unit 1

**Unmet:** RFC4578-2.1-1, RFC4578-2.2-1, RFC4578-2.3-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 4 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **5** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC4578-2.1-2`](#rfc4578-2.1-2)

**Annotated (including scoped evidence) (4):** [`RFC4578-2.1-1`](#rfc4578-2.1-1), [`RFC4578-2.2-1`](#rfc4578-2.2-1), [`RFC4578-2.3-1`](#rfc4578-2.3-1), [`RFC4578-2.4-1`](#rfc4578-2.4-1)

## Figures

- **MUST bar**: Ze proven 1, Not built 3
- **Overview bar**: Ze MUST proven 1, Not owed 1, Not assessed 3, Not built 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 4 |
| Ze proven | 1 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 3 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 3 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4578.json](../../../data/rfc/rfc4578.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4578-2.1-1` | This option MUST be present in all DHCP and PXE packets sent by PXE- compliant clients and servers. (Section 2.1) | MUST | 2.1 - Client System Architecture Type Option Definition, option 93 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's DHCP server reads a received option 93 to select a bootfile (internal/plugins/dhcpserver/handler.go::parsePXEArch) but sends none in its OFFER or ACK (internal/plugins/dhcpserver/handler.go::appendPXEOptions emits options 60, 66, 67 and 43), so the PXE replies it sends lack the option |
| `RFC4578-2.1-2` | Octet "n" gives the number of octets containing "architecture types" (not including the code and len fields). It MUST be an even number greater than zero. (Section 2.1) | MUST | 2.1 - Client System Architecture Type Option Definition, option 93 | **positive:** `unit/verify` [`TestParsePXEArch`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L906). **positive:** `unit/verify` [`TestRFC4578ClientArchLengthEvenAndPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc4578_arch_test.go#L61). **negative:** `unit/verify` [`TestParsePXEArch`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L909). **negative:** `unit/verify` [`TestRFC4578ClientArchLengthEvenAndPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc4578_arch_test.go#L76) |
| `RFC4578-2.2-1` | This option MUST be present in all DHCP and PXE packets sent by PXE- compliant clients and servers. (Section 2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's DHCP server neither reads nor sends option 94 (internal/plugins/dhcpserver/handler.go::appendPXEOptions emits options 60, 66, 67 and 43), so the PXE replies it sends lack the Client Network Interface Identifier |
| `RFC4578-2.3-1` | This option MUST be present in all DHCP and PXE packets sent by PXE- compliant clients and servers. (Section 2.3) | MUST | 2.3 - Client Machine Identifier Option Definition, option 97 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's DHCP server neither reads nor sends option 97 (internal/plugins/dhcpserver/handler.go::appendPXEOptions emits options 60, 66, 67 and 43), so the PXE replies it sends lack the Client Machine Identifier |
| `RFC4578-2.4-1` | All compliant PXE clients MUST include a request for DHCP options 128 through 135 in all DHCP and PXE packets (Section 2.4) | MUST | 2.4 - Options Requested by PXE Clients | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "All compliant PXE clients MUST include a request for DHCP options 128 through 135 in all DHCP and PXE packets."; binds compliant PXE clients; ze fills the DHCP server role for PXE, and its DHCP client (internal/plugins/iface/dhcp) performs no PXE boot |
| `RFC4578-2.1-3` | Clients that support more than one architecture type MAY include a list of these types in their initial DHCP and PXE boot server packets (Section 2.1) | MAY | 2.1 - Client System Architecture Type Option Definition, option 93 | **positive:** no positive test. **negative:** no negative test |
| `RFC4578-2.1-4` | The list of supported architecture types MAY be reduced in any packet exchange between the client and server(s) (Section 2.1) | MAY | 2.1 - Client System Architecture Type Option Definition, option 93 | **positive:** no positive test. **negative:** no negative test |
| `RFC4578-2.4-2` | These options MAY be present in the DHCP and PXE boot server replies and are meant for use by the downloaded network bootstrap programs. (Section 2.4) | MAY | 2.4 - Options Requested by PXE Clients | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4578-2.1-1`](#rfc4578-2.1-1) This option MUST be present in all DHCP and PXE packets sent by PXE- compliant clients and servers. (Section 2.1) | {gap}, no test | ze's DHCP server reads a received option 93 to select a bootfile (internal/plugins/dhcpserver/handler.go::parsePXEArch) but sends none in its OFFER or ACK (internal/plugins/dhcpserver/handler.go::appendPXEOptions emits options 60, 66, 67 and 43), so the PXE replies it sends lack the option |
| [`RFC4578-2.2-1`](#rfc4578-2.2-1) This option MUST be present in all DHCP and PXE packets sent by PXE- compliant clients and servers. (Section 2.2) | {gap}, no test | ze's DHCP server neither reads nor sends option 94 (internal/plugins/dhcpserver/handler.go::appendPXEOptions emits options 60, 66, 67 and 43), so the PXE replies it sends lack the Client Network Interface Identifier |
| [`RFC4578-2.3-1`](#rfc4578-2.3-1) This option MUST be present in all DHCP and PXE packets sent by PXE- compliant clients and servers. (Section 2.3) | {gap}, no test | ze's DHCP server neither reads nor sends option 97 (internal/plugins/dhcpserver/handler.go::appendPXEOptions emits options 60, 66, 67 and 43), so the PXE replies it sends lack the Client Machine Identifier |
| [`RFC4578-2.4-1`](#rfc4578-2.4-1) All compliant PXE clients MUST include a request for DHCP options 128 through 135 in all DHCP and PXE packets (Section 2.4) | no test | no test carries this requirement id; annotated {not-applicable}: "All compliant PXE clients MUST include a request for DHCP options 128 through 135 in all DHCP and PXE packets."; binds compliant PXE clients; ze fills the DHCP server role for PXE, and its DHCP client (internal/plugins/iface/dhcp) performs no PXE boot |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC4578-2.1-1`](#rfc4578-2.1-1)

This option MUST be present in all DHCP and PXE packets sent by PXE- compliant clients and servers. (Section 2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4578-2.1-1, so there is no linked test.

### [`RFC4578-2.1-2`](#rfc4578-2.1-2)

Octet "n" gives the number of octets containing "architecture types" (not including the code and len fields). It MUST be an even number greater than zero. (Section 2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. both earlier gaps closed. positive: lengths 2 and 4 accepted through the handler, type 7 selects the UEFI bootfile (TestRFC4578ClientArchLengthEvenAndPositive) plus TestParsePXEArch Len 2. negative: odd (Len 3, Len 1 in TestParsePXEArch) and now zero: a Len 0 option 93 followed by Pad and code 7 is refused and the BIOS bootfile served, so a parser taking the following octets as a type goes red

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParsePXEArch`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L909) | unit/verify | revert, verified |
| negative | [`TestRFC4578ClientArchLengthEvenAndPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc4578_arch_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestParsePXEArch`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L906) | unit/verify | revert, verified |
| positive | [`TestRFC4578ClientArchLengthEvenAndPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc4578_arch_test.go#L61) | unit/verify | revert, verified |

### [`RFC4578-2.2-1`](#rfc4578-2.2-1)

This option MUST be present in all DHCP and PXE packets sent by PXE- compliant clients and servers. (Section 2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4578-2.2-1, so there is no linked test.

### [`RFC4578-2.3-1`](#rfc4578-2.3-1)

This option MUST be present in all DHCP and PXE packets sent by PXE- compliant clients and servers. (Section 2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4578-2.3-1, so there is no linked test.

### [`RFC4578-2.4-1`](#rfc4578-2.4-1)

All compliant PXE clients MUST include a request for DHCP options 128 through 135 in all DHCP and PXE packets (Section 2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4578-2.4-1, so there is no linked test.

## Requirement list review

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
| `1` | Introduction | 0 | reviewed | Introduction. Explains why the MAC address in the chaddr field was deemed insufficient to identify a booting machine (shared docking stations reusing a MAC, one machine holding several interfaces) and says the three options were defined by Intel in the PXE and EFI specifications and are documented here for completeness. Entirely indicative. No gated requirement of rfc/short/rfc4578.md is read from this section. |
| `1.1` | Requirements Language: the RFC 2119 key-words paragraph | 0 | reviewed | Requirements Language: the RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Option Definitions | 0 | reviewed | Option Definitions. One sentence: 'There are three DHCP options [5] defined for use by PXE clients.' It counts the subsections that follow and directs nobody. |
| `2.1` | Client System Architecture Type Option Definition, option 93 | 2 | reviewed | Client System Architecture Type Option Definition, option 93. Holds two of the document's five sites, both mapped below: the Len constraint (2.1:1) and the presence obligation (2.1:2). The rest of the section is the option figure, the architecture type table (types 0 to 9), the statement that octets n1 and n2 encode a 16-bit architecture type identifier, and two MAY sentences that are advisory and never gate: a client supporting more than one architecture type MAY list them (RFC4578-2.1-3) and the list MAY be reduced in any packet exchange (RFC4578-2.1-4). Ze reads the first type from a received option 93 in parsePXEArch (internal/plugins/dhcpserver/handler.go), which is why a multi-type list is parsed rather than refused. |
| `2.2` | not stated | 1 | reviewed | Client Network Interface Identifier Option Definition, option 94. The figure, the statement that octet t encodes an interface type whose only supported value is 1 (UNDI), the M and m revision encoding, and the UNDI revision table are indicative. The one normative sentence is site 2.2:1, mapped below. |
| `2.3` | Client Machine Identifier Option Definition, option 97 | 1 | reviewed | Client Machine Identifier Option Definition, option 97. The figure, the statement that type 0 is the only defined value and describes the remaining octets as a 16-octet GUID, and the note that octet n is 17 for type 0 are indicative. The one normative sentence is site 2.3:1, mapped below. |
| `2.4` | Options Requested by PXE Clients | 1 | reviewed | Options Requested by PXE Clients. Its one MUST is site 2.4:1, mapped below. The rest states that the format and contents of options 128 to 135 are not defined by the PXE specification, that they MAY be present in replies (RFC4578-2.4-2, advisory and never gated), that they are not used by the PXE boot ROMs, and that because they were site-specific options before November 2004 their use for PXE may conflict with other uses on the same network. |
| `3` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. One sentence thanking Bernie Volz. |
| `4` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA updated the public DHCP option numbering space with references to this document for options 93, 94 and 97, and marked options 128 to 135 as used by PXE. Binds IANA, not a DHCP speaker, and is written in the past tense. |
| `5` | Security Considerations | 0 | reviewed | Security Considerations. Two observations and no countermeasure: a client specifying incorrect option values may reach code intended for another platform, and the options reveal a client's system architecture and pre-OS runtime environment to anyone listening to its DHCP messages. Neither sentence directs a speaker to do or refuse anything, so the section states no requirement. rfc/short/rfc4578.md carries both as its Security Considerations table. |
| `6` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 2131, the Intel PXE and EFI specifications, RFC 2132, RFC 3942, RFC 2939 and RFC 3679. The derivation folds the trailing matter under this heading as well -- the Authors' Addresses, the Full Copyright Statement, the Intellectual Property notice and the RFC Editor funding acknowledgement. None of it binds a DHCP speaker, and the site scan derives no site here. |

### Excluded sentences

The review of RFC 4578 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 4578, so its obligations are stated where they were written.
