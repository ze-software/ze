# RFC 1877 - PPP Internet Protocol Control Protocol Extensions for Name Server Addresses

Partial. Every requirement this repository extracted from RFC 1877, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 3 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 25.0% | 1 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 5 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1877.md` |
| Requirement shard | `rfc/requirements/rfc1877.md` |
| RFC text | `rfc/full/rfc1877.txt` |

## Enrolment

Enrolled: PPP IPCP Extensions for Name Server Addresses (DNS options): four MUST-level requirements. RFC1877-x-1 (link usable for IPv4 with or without DNS) both polarities via existing tests: TestIPCPDNSRejectAbsorbed (a peer Configure-Reject of the DNS options is absorbed and the session survives, still negotiating the IPv4 address) and TestIPCPIPAddressRejectIsFatal (rejecting the IP-Address IS fatal, so DNS not the address is the optional part). RFC1877-x-3 (Configure-Ack echoes the option Data verbatim when acceptable) both polarities via TestRFC1877ConfigureAckEchoesAcceptable: an acceptable request is Acked byte-for-byte (sendNCPConfigureAck writes req.Data unchanged, ncp.go:567) while an unacceptable IP-Address draws a Nak not an Ack. RFC1877-x-4 (Configure-Reject echoes the offending unsupported option) both polarities via TestRFC1877ConfigureRejectEchoesUnsupportedOnly: copyUnknownOptions (ncp.go:619) echoes an unknown type-99 option verbatim and skips the recognized IP-Address option. RFC1877-x-2 (option with Length other than 6 must be Configure-Rejected) is {gap}: Ze validates the length (errIPCPBadOptionLen) but answers a bad-length known-type DNS option with a Configure-Nak rather than a Configure-Reject (its reject path keys on unknown option type, not length); disclosed in the docs/features/rfc-status.md RFC 1877 row. The x-5 MAY is not gated.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Primary and secondary DNS option parsing and negotiation
- the Configure-Ack echoes acceptable options verbatim, the Configure-Reject echoes unsupported ones, and the IPv4 link stays usable with or without DNS. Tests bound per requirement in [`rfc/requirements/rfc1877.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc1877.md).


**What the ledger says remains**

Carries the L2TP and PPPoE Partial status. One MUST gap gated in [`rfc/short/rfc1877.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc1877.md): a DNS option with a Length other than 6 is answered with a Configure-Nak (correcting the value) rather than a Configure-Reject, because Ze's reject path is keyed on unknown option TYPE, not option length.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC1877-x-1`](#rfc1877-x-1), [`RFC1877-x-3`](#rfc1877-x-3), [`RFC1877-x-4`](#rfc1877-x-4)

**Annotated instead of tested (1):** [`RFC1877-x-2`](#rfc1877-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1877-x-1` | Link must still be usable for IPv4 traffic with or without DNS assignment (Scope) | MUST | x | **positive:** `unit/verify` [`TestIPCPDNSRejectAbsorbed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L298). **negative:** `unit/verify` [`TestIPCPIPAddressRejectIsFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L372) |
| `RFC1877-x-2` | Option with Length other than 6 must be Configure-Rejected (Configuration Options) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** A DNS option (Primary/Secondary DNS, type 129/131) with a Length other than 6 is validated by parseIPCPv4Option (internal/component/l2tp/ppp/ipcp.go:100-102, errIPCPBadOptionLen) and flags the Configure-Request as bad in evalIPCPRequest (internal/component/l2tp/ppp/ncp.go:397-400), but Ze responds with a Configure-Nak carrying its own DNS values rather than a Configure-Reject of the malformed option. buildNakOrReject (ncp.go:586-601) takes the Reject branch only for UNKNOWN option TYPES: ipcpHasUnknownOption (ipcp.go:142-158) checks the type, not the length, so a known-type option with a bad length falls to the Nak branch. Disclosed in docs/features/rfc-status.md |
| `RFC1877-x-3` | Configure-Ack must echo the option Data verbatim when value is acceptable (Negotiation Semantics) | MUST | x | **positive:** `unit/verify` [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1877_dns_options_test.go#L13). **negative:** `unit/verify` [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1877_dns_options_test.go#L17) |
| `RFC1877-x-4` | Configure-Reject must echo the offending option when option is not supported (Negotiation Semantics) | MUST | x | **positive:** `unit/verify` [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1877_dns_options_test.go#L66). **negative:** `unit/verify` [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1877_dns_options_test.go#L69) |
| `RFC1877-x-5` | Either side may ignore DNS/NBNS options by Configure-Reject (Scope) | MAY | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1877-x-2`](#rfc1877-x-2) Option with Length other than 6 must be Configure-Rejected (Configuration Options) | {gap}, no test | A DNS option (Primary/Secondary DNS, type 129/131) with a Length other than 6 is validated by parseIPCPv4Option (internal/component/l2tp/ppp/ipcp.go:100-102, errIPCPBadOptionLen) and flags the Configure-Request as bad in evalIPCPRequest (internal/component/l2tp/ppp/ncp.go:397-400), but Ze responds with a Configure-Nak carrying its own DNS values rather than a Configure-Reject of the malformed option. buildNakOrReject (ncp.go:586-601) takes the Reject branch only for UNKNOWN option TYPES: ipcpHasUnknownOption (ipcp.go:142-158) checks the type, not the length, so a known-type option with a bad length falls to the Nak branch. Disclosed in docs/features/rfc-status.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1877-x-1`](#rfc1877-x-1)

Link must still be usable for IPv4 traffic with or without DNS assignment (Scope)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPCPIPAddressRejectIsFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L372) | unit/verify | unproven |
| positive | [`TestIPCPDNSRejectAbsorbed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L298) | unit/verify | unproven |

### [`RFC1877-x-2`](#rfc1877-x-2)

Option with Length other than 6 must be Configure-Rejected (Configuration Options)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1877-x-2, so no unit is bound to it.

### [`RFC1877-x-3`](#rfc1877-x-3)

Configure-Ack must echo the option Data verbatim when value is acceptable (Negotiation Semantics)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1877_dns_options_test.go#L17) | unit/verify | unproven |
| positive | [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1877_dns_options_test.go#L13) | unit/verify | unproven |

### [`RFC1877-x-4`](#rfc1877-x-4)

Configure-Reject must echo the offending option when option is not supported (Negotiation Semantics)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1877_dns_options_test.go#L69) | unit/verify | unproven |
| positive | [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1877_dns_options_test.go#L66) | unit/verify | unproven |

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
| `1` | Additional IPCP Configuration Options | 0 | walked | Additional IPCP Configuration Options. It introduces options 129 to 132, says primary and secondary addresses are negotiated independently, says the options are 'designed to be identical in format and behavior to option 3 (IP-Address)', and suggests they not be included in the list of "IPCP Recommended Options". It carries the document's only RFC 2119 keyword, the SHOULD quoted in the register-reason; SHOULD is not a gated level and is not a site under either scan, and rfc/short/rfc1877.md declares no row for it. The section directs no MUST at any speaker. Three of the summary's four gated rows are read from its indicative prose: RFC1877-x-1 from the 'not ... Recommended Options' suggestion together with the per-option 'Default: No address is provided', and RFC1877-x-3 and RFC1877-x-4 from the 'identical in format and behavior to option 3' sentence, which imports the Configure-Ack and Configure-Reject echo rules of RFC 1661 instead of stating them here. All three are declared unsourced. |
| `1.1` | not stated | 0 | walked | Primary DNS Server Address: Description, packet diagram, Type 129, Length 6, field meaning and Default. Every sentence is indicative. It describes the option's format and what the remote peer typically does ('the remote peer specifies the address by NAKing this option, and returning the IP address of a valid DNS server'), and directs no MUST at a speaker. RFC1877-x-2, the summary's 'Option with Length other than 6 must be Configure-Rejected', is read from the Length value stated here, which sections 1.2, 1.3 and 1.4 then repeat, together with RFC 1661's rule for an option a receiver will not accept. RFC 1877 states no such rule, so the id is declared unsourced once, here, rather than four times. |
| `1.2` | not stated | 0 | walked | Primary NBNS Server Address: Type 130, Length 6, and the same Description, diagram, field meaning and Default shape as 1.1, with NBNS in place of DNS. No sentence states an obligation. Its Length statement is one of the three repetitions RFC1877-x-2 was read from, and that id is declared unsourced on 1.1. |
| `1.3` | not stated | 0 | walked | Secondary DNS Server Address: Type 131, Length 6, same shape again. No sentence states an obligation. The field paragraph carries a copy error in RFC 1877's own text, 'The four octet Secondary-DNS-Address is the address of the primary NBNS server to be used by the local peer', where every other field paragraph names its own option; it is a defect of the source, not an obligation, and it is recorded here so a later reader does not read it as one. |
| `1.4` | not stated | 0 | walked | Secondary NBNS Server Address: Type 132, Length 6, same shape, no obligation. This section also carries the whole unnumbered tail of the document, because 'References', 'Security Considerations', 'Chair's Address' and 'Author's Address' head no numbered heading and sectionHeadingRE matches none of them (internal/le/rfc/inventory.go, sectionBodies), so the derivation folds them in here. The tail was walked with the section: the reference list cites RFC 1661, RFC 1332, STD 19 and STD 13 and binds nobody; Security Considerations is one sentence, 'Security issues are not discussed in this memo.', so the document names no countermeasure and no threat; the two address blocks are contact details. Nothing in this section or its tail is MUST-level. |

### Excluded sentences

The walk over RFC 1877 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 1877, so its obligations are stated where they were written.
