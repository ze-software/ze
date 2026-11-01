# RFC 3765 - NOPEER Community for Border Gateway Protocol (BGP) Route Scope Control

NOPEER community. Every requirement this repository extracted from RFC 3765, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 0 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 0 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 0 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 0 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 0 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0 | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 0 | of 0 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 0 | of 2 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 0 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 0 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 0 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 0 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 0 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

No card above is a part of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | NOPEER community |
| Enrolment | Enrolled |
| Requirements | 2 |
| Gated MUST-level | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3765.md` |
| Requirement shard | `rfc/requirements/rfc3765.md` |
| RFC text | `rfc/full/rfc3765.txt` |

## Enrolment

Enrolled: NOPEER Community for BGP: an Informational document with ZERO gated obligations, and that absence is a property of the text rather than a gap in the summary (owner ruling OR-A, plan/spec-rfcgate-4-ledger.md). It carries no RFC 2119 key-words section and no occurrence of any of the ten keywords in any case; it calls its own mechanism "an advisory qualification to readvertisement of a route prefix" verbatim in section 2 and section 4, phrases the behaviour permissively both times, and defines no wire format -- NOPEER is a value carried in RFC 1997's COMMUNITIES attribute. Its two rows are therefore [MAY]. Enrolled on the evidence of a manual-walk extraction sign-off (rfc/extraction/rfc3765.json) rather than on a fabricated MUST: a requirement the RFC does not contain would put a false claim inside the ledger this gate exists to make honest.

## What the public ledger says

**Area:** NOPEER community

**Proof:** no MUST rows

**Evidence:** -

**Unmet:** -


## Coverage

RFC 3765 declares no MUST-level requirement, so the gate counts nothing here.

## Figures

- **MUST bar**: no rows
- **Overview bar**: SHOULD not decided 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | complete |
| MUST rows | 0 |
| MUST rows kept (the MUST bar) | 0 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 2 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 2 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc3765.json](../../../data/rfc/rfc3765.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3765-2-1` | The semantics of this attribute is to allow an AS to interpret the presence of this community as an advisory qualification to readvertisement of a route prefix, permitting an AS not to readvertise the route prefix to all external bilateral peer neighbour AS's (§2, §4) | MAY | 2 - NOPEER Attribute, the only definitional section | **positive:** no positive test. **negative:** no negative test |
| `RFC3765-2-2` | It is consistent with these semantics that an AS may filter received prefixes that are received across a peering session that the receiver regards as a bilateral peer sessions (§2, §4) | MAY | 2 - NOPEER Attribute, the only definitional section | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 3765 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

RFC 3765 carries no gated, tagged or audited requirement, so there is no proof state to state.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-rfcgate-4-ledger phase 6 (OR-A) |
| Signed off | 2026-07-30 |
| Register | manual-walk |
| Source | rfc/full/rfc3765.txt |
| Source fingerprint | 288010a8fa4f2407 |
| Record | rfc/extraction/rfc3765.json |
| Mapped sentences | 0 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice and Abstract. The Abstract restates section 1 and states no obligation. |
| `1` | Introduction | 0 | walked | Introduction. Describes the problem (more-specific routes redistributed beyond their useful scope) and names the mechanism. No sentence directs a speaker. |
| `2` | NOPEER Attribute, the only definitional section | 0 | walked | NOPEER Attribute, the only definitional section. Two behavioural sentences, both permissive: an AS is 'permitted' not to readvertise, and 'may filter' received prefixes. Captured as RFC3765-2-1 and RFC3765-2-2 at [MAY]. |
| `3` | Motivation | 0 | walked | Motivation. Routing-table growth statistics and the reasoning that leads to a classification-based rather than enumeration-based redistribution qualifier. Rationale only. |
| `4` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Registers NOPEER as 0xFFFFFF04 with global significance and repeats section 2's advisory sentence verbatim. The registration binds IANA, and the repeated sentence is already captured under section 2. |
| `5` | Security Considerations | 0 | walked | Security Considerations. Analyses the attack surface an unauthorised NOPEER addition opens. States no countermeasure a speaker must apply. |
| `6` | References heading | 0 | skipped (references) | References heading. |
| `6.1` | Normative References: RFC 1997 only | 0 | skipped (references) | Normative References: RFC 1997 only. |
| `6.2` | Informative References: RFC 3221 only | 0 | skipped (references) | Informative References: RFC 3221 only. |
| `7` | Author's Address | 0 | skipped (front-matter) | Author's Address. Document furniture. |
| `8` | Full Copyright Statement | 1 | walked | Full Copyright Statement. Walked because the prose scan attributes its one site here; the site is boilerplate and is excluded below. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The standard IETF copyright boilerplate. 'may be required to implement this standard' is the only lowercase 'required' in the document, and it addresses parties holding patents, not a BGP speaker. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 3765, so its obligations are stated where they were written.
