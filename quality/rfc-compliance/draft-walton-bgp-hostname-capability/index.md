# DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY - Hostname Capability for BGP

Supported. Every requirement this repository extracted from DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 0 | of 1 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
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
| Requirements | 1 |
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
| Summary | `rfc/short/draft-walton-bgp-hostname-capability.md` |
| Requirement shard | `rfc/requirements/draft-walton-bgp-hostname-capability.md` |
| RFC text | `rfc/drafts/draft-walton-bgp-hostname-capability.txt` |

## Enrolment

Enrolled: FQDN capability for BGP (code 73). The draft states NO MUST-level obligation: its only RFC 2119 keyword outside the Section 2 key-words paragraph is one SHOULD in Section 4, so the summary declares zero gated rows and rfc/extraction/draft-walton-bgp-hostname-capability.json signs off under 'manual-walk' with the register-reason that says why zero is a property of the document. Ze sends the capability from per-peer config (encodeValue, internal/component/bgp/plugins/hostname/hostname.go), encodes it with (*FQDN).WriteTo and parses a received one with parseFQDN (internal/core/bgp/capability/capability.go).

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

- The draft states no MUST-level obligation: its only RFC 2119 keyword outside the Section 2 key-words paragraph is one SHOULD in Section 4, which is why [`rfc/extraction/draft-walton-bgp-hostname-capability.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/draft-walton-bgp-hostname-capability.json) signs off under `manual-walk`. Capability code 73, decode support, and per-peer hostname and domain advertisement (`FQDN`, [`internal/core/bgp/capability/capability.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability.go)). Carried as `RFC 8516` in the table above until 2026-08-30
- RFC 8516 is the CoAP "Too Many Requests" response code and has nothing to do with BGP. IANA names this draft as the reference for code 73, and the scoped config keys the capability emits had always spelled it.


**What the ledger says remains:**

-

## Coverage

DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY declares no MUST-level requirement, so the gate counts nothing here.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY-4-1` | "The FQDN Capability SHOULD only be used for displaying the hostname and/or domain name of a speaker in order to make troubleshooting easier" (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY carries no gated, tagged or audited requirement, so there is no proof state to state.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 draft walk, draft-walton-bgp-hostname-capability |
| Signed off | 2026-09-01 |
| Register | manual-walk |
| Source | rfc/drafts/draft-walton-bgp-hostname-capability.txt |
| Source fingerprint | 83f56181a3009274 |
| Record | rfc/extraction/draft-walton-bgp-hostname-capability.json |
| Mapped sentences | 0 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | skipped (front-matter) | Title block, Abstract, Status of This Memo and Copyright Notice. The Abstract says the document introduces a BGP capability carrying a speaker's hostname. It directs no speaker. The one site the scan raises here is the Copyright Notice sentence, excluded below. |
| `1` | Introduction | 0 | walked | Introduction. Says BGP is used inside the data center, that displaying a speaker's hostname eases troubleshooting, and that this document defines a capability exchanging a speaker's FQDN. Indicative throughout; no keyword site and no obligation. |
| `2` | Specification of Requirements | 0 | walked | Specification of Requirements. The RFC 2119 key-words paragraph. It tells a reader how to read the rest and binds nobody, which is why the derivation excludes it from the capitalised site inventory. |
| `3` | FQDN Capability | 0 | walked | FQDN Capability. Defines the wire format: Hostname Length, Hostname, Domain Name Length, Domain Name, with the two strings 'encoded via UTF-8' and the capability code deferred to the IANA Considerations section. Every sentence is a field definition in indicative prose, carried by the Wire Format table of rfc/short/draft-walton-bgp-hostname-capability.md. No RFC 2119 keyword and no obligation on a speaker. |
| `4` | Operation | 0 | walked | Operation. Carries the document's one normative sentence, the SHOULD captured as DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY-4-1: 'The FQDN Capability SHOULD only be used for displaying the hostname and/or domain name of a speaker in order to make troubleshooting easier.' The scan raises no site for it because the row is advisory and the sentence sits below the gated levels this artifact counts, so the id is declared unsourced here. The rest of the section is two pages of sample 'cl-bgp summary' output and the sentence that the hostname and domain are assumed to be taken from the ones set on the device. |
| `5` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA has assigned capability number 73 in the BGP Capability Codes registry. Binds IANA, not a speaker. |
| `6` | Security Considerations | 0 | walked | Security Considerations. One sentence: the document 'introduces no new security concerns to BGP or other specifications referenced in this document'. No countermeasure is directed at a speaker. |
| `7` | References | 0 | skipped (references) | References. The heading over sections 7.1 and 7.2. |
| `7.1` | Normative References: RFC 5492 and RFC 2119 | 0 | skipped (references) | Normative References: RFC 5492 and RFC 2119. |
| `7.2` | not stated | 0 | skipped (references) | Implementation References: the Quagga BGP FQDN Capability commit. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate the splitter did not strip. The sentence binds a person who extracts Code Components from the document into other software and tells them to carry the Simplified BSD License text; it directs no BGP speaker and describes no protocol behavior. Its 'must' is lowercase, so it is raised only by the case-insensitive modal scan the 'prose' register uses. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |

## Superseded

No document obsoletes DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY, so its obligations are stated where they were written.
