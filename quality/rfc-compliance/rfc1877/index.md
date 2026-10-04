# RFC 1877 - PPP Internet Protocol Control Protocol Extensions for Name Server Addresses

Partial. Every requirement this repository extracted from RFC 1877, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 0 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 0 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 0 | of 0 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 0 | of 0 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 0 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 0 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 0 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 0 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

No card above is a share of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 0 |
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
| Summary | `rfc/short/rfc1877.md` |
| Requirement shard | no requirement declared, so no shard is generated |
| RFC text | `rfc/full/rfc1877.txt` |

## Enrolment

Enrolled: PPP IPCP Extensions for Name Server Addresses (DNS options). The document states no MUST-level requirement: its only RFC 2119 keyword is one SHOULD in Section 1, and rfc/extraction/rfc1877.json signs that zero off under manual-walk. The five rows this summary once declared were read from RFC 1661 and were retired on 2026-09-26 (rfc/corrections/rfc1877.md); the Configure-Ack and Configure-Reject echo rules they restated are gated as RFC1661-5.2-1, RFC1661-5.2-2, RFC1661-5.4-1 and RFC1661-5.4-2, where their tests now sit.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

Primary and secondary DNS option parsing and negotiation: Ze offers its configured DNS addresses, Naks a request for them with its own values, and absorbs a peer Configure-Reject of the DNS options while still negotiating the IPv4 address.

**What the ledger says remains:**

Carries the L2TP and PPPoE Partial status. The NBNS options (130, 132) are not implemented.

## Coverage

RFC 1877 declares no MUST-level requirement, so the gate counts nothing here.

## Requirements

RFC 1877 declares no requirement, so this summary generates no shard.

## Gaps and untested MUSTs

RFC 1877 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

RFC 1877 carries no gated, tagged or audited requirement, so there is no proof state to state.

## Extraction sign-off

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
| `1` | Additional IPCP Configuration Options | 0 | walked | Additional IPCP Configuration Options. It introduces options 129 to 132, says primary and secondary addresses are negotiated independently, says the options are 'designed to be identical in format and behavior to option 3 (IP-Address)', and suggests they not be included in the list of "IPCP Recommended Options". It carries the document's only RFC 2119 keyword, the SHOULD quoted in the register-reason; SHOULD is not a gated level and is not a site under either scan, and rfc/short/rfc1877.md declares no row for it. The section directs no MUST at any speaker. The 'identical in format and behavior to option 3' sentence imports the Configure-Ack and Configure-Reject rules of RFC 1661 Sections 5.2 and 5.4 instead of stating them here; RFC 1661's own rows carry them. |
| `1.1` | not stated | 0 | walked | Primary DNS Server Address: Description, packet diagram, Type 129, Length 6, field meaning and Default. Every sentence is indicative. It describes the option's format and what the remote peer typically does ('the remote peer specifies the address by NAKing this option, and returning the IP address of a valid DNS server'), and directs no MUST at a speaker. The Length value stated here, which sections 1.2, 1.3 and 1.4 repeat, carries no rule for an option received with another Length; RFC 1661 Section 6 states that rule, and it calls for a Configure-Nak. |
| `1.2` | not stated | 0 | walked | Primary NBNS Server Address: Type 130, Length 6, and the same Description, diagram, field meaning and Default shape as 1.1, with NBNS in place of DNS. No sentence states an obligation. |
| `1.3` | not stated | 0 | walked | Secondary DNS Server Address: Type 131, Length 6, same shape again. No sentence states an obligation. The field paragraph carries a copy error in RFC 1877's own text, 'The four octet Secondary-DNS-Address is the address of the primary NBNS server to be used by the local peer', where every other field paragraph names its own option; it is a defect of the source, not an obligation, and it is recorded here so a later reader does not read it as one. |
| `1.4` | not stated | 0 | walked | Secondary NBNS Server Address: Type 132, Length 6, same shape, no obligation. This section also carries the whole unnumbered tail of the document, because 'References', 'Security Considerations', 'Chair's Address' and 'Author's Address' head no numbered heading and sectionHeadingRE matches none of them (internal/le/rfc/inventory.go, sectionBodies), so the derivation folds them in here. The tail was walked with the section: the reference list cites RFC 1661, RFC 1332, STD 19 and STD 13 and binds nobody; Security Considerations is one sentence, 'Security issues are not discussed in this memo.', so the document names no countermeasure and no threat; the two address blocks are contact details. Nothing in this section or its tail is MUST-level. |

### Excluded sentences

The walk over RFC 1877 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 1877, so its obligations are stated where they were written.
