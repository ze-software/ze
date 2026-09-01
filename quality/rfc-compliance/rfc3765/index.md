# RFC 3765 - NOPEER Community for Border Gateway Protocol (BGP) Route Scope Control

Supported. Every requirement this repository extracted from RFC 3765, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Audit verdicts | 0 | of 0 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 0 | of 2 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 0 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 2 |
| Gated MUST-level | 0 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
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

**Status:** Supported

**What the ledger says is covered**

The NOPEER well-known community (0xFFFFFF04): parsing, text output and display. Enrolled 2026-07-30 on an evidenced zero rather than on a MUST. RFC 3765 is Informational and invokes RFC 2119 nowhere. It describes its own mechanism as "an advisory qualification to readvertisement of a route prefix". Both checklist rows in [`rfc/short/rfc3765.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc3765.md) are therefore `[MAY]`, and the document gates nothing. The section-by-section walk behind that claim is recorded in [`rfc/extraction/rfc3765.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc3765.json).

**What the ledger says remains:**

No tracked gap in current source anchors. The route-server path forwards to every client and applies no NOPEER filter. RFC 7947 permits that, because a route server is not a bilateral peer.

## Coverage

RFC 3765 declares no MUST-level requirement, so the gate counts nothing here.

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
