# RFC 1877 - PPP Internet Protocol Control Protocol Extensions for Name Server Addresses

IPCP DNS options. Every requirement listed for RFC 1877, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 0 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 0 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 0 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 0 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 0 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 0 | of 0 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 0 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 0 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 0 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 0 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 0 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 0 | of 0 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

No card above is a part of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | IPCP DNS options |
| Automatic checks | Included |
| Requirements | 0 |
| Mandatory requirements (MUST level) | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc1877.md` |
| Requirement table | no requirements are listed, so no table is generated |
| RFC text | `rfc/full/rfc1877.txt` |

## Automatic checks

Included in automatic checks: PPP IPCP Extensions for Name Server Addresses (DNS options). The document states no MUST-level requirement: its only RFC 2119 keyword is one SHOULD in Section 1, and rfc/extraction/rfc1877.json signs that zero off under manual-walk. The five rows this summary once declared were read from RFC 1661 and were retired on 2026-09-26 (rfc/corrections/rfc1877.md); the Configure-Ack and Configure-Reject echo rules they restated are gated as RFC1661-5.2-1, RFC1661-5.2-2, RFC1661-5.4-1 and RFC1661-5.4-2, where their tests now sit.

## Published support status

**Area:** IPCP DNS options

**Test verification:** no MUST rows

**Evidence:** -

**Unmet:** -


## Coverage

RFC 1877 lists no mandatory requirements for these checks, so nothing is counted here.

## Figures

- **MUST bar**: no rows
- **Overview bar**: no rows

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 0 |
| MUST rows kept (the MUST bar) | 0 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc1877.json](../../../data/rfc/rfc1877.json).

## Requirements

RFC 1877 lists no requirements, so no requirement table is generated.

## Gaps and untested MUSTs

RFC 1877 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

RFC 1877 has no requirements included in automatic checks, linked to tests, or reviewed, so there is no test evidence to show.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-fixit-rfc-drain-quota-never-armed WP-1 |
| Signed off | 2026-08-31 |
| Register | manual-walk |
| Source | rfc/full/rfc1877.txt |
| Source fingerprint | 868068cbe12bb56c |
| Record | rfc/extraction/rfc1877.json |
| Mapped sentences | 0 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo ('This memo provides information for the Internet community. This memo does not specify an Internet standard of any kind.'), Abstract and Table of Contents. The Abstract says what the document extends and states no obligation. The table of contents lines are indented, so they open no section of their own. |
| `1` | Additional IPCP Configuration Options | 0 | reviewed | Additional IPCP Configuration Options. It introduces options 129 to 132, says primary and secondary addresses are negotiated independently, says the options are 'designed to be identical in format and behavior to option 3 (IP-Address)', and suggests they not be included in the list of "IPCP Recommended Options". It carries the document's only RFC 2119 keyword, the SHOULD quoted in the register-reason; SHOULD is not a gated level and is not a site under either scan, and rfc/short/rfc1877.md declares no row for it. The section directs no MUST at any speaker. The 'identical in format and behavior to option 3' sentence imports the Configure-Ack and Configure-Reject rules of RFC 1661 Sections 5.2 and 5.4 instead of stating them here; RFC 1661's own rows carry them. |
| `1.1` | not stated | 0 | reviewed | Primary DNS Server Address: Description, packet diagram, Type 129, Length 6, field meaning and Default. Every sentence is indicative. It describes the option's format and what the remote peer typically does ('the remote peer specifies the address by NAKing this option, and returning the IP address of a valid DNS server'), and directs no MUST at a speaker. The Length value stated here, which sections 1.2, 1.3 and 1.4 repeat, carries no rule for an option received with another Length; RFC 1661 Section 6 states that rule, and it calls for a Configure-Nak. |
| `1.2` | not stated | 0 | reviewed | Primary NBNS Server Address: Type 130, Length 6, and the same Description, diagram, field meaning and Default shape as 1.1, with NBNS in place of DNS. No sentence states an obligation. |
| `1.3` | not stated | 0 | reviewed | Secondary DNS Server Address: Type 131, Length 6, same shape again. No sentence states an obligation. The field paragraph carries a copy error in RFC 1877's own text, 'The four octet Secondary-DNS-Address is the address of the primary NBNS server to be used by the local peer', where every other field paragraph names its own option; it is a defect of the source, not an obligation, and it is recorded here so a later reader does not read it as one. |
| `1.4` | not stated | 0 | reviewed | Secondary NBNS Server Address: Type 132, Length 6, same shape, no obligation. This section also carries the whole unnumbered tail of the document, because 'References', 'Security Considerations', 'Chair's Address' and 'Author's Address' head no numbered heading and sectionHeadingRE matches none of them (internal/le/rfc/inventory.go, sectionBodies), so the derivation folds them in here. The tail was walked with the section: the reference list cites RFC 1661, RFC 1332, STD 19 and STD 13 and binds nobody; Security Considerations is one sentence, 'Security issues are not discussed in this memo.', so the document names no countermeasure and no threat; the two address blocks are contact details. Nothing in this section or its tail is MUST-level. |

### Excluded sentences

The review of RFC 1877 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 1877, so its obligations are stated where they were written.
