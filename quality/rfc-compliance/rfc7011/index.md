# RFC 7011 - Specification of the IP Flow Information Export (IPFIX) Protocol for the Exchange of Flow Information

Experimental. Every requirement this repository extracted from RFC 7011, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7.4% | 4 of 54 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 9.3% | 5 of 54 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 54 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 54 | of 64 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 9 | of 54 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 16.7% | 9 of 54 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 54 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 54 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 66.7% | 36 of 54 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 54 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 64 |
| Gated MUST-level | 54 |
| Not applicable, so out of scope | 9 |
| Declared gaps | 1 |
| Gated with no test | 35 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7011.md` |
| Requirement shard | `rfc/requirements/rfc7011.md` |
| RFC text | `rfc/full/rfc7011.txt` |

## Enrolment

Enrolled: IP Flow Information Export / IPFIX (RFC 7011): exporter role. The 2026-09-21 extraction walk read all 79 normative sites and the checklist now holds 64 rows: 54 MUST or MUST NOT, 5 SHOULD, 1 RECOMMENDED, 4 MAY. Of the 54: 4 MET (message has >=1 Set, zero-valued Set padding, periodic UDP Template refresh, configurable refresh interval) + 5 single-polarity positive (version 0x000a, padding shorter than a record, Template ID >= 256, no Enterprise Number when E=0, no reduced-size address/timestamp encoding) + 1 gap (no SCTP transport, UDP-only) + 9 not-applicable (no Options Templates, no E=1 enterprise IEs, no variable-length fields, no Template Withdrawals, no SCTP/PR-SCTP, no TCP) + 35 raised by that walk and not yet bound to a test (the Section 6.1 encoding rules, the Section 8.2 Export Time sequencing rules, the Section 10.3 UDP checksum and PMTU rules, and the Section 11.1 and 11.3 TLS/DTLS obligations)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- IPFIX export templates and records over UDP (exporter): version 0x000a, Template IDs >= 256, IANA (E=0) fixed-length field specifiers, zeroed Set padding, and periodic UDP Template refresh at a configurable interval
- tests bound per requirement in [`rfc/requirements/rfc7011.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7011.md).


**What the ledger says remains**

[[`RFC7011-10-1`](#rfc7011-10-1)]: the exporter transmits over UDP only and implements no SCTP transport, so the mandatory SCTP support is absent. [[`RFC7011-11.1-2`](#rfc7011-11.1-2)]: the exporter offers no DTLS for UDP export, which RFC 7011 Section 11.1 makes mandatory. The other 34 MUST rows the 2026-09-21 extraction walk added or raised carry no bound test yet: Section 6.1 encoding rules, Section 8.2 Export Time sequencing, Section 10.3 UDP checksum and PMTU, Section 11.1 and 11.3 TLS/DTLS. ze is an exporter only, and the IPFIX Collecting Process requirements bind a role it does not implement: `internal/plugins/flowexport/` holds encoders and one connected UDP sender (`sender.go:81` net.DialUDP) and opens no listening socket.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 15 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 35 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **54** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7011-3-1`](#rfc7011-3-1), [`RFC7011-3.3.1-1`](#rfc7011-3.3.1-1), [`RFC7011-8-1`](#rfc7011-8-1), [`RFC7011-8-2`](#rfc7011-8-2)

**Annotated instead of tested (15):** [`RFC7011-3.1-1`](#rfc7011-3.1-1), [`RFC7011-3.3.1-2`](#rfc7011-3.3.1-2), [`RFC7011-3.4.1-1`](#rfc7011-3.4.1-1), [`RFC7011-3.4.2-1`](#rfc7011-3.4.2-1), [`RFC7011-3.4.2-2`](#rfc7011-3.4.2-2), [`RFC7011-3.2-1`](#rfc7011-3.2-1), [`RFC7011-3.2-2`](#rfc7011-3.2-2), [`RFC7011-x-1`](#rfc7011-x-1), [`RFC7011-x-2`](#rfc7011-x-2), [`RFC7011-8-3`](#rfc7011-8-3), [`RFC7011-10-1`](#rfc7011-10-1), [`RFC7011-10-2`](#rfc7011-10-2), [`RFC7011-x-3`](#rfc7011-x-3), [`RFC7011-x-4`](#rfc7011-x-4), [`RFC7011-6.2-1`](#rfc7011-6.2-1)

**No test and no annotation (35):** [`RFC7011-3.3.1-3`](#rfc7011-3.3.1-3), [`RFC7011-3.4.2.1-1`](#rfc7011-3.4.2.1-1), [`RFC7011-4.1-1`](#rfc7011-4.1-1), [`RFC7011-4.1-2`](#rfc7011-4.1-2), [`RFC7011-4.1-3`](#rfc7011-4.1-3), [`RFC7011-6.1.1-1`](#rfc7011-6.1.1-1), [`RFC7011-6.1.2-1`](#rfc7011-6.1.2-1), [`RFC7011-6.1.3-1`](#rfc7011-6.1.3-1), [`RFC7011-6.1.4-1`](#rfc7011-6.1.4-1), [`RFC7011-6.1.6-1`](#rfc7011-6.1.6-1), [`RFC7011-6.1.6-2`](#rfc7011-6.1.6-2), [`RFC7011-6.2-3`](#rfc7011-6.2-3), [`RFC7011-7-1`](#rfc7011-7-1), [`RFC7011-8.1-1`](#rfc7011-8.1-1), [`RFC7011-8.2-1`](#rfc7011-8.2-1), [`RFC7011-8.2-2`](#rfc7011-8.2-2), [`RFC7011-8.2-3`](#rfc7011-8.2-3), [`RFC7011-8.2-4`](#rfc7011-8.2-4), [`RFC7011-10-5`](#rfc7011-10-5), [`RFC7011-10.1-1`](#rfc7011-10.1-1), [`RFC7011-10.2.2-1`](#rfc7011-10.2.2-1), [`RFC7011-10.3.2-1`](#rfc7011-10.3.2-1), [`RFC7011-10.3.2-2`](#rfc7011-10.3.2-2), [`RFC7011-10.3.3-1`](#rfc7011-10.3.3-1), [`RFC7011-10.4.1-1`](#rfc7011-10.4.1-1), [`RFC7011-10.4.4-1`](#rfc7011-10.4.4-1), [`RFC7011-11.1-1`](#rfc7011-11.1-1), [`RFC7011-11.1-2`](#rfc7011-11.1-2), [`RFC7011-11.1-3`](#rfc7011-11.1-3), [`RFC7011-11.1-4`](#rfc7011-11.1-4), [`RFC7011-11.3-1`](#rfc7011-11.3-1), [`RFC7011-11.3-2`](#rfc7011-11.3-2), [`RFC7011-11.3-3`](#rfc7011-11.3-3), [`RFC7011-11.3-4`](#rfc7011-11.3-4), [`RFC7011-11-1`](#rfc7011-11-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7011-3-1` | An IPFIX Message MUST contain at least one Set (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7011MessageHasAtLeastOneSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L56). **negative:** `unit/verify` [`TestRFC7011NoEmptyMessageEmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L68) |
| `RFC7011-3.1-1` | Version Number MUST be the value 0x000a (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC7011VersionIsIPFIX`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L101). **negative:** no negative test. **{single-polarity}:** version is the compile-time constant Version = 0x000a written by WriteMessageHeader (internal/plugins/flowexport/ipfix/encoder.go:18,23); no input can alter it, so there is no code path emitting a different version to reject negatively |
| `RFC7011-3.3.1-1` | Padding MUST be composed of octets with value zero (Section 3.3.1) | MUST | 3.3.1 | **positive:** `unit/verify` [`TestRFC7011PaddingIsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L136). **negative:** `unit/verify` [`TestRFC7011PaddingZeroedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L148) |
| `RFC7011-3.3.1-2` | Padding length MUST be shorter than any allowable record in the Set (Section 3.3.1) | MUST | 3.3.1 | **positive:** `unit/verify` [`TestRFC7011PaddingShorterThanRecord`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L160). **negative:** no negative test. **{single-polarity}:** 4-byte-alignment padding is at most 3 octets while the smallest record is 32 octets, so the padLen < recSize guard (internal/plugins/flowexport/ipfix/data.go:47, flow_data.go:68) always takes its true branch and the false branch is unreachable with any real template |
| `RFC7011-3.4.1-1` | Template ID MUST be greater than 255 (Section 3.4.1) | MUST | 3.4.1 | **positive:** `unit/verify` [`TestRFC7011TemplateIDAbove255`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L173). **negative:** no negative test. **{single-polarity}:** Template IDs are the compile-time constants 256/257/258 (internal/plugins/flowexport/ipfix/template.go:10, flow_template.go:11,16); no input produces an ID <= 255, so there is no sub-256 case to reject negatively |
| `RFC7011-3.4.2-1` | Options Template Scope Field Count MUST NOT be zero (Section 3.4.2) | MUST NOT | 3.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter emits only Template Sets (Set ID 2) and never Options Template Sets (Set ID 3); the builders encode no Scope Field Count (internal/plugins/flowexport/ipfix/template.go:48-77, flow_template.go:96-123), so no Options Template Record with a scope field count is produced |
| `RFC7011-3.4.2-2` | An Options Template Record MUST contain at least one Scope Field and at least one non-scope Field (Section 3.4.2) | MUST | 3.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter emits no Options Template Records at all, only Set ID 2 Template Sets (internal/plugins/flowexport/ipfix/template.go:56, flow_template.go:103), so the scope-field / non-scope-field composition rule has no code path |
| `RFC7011-3.2-1` | Enterprise Number MUST NOT be present when E=0 in Field Specifier (Section 3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC7011NoEnterpriseNumberWhenEClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L197). **negative:** no negative test. **{single-polarity}:** every field specifier is the 4-octet E=0 form with bit 15 clear (internal/plugins/flowexport/ipfix/template.go:69-74, flow_template.go:115-119); no code path sets the E bit or appends an Enterprise Number, so the prohibited E=0-with-Enterprise-Number combination cannot be constructed to test negatively |
| `RFC7011-3.2-2` | Enterprise Number MUST be present when E=1 in Field Specifier (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter uses only IANA (E=0) Information Elements (internal/plugins/flowexport/ipfix/ie.go, template.go:69-74); it never sets the Enterprise bit, so no E=1 field specifier is produced and the E=1-requires-Enterprise-Number obligation has no code path |
| `RFC7011-x-1` | Variable-length encoding MUST use short form (1-byte prefix) when value length is 0-254 (Variable-Length IE Encoding) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** every template field specifier declares a fixed Field Length (internal/plugins/flowexport/ipfix/template.go:20-27, flow_template.go:20-48); none uses 65535, so the exporter never emits a variable-length short-form prefix |
| `RFC7011-x-2` | Variable-length encoding MUST use long form (3-byte prefix) when value length is 255-65535 (Variable-Length IE Encoding) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no template field uses Field Length 65535 (internal/plugins/flowexport/ipfix/flow_template.go:20-48), so the exporter never emits a variable-length long-form prefix |
| `RFC7011-8-1` | Over UDP, Exporting Processes MUST periodically retransmit each active Template at regular intervals (Section 8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC7011TemplateRetransmittedAtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L25). **negative:** `unit/verify` [`TestRFC7011TemplateNotRetransmittedBeforeInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L46) |
| `RFC7011-8-2` | Over UDP, the Template retransmission interval MUST be configurable (Section 8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC7011TemplateRefreshConfigurable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L67). **negative:** `unit/verify` [`TestRFC7011TemplateRefreshRangeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L87) |
| `RFC7011-8-3` | Template Withdrawals MUST NOT be sent over UDP (Section 8) | MUST NOT | 8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter is UDP-only (internal/plugins/flowexport/sender.go:82 net.DialUDP) and implements no Template Withdrawal mechanism; the builders always write Field Count = len(fields) > 0 (internal/plugins/flowexport/ipfix/template.go:64, flow_template.go:111), so no Field-Count-0 withdrawal record is ever produced to send |
| `RFC7011-10-1` | An Exporting Process MUST support SCTP (Section 10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the IPFIX exporter transmits over UDP only (internal/plugins/flowexport/sender.go:82 net.DialUDP; internal/plugins/flowexport/config.go:360 accepts sflow/netflow9/ipfix with no SCTP option); SCTP transport is absent, so this mandatory SCTP-support MUST is unmet |
| `RFC7011-10-2` | Templates MUST be sent reliably over SCTP using ordered delivery (Section 10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter implements no SCTP transport (internal/plugins/flowexport/sender.go:82 is UDP-only), so the SCTP ordered-delivery obligation for Templates has no code path; the absence of SCTP itself is the gap recorded under RFC7011-10-1 |
| `RFC7011-x-3` | PR-SCTP MUST NOT be used for Template Records (SCTP Transport) | MUST NOT | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter implements no SCTP or PR-SCTP transport (internal/plugins/flowexport/sender.go:82 net.DialUDP only), so the PR-SCTP prohibition for Template Records has no applicable code path |
| `RFC7011-x-4` | Over TCP, the Exporting Process MUST handle backpressure from congestion control (TCP Transport) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter implements no TCP transport (internal/plugins/flowexport/sender.go uses net.DialUDP; internal/plugins/flowexport/config.go:360-361 accepts only the three UDP protocols), so the TCP backpressure obligation has no code path |
| `RFC7011-6.2-1` | Reduced-size encoding MUST NOT be used for addresses, timestamps, boolean, string, or octetArray (Section 6.2) | MUST NOT | 6.2 | **positive:** `unit/verify` [`TestRFC7011NoReducedSizeForAddressOrTimestamp`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L226). **negative:** no negative test. **{single-polarity}:** the templates hardcode full-width field lengths for every address (4/16) and timestamp (4/8) IE (internal/plugins/flowexport/ipfix/template.go:20-27, flow_template.go:20-48) and the exporter applies no reduced-size encoding to any type, so the prohibited reduced-size-on-address/timestamp combination cannot be produced to test negatively |
| `RFC7011-3.3.1-3` | The record types MUST NOT be mixed within a Set (Section 3.3.1) | MUST NOT | 3.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-3.4.2.1-1` | If a different order of Scope Fields would result in a Record having a different semantic meaning, then the order of Scope Fields MUST be preserved by the Exporting Process (Section 3.4.2.1) | MUST | 3.4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-4.1-1` | The scope Information Element of an Options Template MUST be defined as a Scope Field and MUST be present, unless the Observation Domain ID of the enclosing Message is non-zero (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-4.1-2` | If present, the Options Template's scope Information Element MUST be defined as a Scope Field (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-4.1-3` | If several Metering Processes are available on the Exporter Observation Domain, the Information Element meteringProcessId MUST be specified as an additional Scope Field (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-6.1.1-1` | Integral data types (unsigned8, unsigned16, unsigned32, unsigned64, signed8, signed16, signed32, signed64) MUST be encoded using the default canonical format in network byte order (Section 6.1.1) | MUST | 6.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-6.1.2-1` | Address types (macAddress, ipv4Address, ipv6Address) MUST be encoded the same way as the integral data types, as six, four, and sixteen octets in network byte order (Section 6.1.2) | MUST | 6.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-6.1.3-1` | The float32 data type MUST be encoded as an IEEE binary32 floating point type in network byte order (Section 6.1.3) | MUST | 6.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-6.1.4-1` | The float64 data type MUST be encoded as an IEEE binary64 floating point type in network byte order (Section 6.1.4) | MUST | 6.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-6.1.6-1` | The string data type MUST be encoded in UTF-8 format (Section 6.1.6) | MUST | 6.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-6.1.6-2` | IPFIX Exporting Processes MUST NOT send IPFIX Messages containing ill-formed UTF-8 string values for Information Elements of the string data type (Section 6.1.6) | MUST NOT | 6.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-6.2-3` | Under reduced-size encoding, the signed versus unsigned property of the reported value MUST be preserved (Section 6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-7-1` | The octets carrying the length of a variable-length Information Element (either the first or the first three octets) MUST NOT be included in the length of the Information Element (Section 7) | MUST NOT | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-8.1-1` | A Template Withdrawal's Set ID field MUST contain the value 2 for Template Set Withdrawal or the value 3 for Options Template Set Withdrawal (Section 8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-8.2-1` | An Exporting Process MUST sequence all Template management actions using the Export Time field in the IPFIX Message Header (Section 8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-8.2-2` | An Exporting Process MUST NOT export a Data Set described by a new Template in an IPFIX Message with an Export Time before the Export Time of the IPFIX Message containing that Template (Section 8.2) | MUST NOT | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-8.2-3` | If a new Template and a Data Set described by it appear in the same IPFIX Message, the Template Set containing the Template MUST appear before the Data Set in the Message (Section 8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-8.2-4` | An Exporting Process MUST NOT export any Data Sets described by a withdrawn Template in IPFIX Messages with an Export Time after the Export Time of the Message containing the Template Withdrawal (Section 8.2) | MUST NOT | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10-5` | Transport Session state MUST NOT be migrated by an Exporting Process or Collecting Process among Transport Sessions using different transport protocols between the same pair (Section 10) | MUST NOT | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10.1-1` | It MUST be possible to configure both the Exporting and Collecting Processes to use different ports than the default (Section 10.1) | MUST | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10.2.2-1` | If Data Records are discarded, the IPFIX Sequence Numbers used for export MUST reflect the loss of data (Section 10.2.2) | MUST | 10.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10.3.2-1` | UDP MUST NOT be used unless the application can tolerate some loss of IPFIX Messages (Section 10.3.2) | MUST NOT | 10.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10.3.2-2` | Exporting Processes exporting IPFIX Messages via UDP MUST include a valid UDP checksum in UDP datagrams including IPFIX Messages (Section 10.3.2) | MUST | 10.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10.3.3-1` | The maximum size of exported messages MUST be configured such that the total packet size does not exceed the PMTU (Section 10.3.3) | MUST | 10.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10.4.1-1` | Data Records dropped because the TCP send buffer is full MUST be accounted for, so that the number of lost records can later be reported (Section 10.4.1) | MUST | 10.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10.4.4-1` | In the default configuration, an Exporting Process MUST NOT attempt to establish a connection more frequently than once per minute (Section 10.4.4) | MUST NOT | 10.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11.1-1` | IPFIX Exporting Processes and Collecting Processes using TCP MUST support TLS version 1.1, including the mandatory ciphersuites specified in that version (Section 11.1) | MUST | 11.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11.1-2` | IPFIX Exporting Processes and Collecting Processes using UDP or SCTP MUST support DTLS version 1.0, including the mandatory ciphersuites specified in that version (Section 11.1) | MUST | 11.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11.1-3` | When using DTLS over SCTP, the Exporting Process MUST ensure that each IPFIX Message is sent over the same SCTP Stream that would be used when sending it directly over SCTP (Section 11.1) | MUST | 11.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11.1-4` | Exporting and Collecting Processes MUST NOT request, offer, or use any version of the Secure Socket Layer, or any version of TLS prior to 1.1 (Section 11.1) | MUST NOT | 11.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11.3-1` | Exporting Processes MUST verify the reference identifiers of the Collecting Processes to which they are exporting against those stored in the certificates (Section 11.3) | MUST | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11.3-2` | Exporting Processes MUST NOT export to non-verified Collecting Processes, and Collecting Processes MUST NOT accept IPFIX Messages from non-verified Exporting Processes (Section 11.3) | MUST NOT | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11.3-3` | Exporting Processes and Collecting Processes MUST support the verification of certificates against an explicitly authorized list of peer certificates identified by Common Name (Section 11.3) | MUST | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11.3-4` | IPFIX Exporting Processes and Collecting Processes MUST use non-NULL ciphersuites for authentication, integrity, and confidentiality (Section 11.3) | MUST | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10-3` | Exporting Processes SHOULD support TCP and UDP in addition to SCTP (Section 10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10-4` | Over UDP, IPFIX Messages SHOULD fit within the path MTU to avoid IP fragmentation (Section 10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-x-5` | Over UDP, the Exporting Process SHOULD implement rate limiting or congestion-avoidance mechanisms (UDP Transport) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-x-6` | The Exporting Process over TCP SHOULD use long-lived connections (TCP Transport) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11-1` | To prevent man-in-the-middle attacks from impostor Exporting or Collecting Processes, mutual authentication MUST be used for both TLS and DTLS (Section 11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-x-7` | The Collecting Process SHOULD restrict which Exporting Processes may connect (Security, Access Control) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11-2` | It is RECOMMENDED that IPFIX Exporting and Collecting Processes use TLS or DTLS for all communications (Section 11) | RECOMMENDED | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-x-8` | PR-SCTP MAY be used for Data Records (SCTP Transport) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-x-9` | Data Sets MAY use unordered delivery over SCTP (SCTP Transport) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-8-4` | Over UDP, the Collecting Process MAY expire templates after a configurable timeout (Section 8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-6.2-2` | Reduced-size encoding MAY be used for integer and float types (Section 6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7011-3.4.2-1`](#rfc7011-3.4.2-1) Options Template Scope Field Count MUST NOT be zero (Section 3.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter emits only Template Sets (Set ID 2) and never Options Template Sets (Set ID 3); the builders encode no Scope Field Count (internal/plugins/flowexport/ipfix/template.go:48-77, flow_template.go:96-123), so no Options Template Record with a scope field count is produced |
| [`RFC7011-3.4.2-2`](#rfc7011-3.4.2-2) An Options Template Record MUST contain at least one Scope Field and at least one non-scope Field (Section 3.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter emits no Options Template Records at all, only Set ID 2 Template Sets (internal/plugins/flowexport/ipfix/template.go:56, flow_template.go:103), so the scope-field / non-scope-field composition rule has no code path |
| [`RFC7011-3.2-2`](#rfc7011-3.2-2) Enterprise Number MUST be present when E=1 in Field Specifier (Section 3.2) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter uses only IANA (E=0) Information Elements (internal/plugins/flowexport/ipfix/ie.go, template.go:69-74); it never sets the Enterprise bit, so no E=1 field specifier is produced and the E=1-requires-Enterprise-Number obligation has no code path |
| [`RFC7011-x-1`](#rfc7011-x-1) Variable-length encoding MUST use short form (1-byte prefix) when value length is 0-254 (Variable-Length IE Encoding) | no test | no test carries this requirement id; annotated {not-applicable}: every template field specifier declares a fixed Field Length (internal/plugins/flowexport/ipfix/template.go:20-27, flow_template.go:20-48); none uses 65535, so the exporter never emits a variable-length short-form prefix |
| [`RFC7011-x-2`](#rfc7011-x-2) Variable-length encoding MUST use long form (3-byte prefix) when value length is 255-65535 (Variable-Length IE Encoding) | no test | no test carries this requirement id; annotated {not-applicable}: no template field uses Field Length 65535 (internal/plugins/flowexport/ipfix/flow_template.go:20-48), so the exporter never emits a variable-length long-form prefix |
| [`RFC7011-8-3`](#rfc7011-8-3) Template Withdrawals MUST NOT be sent over UDP (Section 8) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter is UDP-only (internal/plugins/flowexport/sender.go:82 net.DialUDP) and implements no Template Withdrawal mechanism; the builders always write Field Count = len(fields) > 0 (internal/plugins/flowexport/ipfix/template.go:64, flow_template.go:111), so no Field-Count-0 withdrawal record is ever produced to send |
| [`RFC7011-10-1`](#rfc7011-10-1) An Exporting Process MUST support SCTP (Section 10) | {gap}, no test | the IPFIX exporter transmits over UDP only (internal/plugins/flowexport/sender.go:82 net.DialUDP; internal/plugins/flowexport/config.go:360 accepts sflow/netflow9/ipfix with no SCTP option); SCTP transport is absent, so this mandatory SCTP-support MUST is unmet |
| [`RFC7011-10-2`](#rfc7011-10-2) Templates MUST be sent reliably over SCTP using ordered delivery (Section 10) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter implements no SCTP transport (internal/plugins/flowexport/sender.go:82 is UDP-only), so the SCTP ordered-delivery obligation for Templates has no code path; the absence of SCTP itself is the gap recorded under RFC7011-10-1 |
| [`RFC7011-x-3`](#rfc7011-x-3) PR-SCTP MUST NOT be used for Template Records (SCTP Transport) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter implements no SCTP or PR-SCTP transport (internal/plugins/flowexport/sender.go:82 net.DialUDP only), so the PR-SCTP prohibition for Template Records has no applicable code path |
| [`RFC7011-x-4`](#rfc7011-x-4) Over TCP, the Exporting Process MUST handle backpressure from congestion control (TCP Transport) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter implements no TCP transport (internal/plugins/flowexport/sender.go uses net.DialUDP; internal/plugins/flowexport/config.go:360-361 accepts only the three UDP protocols), so the TCP backpressure obligation has no code path |
| [`RFC7011-3.3.1-3`](#rfc7011-3.3.1-3) The record types MUST NOT be mixed within a Set (Section 3.3.1) | no test | no test carries this requirement id |
| [`RFC7011-3.4.2.1-1`](#rfc7011-3.4.2.1-1) If a different order of Scope Fields would result in a Record having a different semantic meaning, then the order of Scope Fields MUST be preserved by the Exporting Process (Section 3.4.2.1) | no test | no test carries this requirement id |
| [`RFC7011-4.1-1`](#rfc7011-4.1-1) The scope Information Element of an Options Template MUST be defined as a Scope Field and MUST be present, unless the Observation Domain ID of the enclosing Message is non-zero (Section 4.1) | no test | no test carries this requirement id |
| [`RFC7011-4.1-2`](#rfc7011-4.1-2) If present, the Options Template's scope Information Element MUST be defined as a Scope Field (Section 4.1) | no test | no test carries this requirement id |
| [`RFC7011-4.1-3`](#rfc7011-4.1-3) If several Metering Processes are available on the Exporter Observation Domain, the Information Element meteringProcessId MUST be specified as an additional Scope Field (Section 4.1) | no test | no test carries this requirement id |
| [`RFC7011-6.1.1-1`](#rfc7011-6.1.1-1) Integral data types (unsigned8, unsigned16, unsigned32, unsigned64, signed8, signed16, signed32, signed64) MUST be encoded using the default canonical format in network byte order (Section 6.1.1) | no test | no test carries this requirement id |
| [`RFC7011-6.1.2-1`](#rfc7011-6.1.2-1) Address types (macAddress, ipv4Address, ipv6Address) MUST be encoded the same way as the integral data types, as six, four, and sixteen octets in network byte order (Section 6.1.2) | no test | no test carries this requirement id |
| [`RFC7011-6.1.3-1`](#rfc7011-6.1.3-1) The float32 data type MUST be encoded as an IEEE binary32 floating point type in network byte order (Section 6.1.3) | no test | no test carries this requirement id |
| [`RFC7011-6.1.4-1`](#rfc7011-6.1.4-1) The float64 data type MUST be encoded as an IEEE binary64 floating point type in network byte order (Section 6.1.4) | no test | no test carries this requirement id |
| [`RFC7011-6.1.6-1`](#rfc7011-6.1.6-1) The string data type MUST be encoded in UTF-8 format (Section 6.1.6) | no test | no test carries this requirement id |
| [`RFC7011-6.1.6-2`](#rfc7011-6.1.6-2) IPFIX Exporting Processes MUST NOT send IPFIX Messages containing ill-formed UTF-8 string values for Information Elements of the string data type (Section 6.1.6) | no test | no test carries this requirement id |
| [`RFC7011-6.2-3`](#rfc7011-6.2-3) Under reduced-size encoding, the signed versus unsigned property of the reported value MUST be preserved (Section 6.2) | no test | no test carries this requirement id |
| [`RFC7011-7-1`](#rfc7011-7-1) The octets carrying the length of a variable-length Information Element (either the first or the first three octets) MUST NOT be included in the length of the Information Element (Section 7) | no test | no test carries this requirement id |
| [`RFC7011-8.1-1`](#rfc7011-8.1-1) A Template Withdrawal's Set ID field MUST contain the value 2 for Template Set Withdrawal or the value 3 for Options Template Set Withdrawal (Section 8.1) | no test | no test carries this requirement id |
| [`RFC7011-8.2-1`](#rfc7011-8.2-1) An Exporting Process MUST sequence all Template management actions using the Export Time field in the IPFIX Message Header (Section 8.2) | no test | no test carries this requirement id |
| [`RFC7011-8.2-2`](#rfc7011-8.2-2) An Exporting Process MUST NOT export a Data Set described by a new Template in an IPFIX Message with an Export Time before the Export Time of the IPFIX Message containing that Template (Section 8.2) | no test | no test carries this requirement id |
| [`RFC7011-8.2-3`](#rfc7011-8.2-3) If a new Template and a Data Set described by it appear in the same IPFIX Message, the Template Set containing the Template MUST appear before the Data Set in the Message (Section 8.2) | no test | no test carries this requirement id |
| [`RFC7011-8.2-4`](#rfc7011-8.2-4) An Exporting Process MUST NOT export any Data Sets described by a withdrawn Template in IPFIX Messages with an Export Time after the Export Time of the Message containing the Template Withdrawal (Section 8.2) | no test | no test carries this requirement id |
| [`RFC7011-10-5`](#rfc7011-10-5) Transport Session state MUST NOT be migrated by an Exporting Process or Collecting Process among Transport Sessions using different transport protocols between the same pair (Section 10) | no test | no test carries this requirement id |
| [`RFC7011-10.1-1`](#rfc7011-10.1-1) It MUST be possible to configure both the Exporting and Collecting Processes to use different ports than the default (Section 10.1) | no test | no test carries this requirement id |
| [`RFC7011-10.2.2-1`](#rfc7011-10.2.2-1) If Data Records are discarded, the IPFIX Sequence Numbers used for export MUST reflect the loss of data (Section 10.2.2) | no test | no test carries this requirement id |
| [`RFC7011-10.3.2-1`](#rfc7011-10.3.2-1) UDP MUST NOT be used unless the application can tolerate some loss of IPFIX Messages (Section 10.3.2) | no test | no test carries this requirement id |
| [`RFC7011-10.3.2-2`](#rfc7011-10.3.2-2) Exporting Processes exporting IPFIX Messages via UDP MUST include a valid UDP checksum in UDP datagrams including IPFIX Messages (Section 10.3.2) | no test | no test carries this requirement id |
| [`RFC7011-10.3.3-1`](#rfc7011-10.3.3-1) The maximum size of exported messages MUST be configured such that the total packet size does not exceed the PMTU (Section 10.3.3) | no test | no test carries this requirement id |
| [`RFC7011-10.4.1-1`](#rfc7011-10.4.1-1) Data Records dropped because the TCP send buffer is full MUST be accounted for, so that the number of lost records can later be reported (Section 10.4.1) | no test | no test carries this requirement id |
| [`RFC7011-10.4.4-1`](#rfc7011-10.4.4-1) In the default configuration, an Exporting Process MUST NOT attempt to establish a connection more frequently than once per minute (Section 10.4.4) | no test | no test carries this requirement id |
| [`RFC7011-11.1-1`](#rfc7011-11.1-1) IPFIX Exporting Processes and Collecting Processes using TCP MUST support TLS version 1.1, including the mandatory ciphersuites specified in that version (Section 11.1) | no test | no test carries this requirement id |
| [`RFC7011-11.1-2`](#rfc7011-11.1-2) IPFIX Exporting Processes and Collecting Processes using UDP or SCTP MUST support DTLS version 1.0, including the mandatory ciphersuites specified in that version (Section 11.1) | no test | no test carries this requirement id |
| [`RFC7011-11.1-3`](#rfc7011-11.1-3) When using DTLS over SCTP, the Exporting Process MUST ensure that each IPFIX Message is sent over the same SCTP Stream that would be used when sending it directly over SCTP (Section 11.1) | no test | no test carries this requirement id |
| [`RFC7011-11.1-4`](#rfc7011-11.1-4) Exporting and Collecting Processes MUST NOT request, offer, or use any version of the Secure Socket Layer, or any version of TLS prior to 1.1 (Section 11.1) | no test | no test carries this requirement id |
| [`RFC7011-11.3-1`](#rfc7011-11.3-1) Exporting Processes MUST verify the reference identifiers of the Collecting Processes to which they are exporting against those stored in the certificates (Section 11.3) | no test | no test carries this requirement id |
| [`RFC7011-11.3-2`](#rfc7011-11.3-2) Exporting Processes MUST NOT export to non-verified Collecting Processes, and Collecting Processes MUST NOT accept IPFIX Messages from non-verified Exporting Processes (Section 11.3) | no test | no test carries this requirement id |
| [`RFC7011-11.3-3`](#rfc7011-11.3-3) Exporting Processes and Collecting Processes MUST support the verification of certificates against an explicitly authorized list of peer certificates identified by Common Name (Section 11.3) | no test | no test carries this requirement id |
| [`RFC7011-11.3-4`](#rfc7011-11.3-4) IPFIX Exporting Processes and Collecting Processes MUST use non-NULL ciphersuites for authentication, integrity, and confidentiality (Section 11.3) | no test | no test carries this requirement id |
| [`RFC7011-11-1`](#rfc7011-11-1) To prevent man-in-the-middle attacks from impostor Exporting or Collecting Processes, mutual authentication MUST be used for both TLS and DTLS (Section 11) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7011-3-1`](#rfc7011-3-1)

An IPFIX Message MUST contain at least one Set (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011NoEmptyMessageEmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L68) | unit/verify | unproven |
| positive | [`TestRFC7011MessageHasAtLeastOneSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L56) | unit/verify | unproven |

### [`RFC7011-3.1-1`](#rfc7011-3.1-1)

Version Number MUST be the value 0x000a (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7011VersionIsIPFIX`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L101) | unit/verify | unproven |

### [`RFC7011-3.3.1-1`](#rfc7011-3.3.1-1)

Padding MUST be composed of octets with value zero (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011PaddingZeroedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L148) | unit/verify | unproven |
| positive | [`TestRFC7011PaddingIsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L136) | unit/verify | unproven |

### [`RFC7011-3.3.1-2`](#rfc7011-3.3.1-2)

Padding length MUST be shorter than any allowable record in the Set (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7011PaddingShorterThanRecord`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L160) | unit/verify | unproven |

### [`RFC7011-3.4.1-1`](#rfc7011-3.4.1-1)

Template ID MUST be greater than 255 (Section 3.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7011TemplateIDAbove255`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L173) | unit/verify | unproven |

### [`RFC7011-3.4.2-1`](#rfc7011-3.4.2-1)

Options Template Scope Field Count MUST NOT be zero (Section 3.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-3.4.2-1, so no unit is bound to it.

### [`RFC7011-3.4.2-2`](#rfc7011-3.4.2-2)

An Options Template Record MUST contain at least one Scope Field and at least one non-scope Field (Section 3.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-3.4.2-2, so no unit is bound to it.

### [`RFC7011-3.2-1`](#rfc7011-3.2-1)

Enterprise Number MUST NOT be present when E=0 in Field Specifier (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7011NoEnterpriseNumberWhenEClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L197) | unit/verify | unproven |

### [`RFC7011-3.2-2`](#rfc7011-3.2-2)

Enterprise Number MUST be present when E=1 in Field Specifier (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-3.2-2, so no unit is bound to it.

### [`RFC7011-x-1`](#rfc7011-x-1)

Variable-length encoding MUST use short form (1-byte prefix) when value length is 0-254 (Variable-Length IE Encoding)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-x-1, so no unit is bound to it.

### [`RFC7011-x-2`](#rfc7011-x-2)

Variable-length encoding MUST use long form (3-byte prefix) when value length is 255-65535 (Variable-Length IE Encoding)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-x-2, so no unit is bound to it.

### [`RFC7011-8-1`](#rfc7011-8-1)

Over UDP, Exporting Processes MUST periodically retransmit each active Template at regular intervals (Section 8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011TemplateNotRetransmittedBeforeInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L46) | unit/verify | unproven |
| positive | [`TestRFC7011TemplateRetransmittedAtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L25) | unit/verify | unproven |

### [`RFC7011-8-2`](#rfc7011-8-2)

Over UDP, the Template retransmission interval MUST be configurable (Section 8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011TemplateRefreshRangeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L87) | unit/verify | unproven |
| positive | [`TestRFC7011TemplateRefreshConfigurable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L67) | unit/verify | unproven |

### [`RFC7011-8-3`](#rfc7011-8-3)

Template Withdrawals MUST NOT be sent over UDP (Section 8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-8-3, so no unit is bound to it.

### [`RFC7011-10-1`](#rfc7011-10-1)

An Exporting Process MUST support SCTP (Section 10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10-1, so no unit is bound to it.

### [`RFC7011-10-2`](#rfc7011-10-2)

Templates MUST be sent reliably over SCTP using ordered delivery (Section 10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10-2, so no unit is bound to it.

### [`RFC7011-x-3`](#rfc7011-x-3)

PR-SCTP MUST NOT be used for Template Records (SCTP Transport)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-x-3, so no unit is bound to it.

### [`RFC7011-x-4`](#rfc7011-x-4)

Over TCP, the Exporting Process MUST handle backpressure from congestion control (TCP Transport)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-x-4, so no unit is bound to it.

### [`RFC7011-6.2-1`](#rfc7011-6.2-1)

Reduced-size encoding MUST NOT be used for addresses, timestamps, boolean, string, or octetArray (Section 6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7011NoReducedSizeForAddressOrTimestamp`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L226) | unit/verify | unproven |

### [`RFC7011-3.3.1-3`](#rfc7011-3.3.1-3)

The record types MUST NOT be mixed within a Set (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-3.3.1-3, so no unit is bound to it.

### [`RFC7011-3.4.2.1-1`](#rfc7011-3.4.2.1-1)

If a different order of Scope Fields would result in a Record having a different semantic meaning, then the order of Scope Fields MUST be preserved by the Exporting Process (Section 3.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-3.4.2.1-1, so no unit is bound to it.

### [`RFC7011-4.1-1`](#rfc7011-4.1-1)

The scope Information Element of an Options Template MUST be defined as a Scope Field and MUST be present, unless the Observation Domain ID of the enclosing Message is non-zero (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-4.1-1, so no unit is bound to it.

### [`RFC7011-4.1-2`](#rfc7011-4.1-2)

If present, the Options Template's scope Information Element MUST be defined as a Scope Field (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-4.1-2, so no unit is bound to it.

### [`RFC7011-4.1-3`](#rfc7011-4.1-3)

If several Metering Processes are available on the Exporter Observation Domain, the Information Element meteringProcessId MUST be specified as an additional Scope Field (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-4.1-3, so no unit is bound to it.

### [`RFC7011-6.1.1-1`](#rfc7011-6.1.1-1)

Integral data types (unsigned8, unsigned16, unsigned32, unsigned64, signed8, signed16, signed32, signed64) MUST be encoded using the default canonical format in network byte order (Section 6.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.1.1-1, so no unit is bound to it.

### [`RFC7011-6.1.2-1`](#rfc7011-6.1.2-1)

Address types (macAddress, ipv4Address, ipv6Address) MUST be encoded the same way as the integral data types, as six, four, and sixteen octets in network byte order (Section 6.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.1.2-1, so no unit is bound to it.

### [`RFC7011-6.1.3-1`](#rfc7011-6.1.3-1)

The float32 data type MUST be encoded as an IEEE binary32 floating point type in network byte order (Section 6.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.1.3-1, so no unit is bound to it.

### [`RFC7011-6.1.4-1`](#rfc7011-6.1.4-1)

The float64 data type MUST be encoded as an IEEE binary64 floating point type in network byte order (Section 6.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.1.4-1, so no unit is bound to it.

### [`RFC7011-6.1.6-1`](#rfc7011-6.1.6-1)

The string data type MUST be encoded in UTF-8 format (Section 6.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.1.6-1, so no unit is bound to it.

### [`RFC7011-6.1.6-2`](#rfc7011-6.1.6-2)

IPFIX Exporting Processes MUST NOT send IPFIX Messages containing ill-formed UTF-8 string values for Information Elements of the string data type (Section 6.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.1.6-2, so no unit is bound to it.

### [`RFC7011-6.2-3`](#rfc7011-6.2-3)

Under reduced-size encoding, the signed versus unsigned property of the reported value MUST be preserved (Section 6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.2-3, so no unit is bound to it.

### [`RFC7011-7-1`](#rfc7011-7-1)

The octets carrying the length of a variable-length Information Element (either the first or the first three octets) MUST NOT be included in the length of the Information Element (Section 7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-7-1, so no unit is bound to it.

### [`RFC7011-8.1-1`](#rfc7011-8.1-1)

A Template Withdrawal's Set ID field MUST contain the value 2 for Template Set Withdrawal or the value 3 for Options Template Set Withdrawal (Section 8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-8.1-1, so no unit is bound to it.

### [`RFC7011-8.2-1`](#rfc7011-8.2-1)

An Exporting Process MUST sequence all Template management actions using the Export Time field in the IPFIX Message Header (Section 8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-8.2-1, so no unit is bound to it.

### [`RFC7011-8.2-2`](#rfc7011-8.2-2)

An Exporting Process MUST NOT export a Data Set described by a new Template in an IPFIX Message with an Export Time before the Export Time of the IPFIX Message containing that Template (Section 8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-8.2-2, so no unit is bound to it.

### [`RFC7011-8.2-3`](#rfc7011-8.2-3)

If a new Template and a Data Set described by it appear in the same IPFIX Message, the Template Set containing the Template MUST appear before the Data Set in the Message (Section 8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-8.2-3, so no unit is bound to it.

### [`RFC7011-8.2-4`](#rfc7011-8.2-4)

An Exporting Process MUST NOT export any Data Sets described by a withdrawn Template in IPFIX Messages with an Export Time after the Export Time of the Message containing the Template Withdrawal (Section 8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-8.2-4, so no unit is bound to it.

### [`RFC7011-10-5`](#rfc7011-10-5)

Transport Session state MUST NOT be migrated by an Exporting Process or Collecting Process among Transport Sessions using different transport protocols between the same pair (Section 10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10-5, so no unit is bound to it.

### [`RFC7011-10.1-1`](#rfc7011-10.1-1)

It MUST be possible to configure both the Exporting and Collecting Processes to use different ports than the default (Section 10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.1-1, so no unit is bound to it.

### [`RFC7011-10.2.2-1`](#rfc7011-10.2.2-1)

If Data Records are discarded, the IPFIX Sequence Numbers used for export MUST reflect the loss of data (Section 10.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.2.2-1, so no unit is bound to it.

### [`RFC7011-10.3.2-1`](#rfc7011-10.3.2-1)

UDP MUST NOT be used unless the application can tolerate some loss of IPFIX Messages (Section 10.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.3.2-1, so no unit is bound to it.

### [`RFC7011-10.3.2-2`](#rfc7011-10.3.2-2)

Exporting Processes exporting IPFIX Messages via UDP MUST include a valid UDP checksum in UDP datagrams including IPFIX Messages (Section 10.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.3.2-2, so no unit is bound to it.

### [`RFC7011-10.3.3-1`](#rfc7011-10.3.3-1)

The maximum size of exported messages MUST be configured such that the total packet size does not exceed the PMTU (Section 10.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.3.3-1, so no unit is bound to it.

### [`RFC7011-10.4.1-1`](#rfc7011-10.4.1-1)

Data Records dropped because the TCP send buffer is full MUST be accounted for, so that the number of lost records can later be reported (Section 10.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.4.1-1, so no unit is bound to it.

### [`RFC7011-10.4.4-1`](#rfc7011-10.4.4-1)

In the default configuration, an Exporting Process MUST NOT attempt to establish a connection more frequently than once per minute (Section 10.4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.4.4-1, so no unit is bound to it.

### [`RFC7011-11.1-1`](#rfc7011-11.1-1)

IPFIX Exporting Processes and Collecting Processes using TCP MUST support TLS version 1.1, including the mandatory ciphersuites specified in that version (Section 11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.1-1, so no unit is bound to it.

### [`RFC7011-11.1-2`](#rfc7011-11.1-2)

IPFIX Exporting Processes and Collecting Processes using UDP or SCTP MUST support DTLS version 1.0, including the mandatory ciphersuites specified in that version (Section 11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.1-2, so no unit is bound to it.

### [`RFC7011-11.1-3`](#rfc7011-11.1-3)

When using DTLS over SCTP, the Exporting Process MUST ensure that each IPFIX Message is sent over the same SCTP Stream that would be used when sending it directly over SCTP (Section 11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.1-3, so no unit is bound to it.

### [`RFC7011-11.1-4`](#rfc7011-11.1-4)

Exporting and Collecting Processes MUST NOT request, offer, or use any version of the Secure Socket Layer, or any version of TLS prior to 1.1 (Section 11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.1-4, so no unit is bound to it.

### [`RFC7011-11.3-1`](#rfc7011-11.3-1)

Exporting Processes MUST verify the reference identifiers of the Collecting Processes to which they are exporting against those stored in the certificates (Section 11.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.3-1, so no unit is bound to it.

### [`RFC7011-11.3-2`](#rfc7011-11.3-2)

Exporting Processes MUST NOT export to non-verified Collecting Processes, and Collecting Processes MUST NOT accept IPFIX Messages from non-verified Exporting Processes (Section 11.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.3-2, so no unit is bound to it.

### [`RFC7011-11.3-3`](#rfc7011-11.3-3)

Exporting Processes and Collecting Processes MUST support the verification of certificates against an explicitly authorized list of peer certificates identified by Common Name (Section 11.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.3-3, so no unit is bound to it.

### [`RFC7011-11.3-4`](#rfc7011-11.3-4)

IPFIX Exporting Processes and Collecting Processes MUST use non-NULL ciphersuites for authentication, integrity, and confidentiality (Section 11.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.3-4, so no unit is bound to it.

### [`RFC7011-11-1`](#rfc7011-11-1)

To prevent man-in-the-middle attacks from impostor Exporting or Collecting Processes, mutual authentication MUST be used for both TLS and DTLS (Section 11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7011.txt |
| Source fingerprint | 21fa7c3fe6dbb5b9 |
| Record | rfc/extraction/rfc7011.json |
| Mapped sentences | 48 |
| Declined as scope | 31 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 4 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.3.1` | not stated | 3 | walked | not stated |
| `3.3.2` | not stated | 1 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.4.1` | not stated | 1 | walked | not stated |
| `3.4.2` | not stated | 0 | walked | not stated |
| `3.4.2.1` | not stated | 1 | walked | not stated |
| `3.4.2.2` | not stated | 2 | walked | not stated |
| `3.4.3` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `4.1` | not stated | 3 | walked | not stated |
| `4.2` | not stated | 3 | walked | not stated |
| `4.3` | not stated | 1 | walked | not stated |
| `4.4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.1.1` | not stated | 1 | walked | not stated |
| `6.1.2` | not stated | 1 | walked | not stated |
| `6.1.3` | not stated | 1 | walked | not stated |
| `6.1.4` | not stated | 1 | walked | not stated |
| `6.1.5` | not stated | 0 | walked | not stated |
| `6.1.6` | not stated | 2 | walked | not stated |
| `6.1.7` | not stated | 0 | walked | not stated |
| `6.1.8` | not stated | 0 | walked | not stated |
| `6.1.9` | not stated | 1 | walked | not stated |
| `6.1.10` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 2 | walked | not stated |
| `7` | not stated | 3 | walked | not stated |
| `8` | not stated | 5 | walked | not stated |
| `8.1` | not stated | 3 | walked | not stated |
| `8.2` | not stated | 4 | walked | not stated |
| `8.3` | not stated | 2 | walked | not stated |
| `8.4` | not stated | 4 | walked | not stated |
| `9` | not stated | 3 | walked | not stated |
| `9.1` | not stated | 1 | walked | not stated |
| `9.2` | not stated | 1 | walked | not stated |
| `9.3` | not stated | 0 | walked | not stated |
| `10` | not stated | 2 | walked | not stated |
| `10.1` | not stated | 2 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |
| `10.2.1` | not stated | 0 | walked | not stated |
| `10.2.2` | not stated | 1 | walked | not stated |
| `10.2.3` | not stated | 0 | walked | not stated |
| `10.2.4` | not stated | 1 | walked | not stated |
| `10.2.5` | not stated | 0 | walked | not stated |
| `10.2.6` | not stated | 0 | walked | not stated |
| `10.3` | not stated | 0 | walked | not stated |
| `10.3.1` | not stated | 0 | walked | not stated |
| `10.3.2` | not stated | 2 | walked | not stated |
| `10.3.3` | not stated | 1 | walked | not stated |
| `10.3.4` | not stated | 0 | walked | not stated |
| `10.3.5` | not stated | 0 | walked | not stated |
| `10.4` | not stated | 0 | walked | not stated |
| `10.4.1` | not stated | 1 | walked | not stated |
| `10.4.2` | not stated | 0 | walked | not stated |
| `10.4.3` | not stated | 0 | walked | not stated |
| `10.4.4` | not stated | 2 | walked | not stated |
| `10.4.5` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `11.1` | not stated | 4 | walked | not stated |
| `11.2` | not stated | 0 | walked | not stated |
| `11.3` | not stated | 6 | walked | not stated |
| `11.4` | not stated | 0 | walked | not stated |
| `11.5` | not stated | 0 | walked | not stated |
| `11.6` | not stated | 1 | walked | not stated |
| `11.7` | not stated | 0 | walked | not stated |
| `11.8` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `A.1` | not stated | 0 | walked | not stated |
| `A.2` | not stated | 0 | walked | not stated |
| `A.2.1` | not stated | 0 | walked | not stated |
| `A.2.2` | not stated | 0 | walked | not stated |
| `A.3` | not stated | 0 | walked | not stated |
| `A.4` | not stated | 0 | walked | not stated |
| `A.4.1` | not stated | 0 | walked | not stated |
| `A.4.2` | not stated | 0 | walked | not stated |
| `A.4.3` | not stated | 0 | walked | not stated |
| `A.4.4` | not stated | 0 | walked | not stated |
| `A.5` | not stated | 0 | walked | not stated |
| `A.5.1` | not stated | 0 | walked | not stated |
| `A.5.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the E=0 prohibition already mapped at site 3.2:1, in the field-specifier field description | If this bit is zero, the Information Element identifier identifies an Information Element in [IANA-IPFIX], and the four-octet Enterprise Number field MUST NOT be present. |
| `3.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the E=1 obligation already mapped at site 3.2:2, in the field-specifier field description | If this bit is one, the Information Element identifier identifies an enterprise-specific Information Element, and the Enterprise Number field MUST be present. |
| `3.3.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds the IPFIX Collecting Process, which walks Sets to find the next one; ze implements no collector. The ipfix package encodes and sends only (producer internal/plugins/flowexport/ipfix/encoder.go WriteMessageHeader), and the single transport is the connected UDP socket at internal/plugins/flowexport/sender.go:81 net.DialUDP, which never listens No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | Because an individual Set MAY contain multiple records, the Length value MUST be used to determine the position of the next Set. |
| `3.4.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names Collecting Processes; ze implements no IPFIX collector. Producer that would act as the role is absent: internal/plugins/flowexport/sender.go:81 net.DialUDP is a connected sender and internal/plugins/flowexport/ipfix/ holds encoders only No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | As Exporting Processes are free to allocate Template IDs as they see fit, Collecting Processes MUST NOT assume incremental Template IDs, or anything about the contents of a Template based on its Template ID alone. |
| `3.4.2.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names Collecting Processes reading Options Template IDs; ze implements no IPFIX collector. Producer absent: internal/plugins/flowexport/sender.go:81 net.DialUDP sends only, internal/plugins/flowexport/ipfix/ decodes nothing No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | As Exporting Processes are free to allocate Template IDs as they see fit, Collecting Processes MUST NOT assume incremental Template IDs, or anything about the contents of an Options Template based on its Template ID alone. |
| `4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process checking Options Template Records; ze implements no IPFIX collector. Producer absent: internal/plugins/flowexport/ipfix/ encodes only and internal/plugins/flowexport/sender.go:81 net.DialUDP never listens No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | The Collecting Process MUST check the possible combinations of Information Elements within the Options Template Records to correctly interpret the following Options Templates. |
| `4.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Exporting Process Statistics Options Template repeats the Metering Process Statistics sentence already mapped at site 4.1:1, word for word | This Information Element MUST be defined as a Scope Field and MUST be present, unless the Observation Domain ID of the enclosing Message is non-zero. |
| `4.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the sentence already mapped at site 4.1:2, word for word | If present, this Information Element MUST be defined as a Scope Field. |
| `4.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the meteringProcessId sentence already mapped at site 4.1:3, word for word | Note that if several Metering Processes are available on the Exporter Observation Domain, the Information Element meteringProcessId MUST be specified as an additional Scope Field. |
| `4.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Metering Process Reliability Statistics template restates the scope-field obligation mapped at site 4.1:2 for its own scope Information Element | This Information Element MUST be defined as a Scope Field. |
| `4.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Flow Keys Options Template restates the scope-field obligation mapped at site 4.1:2 for its own scope Information Element | This Information Element MUST be defined as a Scope Field. |
| `6.1.9:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the MUST binds the process that DECODES a dateTimeNanoseconds value and must ignore the bottom 11 bits of the Fraction field; ze implements no IPFIX collector, so nothing on its side reads the field. Producer absent: internal/plugins/flowexport/ipfix/ holds encoders only and internal/plugins/flowexport/sender.go:81 net.DialUDP never listens No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | Therefore, the bottom 11 bits of the Fraction field SHOULD be zero and MUST be ignored for all Information Elements of this data type (as 2^11 x 233 picoseconds = .477 microseconds). |
| `8:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process storing Template Records; ze implements no IPFIX collector. Producer absent: internal/plugins/flowexport/ipfix/ encodes only, internal/plugins/flowexport/sender.go:81 net.DialUDP never listens No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | The Collecting Process MUST store all received Template Record information for the duration of each Transport Session until reuse or withdrawal as described in Section 8.1, or expiry over UDP as described in Section 8.4, so that it can interpret the corresponding Data Records. |
| `8:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process and Template reuse across Transport Sessions; ze implements no IPFIX collector (internal/plugins/flowexport/sender.go:81 net.DialUDP sends only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | The Collecting Process MUST NOT assume that the Template IDs from a given Exporting Process refer to the same Templates as they did in previous Transport Sessions from the same Exporting Process; a Collecting Process MUST NOT use Templates from one Transport Session to decode Data Sets in a subsequent Transport Session. |
| `8:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names Collecting Processes handling identical Information Elements; ze implements no IPFIX collector (internal/plugins/flowexport/ipfix/ encodes only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | Collecting Processes MUST properly handle Templates with multiple identical Information Elements. |
| `8:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process and what it may assume about Message layout; ze implements no IPFIX collector (internal/plugins/flowexport/sender.go:81 net.DialUDP sends only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | However, a Collecting Process MUST NOT assume that the Data Set and the associated Template Set (or Options Template Set) are exported in the same IPFIX Message. |
| `8:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the MUST names Collecting Processes handling one Template ID in several Observation Domains; ze implements no IPFIX collector (internal/plugins/flowexport/ipfix/ encodes only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | Different Observation Domains within a Transport Session MAY use the same Template ID value to refer to different Templates; Collecting Processes MUST properly handle this case. |
| `8.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process reacting to a Template Withdrawal; ze implements no IPFIX collector (internal/plugins/flowexport/sender.go:81 net.DialUDP sends only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | After receiving a Template Withdrawal, a Collecting Process MUST stop using the Template to interpret subsequently exported Data Sets. |
| `8.1:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process ignoring an out-of-order Template Withdrawal; ze implements no IPFIX collector (internal/plugins/flowexport/ipfix/ encodes only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | The continued receipt and interpretation of Data Records are still possible, but the Collecting Process MUST ignore the Template Withdrawal and SHOULD log the error. |
| `8.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the SCTP ordered-delivery obligation mapped at site 8.3:1, for Template Withdrawals instead of Template Sets | Template Withdrawals MUST be sent reliably, using SCTP ordered delivery. |
| `8.4:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process replacing a Template on UDP redefinition; ze implements no IPFIX collector (internal/plugins/flowexport/sender.go:81 net.DialUDP sends only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | When a Collecting Process receives a new Template Record or Options Template Record for an already-allocated Template ID, and that Template or Options Template is different from the already-received Template or Options Template, the Collecting Process MUST replace the Template or Options Template for that Template ID with the newly received Template or Options Template. |
| `9:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process listening for association requests; ze implements no IPFIX collector and opens no listening socket (internal/plugins/flowexport/sender.go:81 net.DialUDP is a connected sender) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | The Collecting Process MUST listen for association requests / connections to start new Transport Sessions from the Exporting Process. |
| `9:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process noting unknown Information Elements; ze implements no IPFIX collector (internal/plugins/flowexport/ipfix/ encodes only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | The Collecting Process MUST note the Information Element identifier of any Information Element that it does not understand and MAY discard that Information Element from received Data Records. |
| `9:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process accepting padding; ze implements no IPFIX collector (internal/plugins/flowexport/ipfix/ encodes only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | The Collecting Process MUST accept padding in Data Records and Template Records. |
| `9.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process discarding a malformed IPFIX Message; ze implements no IPFIX collector (internal/plugins/flowexport/sender.go:81 net.DialUDP sends only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | If the Collecting Process receives a malformed IPFIX Message, it MUST discard the IPFIX Message and SHOULD log the error. |
| `9.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process opening multiple SCTP Streams; ze implements no IPFIX collector and no SCTP transport (internal/plugins/flowexport/sender.go:81 net.DialUDP) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | As an Exporting Process may request and support more than one stream per SCTP association, the Collecting Process MUST support the opening of multiple SCTP Streams. |
| `10:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process handling 65535-octet Messages; ze implements no IPFIX collector (internal/plugins/flowexport/ipfix/ encodes only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | A Collecting Process MUST be able to handle IPFIX Message lengths of up to 65535 octets. |
| `10.2.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process re-listening after an aborted SCTP association; ze implements no IPFIX collector and no SCTP transport (internal/plugins/flowexport/sender.go:81 net.DialUDP) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | When a Collecting Process detects that the SCTP association has been abnormally terminated, it MUST continue to listen for a new association establishment. |
| `10.4.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names the Collecting Process re-listening after an abnormal TCP close; ze implements no IPFIX collector and no TCP transport (internal/plugins/flowexport/sender.go:81 net.DialUDP) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | When a Collecting Process detects that the TCP connection to the Exporting Process has terminated abnormally, it MUST continue to listen for a new connection. |
| `11.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the sentence opens with Likewise and restates for the Collecting Process the reference-identifier verification mapped at site 11.3:2 | Likewise, Collecting Processes MUST verify the reference identifiers of the Exporting Processes from which they are receiving IPFIX Messages against those stored in the certificates. |
| `11.6:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence names IPFIX Collecting Processes detecting insertion or loss by Sequence Number; ze implements no IPFIX collector (internal/plugins/flowexport/sender.go:81 net.DialUDP sends only) No producer acts as the IPFIX Collecting Process in this tree: internal/plugins/flowexport/sender.go (line 81, net.DialUDP) is the only IPFIX transport and it is a connected sender that never listens. | IPFIX Collecting Processes MUST detect potential IPFIX Message insertion or loss conditions by tracking the IPFIX Sequence Number and SHOULD provide a logging mechanism for reporting out-of-sequence messages. |

## Superseded

No document obsoletes RFC 7011, so its obligations are stated where they were written.
