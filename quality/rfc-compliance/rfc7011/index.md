# RFC 7011 - Specification of the IP Flow Information Export (IPFIX) Protocol for the Exchange of Flow Information

Experimental. Every requirement this repository extracted from RFC 7011, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 24.0% | 12 of 50 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.0% | 6 of 50 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 50 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 50 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 77.5% | 31 of 40 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 50 | of 57 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 23 | of 50 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 26.0% | 13 of 50 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 50 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 20.0% | 10 of 50 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 18.0% | 9 of 50 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 50 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
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
| Requirements | 57 |
| Gated MUST-level | 50 |
| Not applicable, so out of scope | 13 |
| Declared gaps | 9 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 40 |
| Tagged units | 40 |
| Recorded audit verdicts | 13 |
| Discrimination records | 31 |
| Summary | `rfc/short/rfc7011.md` |
| Requirement shard | `rfc/requirements/rfc7011.md` |
| RFC text | `rfc/full/rfc7011.txt` |

## Enrolment

Enrolled: IP Flow Information Export / IPFIX (RFC 7011): exporter role. The 2026-09-21 extraction walk read all 79 normative sites and the checklist now holds 64 rows: 54 MUST or MUST NOT, 5 SHOULD, 1 RECOMMENDED, 4 MAY. Of the 54: 4 MET (message has >=1 Set, zero-valued Set padding, periodic UDP Template refresh, configurable refresh interval) + 5 single-polarity positive (version 0x000a, padding shorter than a record, Template ID >= 256, no Enterprise Number when E=0, no reduced-size address/timestamp encoding) + 2 gaps (no SCTP transport, UDP-only; no DTLS) + 9 not-applicable (no Options Templates, no E=1 enterprise IEs, no variable-length fields, no Template Withdrawals, no SCTP/PR-SCTP, no TCP) + the rows raised by that walk, each now tested or annotated: the TLS/DTLS authentication rows are gaps, the Options Template, reduced-size and TCP rows are feature-declined, and the float, string, withdrawal and single-transport rows are not-applicable

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- IPFIX export templates and records over UDP (exporter): version 0x000a, Template IDs >= 256, IANA (E=0) fixed-length field specifiers, zeroed Set padding, and periodic UDP Template refresh at a configurable interval
- tests bound per requirement in [`rfc/requirements/rfc7011.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7011.md).


**What the ledger says remains**

[[`RFC7011-10-1`](#rfc7011-10-1)]: the exporter transmits over UDP only and implements no SCTP transport, so the mandatory SCTP support is absent. [[`RFC7011-11.1-2`](#rfc7011-11.1-2)]: the exporter offers no DTLS for UDP export, which RFC 7011 Section 11.1 makes mandatory. [[`RFC7011-11.1-3`](#rfc7011-11.1-3)] [[`RFC7011-11-1`](#rfc7011-11-1)] [[`RFC7011-11.3-1`](#rfc7011-11.3-1)] [[`RFC7011-11.3-2`](#rfc7011-11.3-2)] [[`RFC7011-11.3-3`](#rfc7011-11.3-3)] [[`RFC7011-11.3-4`](#rfc7011-11.3-4)]: with no DTLS, the exporter has no DTLS over SCTP, no mutual authentication, no certificate verification and no ciphersuite, and it exports to unverified collectors. [[`RFC7011-10.3.2-1`](#rfc7011-10.3.2-1)]: UDP is the only transport, so an application that cannot tolerate loss has no alternative. Options Templates, reduced-size encoding and TCP transport are optional features the exporter does not implement. ze is an exporter only, and the IPFIX Collecting Process requirements bind a role it does not implement: `internal/plugins/flowexport/` holds encoders and one connected UDP sender (`sender.go:81` net.DialUDP) and opens no listening socket.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 12 | one part of the gated population |
| Annotated (including scoped evidence) | 38 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **50** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (12):** [`RFC7011-3.3.1-1`](#rfc7011-3.3.1-1), [`RFC7011-8-1`](#rfc7011-8-1), [`RFC7011-8-2`](#rfc7011-8-2), [`RFC7011-8-3`](#rfc7011-8-3), [`RFC7011-3.3.1-3`](#rfc7011-3.3.1-3), [`RFC7011-6.1.1-1`](#rfc7011-6.1.1-1), [`RFC7011-6.1.2-1`](#rfc7011-6.1.2-1), [`RFC7011-8.2-1`](#rfc7011-8.2-1), [`RFC7011-8.2-2`](#rfc7011-8.2-2), [`RFC7011-8.2-3`](#rfc7011-8.2-3), [`RFC7011-10.1-1`](#rfc7011-10.1-1), [`RFC7011-10.3.3-1`](#rfc7011-10.3.3-1)

**Annotated (including scoped evidence) (38):** [`RFC7011-3.1-1`](#rfc7011-3.1-1), [`RFC7011-3.3.1-2`](#rfc7011-3.3.1-2), [`RFC7011-3.4.1-1`](#rfc7011-3.4.1-1), [`RFC7011-3.4.2-1`](#rfc7011-3.4.2-1), [`RFC7011-3.2-1`](#rfc7011-3.2-1), [`RFC7011-3.2-2`](#rfc7011-3.2-2), [`RFC7011-7-2`](#rfc7011-7-2), [`RFC7011-x-2`](#rfc7011-x-2), [`RFC7011-10-1`](#rfc7011-10-1), [`RFC7011-10-2`](#rfc7011-10-2), [`RFC7011-6.2-1`](#rfc7011-6.2-1), [`RFC7011-3.4.2.1-1`](#rfc7011-3.4.2.1-1), [`RFC7011-4.1-1`](#rfc7011-4.1-1), [`RFC7011-4.1-2`](#rfc7011-4.1-2), [`RFC7011-4.1-3`](#rfc7011-4.1-3), [`RFC7011-6.1.3-1`](#rfc7011-6.1.3-1), [`RFC7011-6.1.4-1`](#rfc7011-6.1.4-1), [`RFC7011-6.1.6-1`](#rfc7011-6.1.6-1), [`RFC7011-6.1.6-2`](#rfc7011-6.1.6-2), [`RFC7011-6.2-3`](#rfc7011-6.2-3), [`RFC7011-7-1`](#rfc7011-7-1), [`RFC7011-8.1-1`](#rfc7011-8.1-1), [`RFC7011-8.2-4`](#rfc7011-8.2-4), [`RFC7011-10-5`](#rfc7011-10-5), [`RFC7011-10.2.2-1`](#rfc7011-10.2.2-1), [`RFC7011-10.3.2-1`](#rfc7011-10.3.2-1), [`RFC7011-10.3.2-2`](#rfc7011-10.3.2-2), [`RFC7011-10.4.1-1`](#rfc7011-10.4.1-1), [`RFC7011-10.4.4-1`](#rfc7011-10.4.4-1), [`RFC7011-11.1-1`](#rfc7011-11.1-1), [`RFC7011-11.1-2`](#rfc7011-11.1-2), [`RFC7011-11.1-3`](#rfc7011-11.1-3), [`RFC7011-11.1-4`](#rfc7011-11.1-4), [`RFC7011-11.3-1`](#rfc7011-11.3-1), [`RFC7011-11.3-2`](#rfc7011-11.3-2), [`RFC7011-11.3-3`](#rfc7011-11.3-3), [`RFC7011-11.3-4`](#rfc7011-11.3-4), [`RFC7011-11-1`](#rfc7011-11-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7011-3.1-1` | The value of this field is 0x000a for the current version (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC7011VersionIsIPFIX`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L102). **negative:** no negative test. **{single-polarity}:** version is the compile-time constant Version = 0x000a written by WriteMessageHeader (internal/plugins/flowexport/ipfix/encoder.go:18,23); no input can alter it, so there is no code path emitting a different version to reject negatively |
| `RFC7011-3.3.1-1` | For security reasons, the padding octet(s) MUST be composed of octets with value zero (0). (Section 3.3.1) | MUST | 3.3.1 | **positive:** `unit/verify` [`TestRFC7011PaddingIsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L137). **negative:** `unit/verify` [`TestRFC7011PaddingZeroedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L149) |
| `RFC7011-3.3.1-2` | The padding length MUST be shorter than any allowable record in this Set. (Section 3.3.1) | MUST | 3.3.1 | **positive:** `unit/verify` [`TestRFC7011PaddingShorterThanRecord`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L161). **negative:** no negative test. **{single-polarity}:** 4-byte-alignment padding is at most 3 octets while the smallest record is 32 octets, so the padLen < recSize guard (internal/plugins/flowexport/ipfix/data.go:47, flow_data.go:68) always takes its true branch and the false branch is unreachable with any real template |
| `RFC7011-3.4.1-1` | Each Template Record is given a unique Template ID in the range 256 to 65535. (Section 3.4.1) | MUST | 3.4.1 | **positive:** `unit/verify` [`TestRFC7011TemplateIDAbove255`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L174). **positive:** `unit/verify` [`TestRFC7011TemplateIDsUniqueInSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/integral_rfc7011_test.go#L84). **negative:** no negative test. **{single-polarity}:** Template IDs are the compile-time constants 256/257/258 (internal/plugins/flowexport/ipfix/template.go:10, flow_template.go:11,16); no input produces an ID <= 255, so there is no sub-256 case to reject negatively |
| `RFC7011-3.4.2-1` | The Scope Field Count MUST NOT be zero. (Section 3.4.2) | MUST NOT | 3.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter emits only Template Sets (Set ID 2) and never Options Template Sets (Set ID 3); the builders encode no Scope Field Count (internal/plugins/flowexport/ipfix/template.go:48-77, flow_template.go:96-123), so no Options Template Record with a scope field count is produced |
| `RFC7011-3.2-1` | When the Enterprise bit is set to 0, the corresponding Information Element appears in [IANA-IPFIX], and the Enterprise Number MUST NOT be present. (Section 3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestIPFIXTemplateSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_template_test.go#L9). **positive:** `unit/verify` [`TestRFC7011NoEnterpriseNumberWhenEClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L198). **negative:** no negative test. **{single-polarity}:** every field specifier is the 4-octet E=0 form with bit 15 clear (internal/plugins/flowexport/ipfix/template.go:69-74, flow_template.go:115-119); no code path sets the E bit or appends an Enterprise Number, so the prohibited E=0-with-Enterprise-Number combination cannot be constructed to test negatively |
| `RFC7011-3.2-2` | When the Enterprise bit is set to 1, the corresponding Information Element identifier identified an enterprise-specific Information Element; the Enterprise Number MUST be present. (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter uses only IANA (E=0) Information Elements (internal/plugins/flowexport/ipfix/ie.go, template.go:69-74); it never sets the Enterprise bit, so no E=1 field specifier is produced and the E=1-requires-Enterprise-Number obligation has no code path |
| `RFC7011-7-2` | Where an Information Element has a variable length, the following mechanism MUST be used to carry the length information for both the IANA-assigned and enterprise-specific Information Elements. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "Information Elements of type octetArray and string may be exported using any length, subject to restrictions on length specific to each Information Element"; the exporter sends no variable-length Information Element and so writes no length prefix: internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate declare a fixed Field Length for every field and none of 65535, and export no octetArray or string Information Element |
| `RFC7011-x-2` | The length may also be encoded into 3 octets before the Information Element, allowing the length of the Information Element to be greater than or equal to 255 octets. In this case, the first octet of the Length field MUST be 255, and the length is carried in the second and third octets, as shown in Figure S. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no template field uses Field Length 65535 (internal/plugins/flowexport/ipfix/flow_template.go:20-48), so the exporter never emits a variable-length long-form prefix |
| `RFC7011-8-1` | Since UDP provides no method for reliable transmission of Templates, Exporting Processes using UDP as the transport protocol MUST periodically retransmit each active Template at regular intervals. (Section 8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC7011EachActiveTemplateRetransmittedEveryInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_template_refresh_test.go#L43). **positive:** `unit/verify` [`TestRFC7011TemplateRetransmittedAtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L25). **negative:** `unit/verify` [`TestRFC7011TemplateNotRetransmittedBeforeInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L46) |
| `RFC7011-8-2` | The Template retransmission interval MUST be configurable via, for example, the templateRefreshTimeout and optionsTemplateRefreshTimeout parameters as defined in [RFC6728]. (Section 8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC7011ConfiguredRefreshDrivesRetransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_template_refresh_test.go#L76). **positive:** `unit/verify` [`TestRFC7011TemplateRefreshConfigurable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L67). **negative:** `unit/verify` [`TestRFC7011ConfiguredRefreshOverridesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_template_refresh_test.go#L109). **negative:** `unit/verify` [`TestRFC7011TemplateRefreshRangeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L87) |
| `RFC7011-8-3` | Template Withdrawals (Section 8.1) MUST NOT be sent by Exporting Processes exporting via UDP (§8.4) | MUST NOT | 8.4 | **positive:** `unit/verify` [`TestRFC7011UDPExporterSendsNoTemplateWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_withdrawal_test.go#L161). **negative:** `unit/verify` [`TestRFC7011UDPExporterReloadSendsNoTemplateWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_withdrawal_test.go#L201) |
| `RFC7011-10-1` | SCTP [RFC4960] using the Partially Reliable SCTP (PR-SCTP) extension as specified in [RFC3758] MUST be implemented by all compliant implementations. (§10.1) | MUST | 10.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the IPFIX exporter transmits over UDP only (internal/plugins/flowexport/sender.go:82 net.DialUDP; internal/plugins/flowexport/config.go:360 accepts sflow/netflow9/ipfix with no SCTP option); SCTP transport is absent, so this mandatory SCTP-support MUST is unmet |
| `RFC7011-10-2` | Template Sets and Options Template Sets MUST be sent reliably, using SCTP ordered delivery. (§8.3) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter implements no SCTP transport (internal/plugins/flowexport/sender.go:82 is UDP-only), so the SCTP ordered-delivery obligation for Templates has no code path; the absence of SCTP itself is the gap recorded under RFC7011-10-1 |
| `RFC7011-6.2-1` | Reduced-size encoding MUST NOT be applied to any other data type defined in [RFC7012] that implies a fixed length, as these types either have internal structure (such as ipv4Address or dateTimeMicroseconds) or restricted ranges that are not suitable for reduced-size encoding (such as dateTimeMilliseconds). (Section 6.2) | MUST NOT | 6.2 | **positive:** `unit/verify` [`TestRFC7011NoReducedSizeForAddressOrTimestamp`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L227). **negative:** no negative test. **{single-polarity}:** the templates hardcode full-width field lengths for every address (4/16) and timestamp (4/8) IE (internal/plugins/flowexport/ipfix/template.go:20-27, flow_template.go:20-48) and the exporter applies no reduced-size encoding to any type, so the prohibited reduced-size-on-address/timestamp combination cannot be produced to test negatively |
| `RFC7011-3.3.1-3` | The record types MUST NOT be mixed within a Set (Section 3.3.1) | MUST NOT | 3.3.1 | **positive:** `unit/verify` [`TestRFC7011OneRecordTypePerSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L88). **negative:** `unit/verify` [`TestRFC7011RecordTypesNotMixedInSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L120) |
| `RFC7011-3.4.2.1-1` | If a different order of Scope Fields would result in a Record having a different semantic meaning, then the order of Scope Fields MUST be preserved by the Exporting Process (Section 3.4.2.1) | MUST | 3.4.2.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "Multiple Scope Fields MAY be present in the Options Template Record"; the exporter writes no Options Template Record and so no Scope Field whose order could matter. internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate build only Set ID 2 Template Sets |
| `RFC7011-4.1-1` | This Information Element MUST be defined as a Scope Field and MUST be present, unless the Observation Domain ID of the enclosing Message is non-zero. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The Options Template and Options Template Records defined in these subsections, which impose some constraints on the Metering Process and Exporting Process implementations, MAY be implemented."; the exporter does not implement the Metering Process Statistics Options Template: internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate build only Set ID 2 Template Sets with no scope field |
| `RFC7011-4.1-2` | If present, this Information Element MUST be defined as a Scope Field. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The Options Template and Options Template Records defined in these subsections, which impose some constraints on the Metering Process and Exporting Process implementations, MAY be implemented."; the exporter does not implement the Metering Process Statistics Options Template: internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate build only Set ID 2 Template Sets with no scope field |
| `RFC7011-4.1-3` | Note that if several Metering Processes are available on the Exporter Observation Domain, the Information Element meteringProcessId MUST be specified as an additional Scope Field. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The Options Template and Options Template Records defined in these subsections, which impose some constraints on the Metering Process and Exporting Process implementations, MAY be implemented."; the exporter does not implement the Metering Process Statistics Options Template: internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate build only Set ID 2 Template Sets with no scope field |
| `RFC7011-6.1.1-1` | Integral data types -- unsigned8, unsigned16, unsigned32, unsigned64, signed8, signed16, signed32, and signed64 -- MUST be encoded using the default canonical format in network byte order. (Section 6.1.1) | MUST | 6.1.1 | **positive:** `unit/verify` [`TestIPFIXFlowData`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_flow_data_test.go#L10). **positive:** `unit/verify` [`TestRFC7011CounterRecordIntegralsNetworkByteOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/integral_rfc7011_test.go#L32). **positive:** `unit/verify` [`TestRFC7011IntegralNetworkByteOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L163). **negative:** `unit/verify` [`TestRFC7011CounterRecordIntegralsNeverLittleEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/integral_rfc7011_test.go#L55). **negative:** `unit/verify` [`TestRFC7011IntegralNeverLittleEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L187) |
| `RFC7011-6.1.2-1` | Address types -- macAddress, ipv4Address, and ipv6Address -- MUST be encoded the same way as the integral data types, as six, four, and sixteen octets in network byte order, respectively. (Section 6.1.2) | MUST | 6.1.2 | **positive:** `unit/verify` [`TestRFC7011AddressOctetsNetworkByteOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L202). **negative:** `unit/verify` [`TestRFC7011IPv4NeverWidenedToSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L231) |
| `RFC7011-6.1.3-1` | The float32 data type MUST be encoded as an IEEE binary32 floating point type as specified in [IEEE.754.2008], in network byte order as specified in Section 3.6 of [RFC1014]. (Section 6.1.3) | MUST | 6.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no exported IE is of type float32; the IANA registry gives every IE the templates declare (counterTemplateFields and flowTemplateFields in internal/plugins/flowexport/ipfix) an unsigned, address or dateTime abstract type, so no float32 value is ever encoded |
| `RFC7011-6.1.4-1` | The float64 data type MUST be encoded as an IEEE binary64 floating point type as specified in [IEEE.754.2008], in network byte order as specified in Section 3.7 of [RFC1014]. (Section 6.1.4) | MUST | 6.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no exported IE is of type float64; the IANA registry gives every IE the templates declare (counterTemplateFields and flowTemplateFields in internal/plugins/flowexport/ipfix) an unsigned, address or dateTime abstract type, so no float64 value is ever encoded |
| `RFC7011-6.1.6-1` | The string data type MUST be encoded in UTF-8 [RFC3629] format. (Section 6.1.6) | MUST | 6.1.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no exported IE is of type string; the IANA registry gives every IE the templates declare (counterTemplateFields and flowTemplateFields in internal/plugins/flowexport/ipfix) an unsigned, address or dateTime abstract type, so no string value is ever encoded |
| `RFC7011-6.1.6-2` | IPFIX Exporting Processes MUST NOT send IPFIX Messages containing ill-formed UTF-8 string values for Information Elements of the string data type (Section 6.1.6) | MUST NOT | 6.1.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no exported IE is of type string; the IANA registry gives every IE the templates declare (counterTemplateFields and flowTemplateFields in internal/plugins/flowexport/ipfix) an unsigned, address or dateTime abstract type, so no UTF-8 string value, well-formed or not, is ever sent |
| `RFC7011-6.2-3` | The signed versus unsigned property of the reported value MUST be preserved. (Section 6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "Reduced-size encoding MAY be applied to the following integer types"; the exporter applies no reduced-size encoding. internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate declare every integer IE at its full registry width (unsigned64 counters at 8 octets, unsigned32 at 4), so no reduced value exists whose sign could be lost |
| `RFC7011-7-1` | The octets carrying the length (either the first or the first three octets) MUST NOT be included in the length of the Information Element. (Section 7) | MUST NOT | 7 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "Information Elements of type octetArray and string may be exported using any length, subject to restrictions on length specific to each Information Element"; the exporter writes no variable-length length octets: internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate declare a fixed Field Length for every field and none of 65535, and export no octetArray or string Information Element |
| `RFC7011-8.1-1` | Figure T: Template Withdrawal Format The Set ID field MUST contain the value 2 for Template Set Withdrawal or the value 3 for Options Template Set Withdrawal. (Section 8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 7011 Section 8 forbids Template Withdrawals over UDP (RFC7011-8-3) and the exporter is UDP only (internal/plugins/flowexport/sender.go::NewSender calls net.DialUDP); the builders always write Field Count = len(fields) > 0, so no withdrawal Set is ever written |
| `RFC7011-8.2-1` | Since there is no guarantee of the ordering of exported IPFIX Messages across SCTP Streams or over UDP, an Exporting Process MUST sequence all Template management actions (i.e., Template Records defining new Templates and Template Withdrawals withdrawing them) using the Export Time field in the IPFIX Message Header. (Section 8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestRFC7011CounterTemplateExportTimeIsSendTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_template_time_test.go#L19). **positive:** `unit/verify` [`TestRFC7011TemplateExportTimeIsSendTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L246). **negative:** `unit/verify` [`TestRFC7011TemplateExportTimeNeverRegresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L265). **negative:** `unit/verify` [`TestRFC7011TemplateRefreshAfterClockRollbackKeepsOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_template_time_test.go#L35) |
| `RFC7011-8.2-2` | An Exporting Process MUST NOT export a Data Set described by a new Template in an IPFIX Message with an Export Time before the Export Time of the IPFIX Message containing that Template (Section 8.2) | MUST NOT | 8.2 | **positive:** `unit/verify` [`TestRFC7011DataExportTimeNotBeforeTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L286). **negative:** `unit/verify` [`TestRFC7011StaleSnapshotClampedToTemplateTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L308) |
| `RFC7011-8.2-3` | If a new Template and a Data Set described by it appear in the same IPFIX Message, the Template Set containing the Template MUST appear before the Data Set in the Message (Section 8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestRFC7011TemplateSetPrecedesDataSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L483). **negative:** `unit/verify` [`TestRFC7011DataSetNeverPrecedesTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L500) |
| `RFC7011-8.2-4` | An Exporting Process MUST NOT export any Data Sets described by a withdrawn Template in IPFIX Messages with an Export Time after the Export Time of the IPFIX Message containing the Template Withdrawal withdrawing that Template. (Section 8.2) | MUST NOT | 8.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter never withdraws a Template (RFC7011-8-3 forbids withdrawals over UDP and internal/plugins/flowexport/sender.go::NewSender is UDP only), so no Data Set follows a withdrawal |
| `RFC7011-10-5` | Transport Session state MUST NOT be migrated by an Exporting Process or Collecting Process among Transport Sessions using different transport protocols between the same Exporting Process and Collecting Process pair. (Section 10) | MUST NOT | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter implements one transport protocol, UDP (internal/plugins/flowexport/sender.go::NewSender calls net.DialUDP), so no Transport Session of a second protocol exists to migrate state into; the absent SCTP is the gap under RFC7011-10-1 |
| `RFC7011-10.1-1` | It MUST be possible to configure both the Exporting and Collecting Processes to use different ports than the default (Section 10.1) | MUST | 10.1 | **positive:** `unit/verify` [`TestRFC7011CollectorPortConfigurable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_config_test.go#L20). **negative:** `unit/verify` [`TestRFC7011CollectorPortOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_config_test.go#L74) |
| `RFC7011-10.2.2-1` | If Data Records are discarded, the IPFIX Sequence Numbers used for export MUST reflect the loss of data. (Section 10.2.2) | MUST | 10.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter implements no SCTP transport (internal/plugins/flowexport/sender.go::NewSender is UDP only), so no SCTP export discards records; the absent SCTP is the gap under RFC7011-10-1 |
| `RFC7011-10.3.2-1` | UDP MUST NOT be used unless the application can tolerate some loss of IPFIX Messages (Section 10.3.2) | MUST NOT | 10.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the exporter offers UDP as its only transport (internal/plugins/flowexport/sender.go::NewSender calls net.DialUDP), so an operator whose application cannot tolerate lost IPFIX Messages has no reliable transport to select; this follows from the SCTP gap under RFC7011-10-1 |
| `RFC7011-10.3.2-2` | Exporting Processes exporting IPFIX Messages via UDP MUST include a valid UDP checksum [UDP] in UDP datagrams including IPFIX Messages. (Section 10.3.2) | MUST | 10.3.2 | **positive:** `unit/verify` [`TestRFC7011UDPChecksumEnabledOnSenderSocket`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_sender_linux_test.go#L38). **negative:** no negative test. **{single-polarity}:** Linux computes the UDP checksum unless the socket sets SO_NO_CHECK or UDP_NO_CHECK6_TX, and internal/plugins/flowexport/sender.go::NewSender never sets either, so no code path yields a checksum-less datagram to assert against |
| `RFC7011-10.3.3-1` | The maximum size of exported messages MUST be configured such that the total packet size does not exceed the PMTU (Section 10.3.3) | MUST | 10.3.3 | **positive:** `unit/verify` [`TestRFC7011MaxDatagramSizeConfigured`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_datagram_test.go#L23). **negative:** `unit/verify` [`TestRFC7011MaxDatagramSizeOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_datagram_test.go#L56) |
| `RFC7011-10.4.1-1` | The dropped Data Records MUST be accounted for, so that the number of lost records can later be reported as described in Section 4.3. (Section 10.4.1) | MUST | 10.4.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "TCP [TCP] MAY also be implemented by compliant implementations."; the exporter implements no TCP transport, so no TCP send buffer drops records: internal/plugins/flowexport/sender.go::NewSender opens a UDP socket only |
| `RFC7011-10.4.4-1` | In the default configuration, an Exporting Process MUST NOT attempt to establish a connection more frequently than once per minute (Section 10.4.4) | MUST NOT | 10.4.4 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "TCP [TCP] MAY also be implemented by compliant implementations."; the exporter implements no TCP transport and establishes no connection: internal/plugins/flowexport/sender.go::NewSender opens a connectionless UDP socket only |
| `RFC7011-11.1-1` | IPFIX Exporting Processes and Collecting Processes using TCP MUST support TLS version 1.1 and SHOULD support TLS version 1.2 [RFC5246], including the mandatory ciphersuite(s) specified in each version. (Section 11.1) | MUST | 11.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "TCP [TCP] MAY also be implemented by compliant implementations."; the exporter implements no TCP transport, so the TLS-over-TCP obligation has no path: internal/plugins/flowexport/sender.go::NewSender opens a UDP socket only |
| `RFC7011-11.1-2` | IPFIX Exporting Processes and Collecting Processes using UDP or SCTP MUST support DTLS version 1.0 and SHOULD support DTLS version 1.2 [RFC6347], including the mandatory ciphersuite(s) specified in each version. (Section 11.1) | MUST | 11.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the IPFIX exporter has no DTLS. NewSender in internal/plugins/flowexport/sender.go opens a plain connected UDP socket with net.DialUDP, and Send writes unencrypted datagrams to it. No flowexport configuration selects DTLS. |
| `RFC7011-11.1-3` | When using DTLS over SCTP, the Exporting Process MUST ensure that each IPFIX Message is sent over the same SCTP Stream that would be used when sending the same IPFIX Message directly over SCTP. (Section 11.1) | MUST | 11.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the exporter has neither SCTP nor DTLS (internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket), so DTLS over SCTP is absent; both absent capabilities are mandatory and recorded under RFC7011-10-1 and RFC7011-11.1-2 |
| `RFC7011-11.1-4` | Exporting and Collecting Processes MUST NOT request, offer, or use any version of the Secure Socket Layer (SSL), or any version of TLS prior to 1.1, due to known security vulnerabilities in prior versions of TLS; see Appendix E of [RFC5246] for more information. (Section 11.1) | MUST NOT | 11.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the exporter negotiates no SSL, TLS or DTLS of any version (internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket and Send writes cleartext), so it never requests, offers or uses SSL or TLS before 1.1; the missing DTLS is the gap under RFC7011-11.1-2 |
| `RFC7011-11.3-1` | Exporting Processes MUST verify the reference identifiers of the Collecting Processes to which they are exporting IPFIX Messages against those stored in the certificates. (Section 11.3) | MUST | 11.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the exporter has no DTLS or TLS and so verifies no certificate reference identifier; internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket to the configured address |
| `RFC7011-11.3-2` | Exporting Processes MUST NOT export to non-verified Collecting Processes, and Collecting Processes MUST NOT accept IPFIX Messages from non-verified Exporting Processes (Section 11.3) | MUST NOT | 11.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the exporter exports to any configured Collecting Process without verifying it; internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket with no DTLS or TLS authentication |
| `RFC7011-11.3-3` | Exporting Processes and Collecting Processes MUST support the verification of certificates against an explicitly authorized list of peer certificates identified by Common Name (Section 11.3) | MUST | 11.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the exporter has no certificate verification and no authorized-certificate list; internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket with no DTLS or TLS |
| `RFC7011-11.3-4` | IPFIX Exporting Processes and Collecting Processes MUST use non-NULL ciphersuites for authentication, integrity, and confidentiality (Section 11.3) | MUST | 11.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the exporter uses no ciphersuite at all; internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket and Send writes cleartext IPFIX Messages |
| `RFC7011-10-3` | UDP [UDP] MAY also be implemented by compliant implementations. TCP [TCP] MAY also be implemented by compliant implementations. (§10.1) | MAY | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-x-5` | UDP may be used, although it is not a congestion-aware protocol. However, in this case the IPFIX traffic between the Exporter and Collector must be separately contained or provisioned to minimize the risk of congestion-related loss. (§10.1) | SHOULD | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11-1` | To prevent man-in-the-middle attacks from impostor Exporting or Collecting Processes, the acceptance of data from an unauthorized Exporting Process, or the export of data to an unauthorized Collecting Process, mutual authentication MUST be used for both TLS and DTLS. (Section 11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the exporter has no DTLS or TLS and therefore no mutual authentication; internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket |
| `RFC7011-x-7` | In any case, the use of open Collecting Processes (those that will accept IPFIX Messages from any Exporting Process regardless of IP address or identity) is discouraged. (§11.5) | SHOULD | 11.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-x-8` | An IPFIX Exporting Process MAY use any PR-SCTP service definition as per Section 4 of the PR-SCTP specification [RFC3758] when using partial reliability to transmit IPFIX Messages containing only Data Sets. (§10.2.6) | MAY | 10.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-x-9` | Depending on the requirements of the application, the Exporting Process may send Data Sets with full or partial reliability, using ordered or out-of-order delivery, over any SCTP Stream established during SCTP association setup. (§10.2.6) | MAY | 10.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-8-4` | In order to minimize resource requirements for Templates that are no longer being used by the Exporting Process, the Collecting Process MAY associate a lifetime with each Template received in a Transport Session. Templates not refreshed by the Exporting Process within the lifetime can then be discarded by the Collecting Process. The Template lifetime at the Collecting Process MAY be exposed by a configuration parameter or MAY be derived from observation of the interval of periodic Template retransmissions from the Exporting Process. (§8.4) | MAY | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-6.2-2` | Information Elements encoded as signed, unsigned, or float data types MAY be encoded using fewer octets than those implied by their type in the information model definition (Section 6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7011-3.4.2-1`](#rfc7011-3.4.2-1) The Scope Field Count MUST NOT be zero. (Section 3.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter emits only Template Sets (Set ID 2) and never Options Template Sets (Set ID 3); the builders encode no Scope Field Count (internal/plugins/flowexport/ipfix/template.go:48-77, flow_template.go:96-123), so no Options Template Record with a scope field count is produced |
| [`RFC7011-3.2-2`](#rfc7011-3.2-2) When the Enterprise bit is set to 1, the corresponding Information Element identifier identified an enterprise-specific Information Element; the Enterprise Number MUST be present. (Section 3.2) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter uses only IANA (E=0) Information Elements (internal/plugins/flowexport/ipfix/ie.go, template.go:69-74); it never sets the Enterprise bit, so no E=1 field specifier is produced and the E=1-requires-Enterprise-Number obligation has no code path |
| [`RFC7011-7-2`](#rfc7011-7-2) Where an Information Element has a variable length, the following mechanism MUST be used to carry the length information for both the IANA-assigned and enterprise-specific Information Elements. (§7) | no test | no test carries this requirement id; annotated {feature-declined}: "Information Elements of type octetArray and string may be exported using any length, subject to restrictions on length specific to each Information Element"; the exporter sends no variable-length Information Element and so writes no length prefix: internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate declare a fixed Field Length for every field and none of 65535, and export no octetArray or string Information Element |
| [`RFC7011-x-2`](#rfc7011-x-2) The length may also be encoded into 3 octets before the Information Element, allowing the length of the Information Element to be greater than or equal to 255 octets. In this case, the first octet of the Length field MUST be 255, and the length is carried in the second and third octets, as shown in Figure S. (§7) | no test | no test carries this requirement id; annotated {not-applicable}: no template field uses Field Length 65535 (internal/plugins/flowexport/ipfix/flow_template.go:20-48), so the exporter never emits a variable-length long-form prefix |
| [`RFC7011-10-1`](#rfc7011-10-1) SCTP [RFC4960] using the Partially Reliable SCTP (PR-SCTP) extension as specified in [RFC3758] MUST be implemented by all compliant implementations. (§10.1) | {gap}, no test | the IPFIX exporter transmits over UDP only (internal/plugins/flowexport/sender.go:82 net.DialUDP; internal/plugins/flowexport/config.go:360 accepts sflow/netflow9/ipfix with no SCTP option); SCTP transport is absent, so this mandatory SCTP-support MUST is unmet |
| [`RFC7011-10-2`](#rfc7011-10-2) Template Sets and Options Template Sets MUST be sent reliably, using SCTP ordered delivery. (§8.3) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter implements no SCTP transport (internal/plugins/flowexport/sender.go:82 is UDP-only), so the SCTP ordered-delivery obligation for Templates has no code path; the absence of SCTP itself is the gap recorded under RFC7011-10-1 |
| [`RFC7011-3.4.2.1-1`](#rfc7011-3.4.2.1-1) If a different order of Scope Fields would result in a Record having a different semantic meaning, then the order of Scope Fields MUST be preserved by the Exporting Process (Section 3.4.2.1) | no test | no test carries this requirement id; annotated {feature-declined}: "Multiple Scope Fields MAY be present in the Options Template Record"; the exporter writes no Options Template Record and so no Scope Field whose order could matter. internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate build only Set ID 2 Template Sets |
| [`RFC7011-4.1-1`](#rfc7011-4.1-1) This Information Element MUST be defined as a Scope Field and MUST be present, unless the Observation Domain ID of the enclosing Message is non-zero. (Section 4.1) | no test | no test carries this requirement id; annotated {feature-declined}: "The Options Template and Options Template Records defined in these subsections, which impose some constraints on the Metering Process and Exporting Process implementations, MAY be implemented."; the exporter does not implement the Metering Process Statistics Options Template: internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate build only Set ID 2 Template Sets with no scope field |
| [`RFC7011-4.1-2`](#rfc7011-4.1-2) If present, this Information Element MUST be defined as a Scope Field. (Section 4.1) | no test | no test carries this requirement id; annotated {feature-declined}: "The Options Template and Options Template Records defined in these subsections, which impose some constraints on the Metering Process and Exporting Process implementations, MAY be implemented."; the exporter does not implement the Metering Process Statistics Options Template: internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate build only Set ID 2 Template Sets with no scope field |
| [`RFC7011-4.1-3`](#rfc7011-4.1-3) Note that if several Metering Processes are available on the Exporter Observation Domain, the Information Element meteringProcessId MUST be specified as an additional Scope Field. (Section 4.1) | no test | no test carries this requirement id; annotated {feature-declined}: "The Options Template and Options Template Records defined in these subsections, which impose some constraints on the Metering Process and Exporting Process implementations, MAY be implemented."; the exporter does not implement the Metering Process Statistics Options Template: internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate build only Set ID 2 Template Sets with no scope field |
| [`RFC7011-6.1.3-1`](#rfc7011-6.1.3-1) The float32 data type MUST be encoded as an IEEE binary32 floating point type as specified in [IEEE.754.2008], in network byte order as specified in Section 3.6 of [RFC1014]. (Section 6.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: no exported IE is of type float32; the IANA registry gives every IE the templates declare (counterTemplateFields and flowTemplateFields in internal/plugins/flowexport/ipfix) an unsigned, address or dateTime abstract type, so no float32 value is ever encoded |
| [`RFC7011-6.1.4-1`](#rfc7011-6.1.4-1) The float64 data type MUST be encoded as an IEEE binary64 floating point type as specified in [IEEE.754.2008], in network byte order as specified in Section 3.7 of [RFC1014]. (Section 6.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: no exported IE is of type float64; the IANA registry gives every IE the templates declare (counterTemplateFields and flowTemplateFields in internal/plugins/flowexport/ipfix) an unsigned, address or dateTime abstract type, so no float64 value is ever encoded |
| [`RFC7011-6.1.6-1`](#rfc7011-6.1.6-1) The string data type MUST be encoded in UTF-8 [RFC3629] format. (Section 6.1.6) | no test | no test carries this requirement id; annotated {not-applicable}: no exported IE is of type string; the IANA registry gives every IE the templates declare (counterTemplateFields and flowTemplateFields in internal/plugins/flowexport/ipfix) an unsigned, address or dateTime abstract type, so no string value is ever encoded |
| [`RFC7011-6.1.6-2`](#rfc7011-6.1.6-2) IPFIX Exporting Processes MUST NOT send IPFIX Messages containing ill-formed UTF-8 string values for Information Elements of the string data type (Section 6.1.6) | no test | no test carries this requirement id; annotated {not-applicable}: no exported IE is of type string; the IANA registry gives every IE the templates declare (counterTemplateFields and flowTemplateFields in internal/plugins/flowexport/ipfix) an unsigned, address or dateTime abstract type, so no UTF-8 string value, well-formed or not, is ever sent |
| [`RFC7011-6.2-3`](#rfc7011-6.2-3) The signed versus unsigned property of the reported value MUST be preserved. (Section 6.2) | no test | no test carries this requirement id; annotated {feature-declined}: "Reduced-size encoding MAY be applied to the following integer types"; the exporter applies no reduced-size encoding. internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate declare every integer IE at its full registry width (unsigned64 counters at 8 octets, unsigned32 at 4), so no reduced value exists whose sign could be lost |
| [`RFC7011-7-1`](#rfc7011-7-1) The octets carrying the length (either the first or the first three octets) MUST NOT be included in the length of the Information Element. (Section 7) | no test | no test carries this requirement id; annotated {feature-declined}: "Information Elements of type octetArray and string may be exported using any length, subject to restrictions on length specific to each Information Element"; the exporter writes no variable-length length octets: internal/plugins/flowexport/ipfix/template.go::BuildCounterTemplate and internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate declare a fixed Field Length for every field and none of 65535, and export no octetArray or string Information Element |
| [`RFC7011-8.1-1`](#rfc7011-8.1-1) Figure T: Template Withdrawal Format The Set ID field MUST contain the value 2 for Template Set Withdrawal or the value 3 for Options Template Set Withdrawal. (Section 8.1) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 7011 Section 8 forbids Template Withdrawals over UDP (RFC7011-8-3) and the exporter is UDP only (internal/plugins/flowexport/sender.go::NewSender calls net.DialUDP); the builders always write Field Count = len(fields) > 0, so no withdrawal Set is ever written |
| [`RFC7011-8.2-4`](#rfc7011-8.2-4) An Exporting Process MUST NOT export any Data Sets described by a withdrawn Template in IPFIX Messages with an Export Time after the Export Time of the IPFIX Message containing the Template Withdrawal withdrawing that Template. (Section 8.2) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter never withdraws a Template (RFC7011-8-3 forbids withdrawals over UDP and internal/plugins/flowexport/sender.go::NewSender is UDP only), so no Data Set follows a withdrawal |
| [`RFC7011-10-5`](#rfc7011-10-5) Transport Session state MUST NOT be migrated by an Exporting Process or Collecting Process among Transport Sessions using different transport protocols between the same Exporting Process and Collecting Process pair. (Section 10) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter implements one transport protocol, UDP (internal/plugins/flowexport/sender.go::NewSender calls net.DialUDP), so no Transport Session of a second protocol exists to migrate state into; the absent SCTP is the gap under RFC7011-10-1 |
| [`RFC7011-10.2.2-1`](#rfc7011-10.2.2-1) If Data Records are discarded, the IPFIX Sequence Numbers used for export MUST reflect the loss of data. (Section 10.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter implements no SCTP transport (internal/plugins/flowexport/sender.go::NewSender is UDP only), so no SCTP export discards records; the absent SCTP is the gap under RFC7011-10-1 |
| [`RFC7011-10.3.2-1`](#rfc7011-10.3.2-1) UDP MUST NOT be used unless the application can tolerate some loss of IPFIX Messages (Section 10.3.2) | {gap}, no test | the exporter offers UDP as its only transport (internal/plugins/flowexport/sender.go::NewSender calls net.DialUDP), so an operator whose application cannot tolerate lost IPFIX Messages has no reliable transport to select; this follows from the SCTP gap under RFC7011-10-1 |
| [`RFC7011-10.4.1-1`](#rfc7011-10.4.1-1) The dropped Data Records MUST be accounted for, so that the number of lost records can later be reported as described in Section 4.3. (Section 10.4.1) | no test | no test carries this requirement id; annotated {feature-declined}: "TCP [TCP] MAY also be implemented by compliant implementations."; the exporter implements no TCP transport, so no TCP send buffer drops records: internal/plugins/flowexport/sender.go::NewSender opens a UDP socket only |
| [`RFC7011-10.4.4-1`](#rfc7011-10.4.4-1) In the default configuration, an Exporting Process MUST NOT attempt to establish a connection more frequently than once per minute (Section 10.4.4) | no test | no test carries this requirement id; annotated {feature-declined}: "TCP [TCP] MAY also be implemented by compliant implementations."; the exporter implements no TCP transport and establishes no connection: internal/plugins/flowexport/sender.go::NewSender opens a connectionless UDP socket only |
| [`RFC7011-11.1-1`](#rfc7011-11.1-1) IPFIX Exporting Processes and Collecting Processes using TCP MUST support TLS version 1.1 and SHOULD support TLS version 1.2 [RFC5246], including the mandatory ciphersuite(s) specified in each version. (Section 11.1) | no test | no test carries this requirement id; annotated {feature-declined}: "TCP [TCP] MAY also be implemented by compliant implementations."; the exporter implements no TCP transport, so the TLS-over-TCP obligation has no path: internal/plugins/flowexport/sender.go::NewSender opens a UDP socket only |
| [`RFC7011-11.1-2`](#rfc7011-11.1-2) IPFIX Exporting Processes and Collecting Processes using UDP or SCTP MUST support DTLS version 1.0 and SHOULD support DTLS version 1.2 [RFC6347], including the mandatory ciphersuite(s) specified in each version. (Section 11.1) | {gap}, no test | the IPFIX exporter has no DTLS. NewSender in internal/plugins/flowexport/sender.go opens a plain connected UDP socket with net.DialUDP, and Send writes unencrypted datagrams to it. No flowexport configuration selects DTLS. |
| [`RFC7011-11.1-3`](#rfc7011-11.1-3) When using DTLS over SCTP, the Exporting Process MUST ensure that each IPFIX Message is sent over the same SCTP Stream that would be used when sending the same IPFIX Message directly over SCTP. (Section 11.1) | {gap}, no test | the exporter has neither SCTP nor DTLS (internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket), so DTLS over SCTP is absent; both absent capabilities are mandatory and recorded under RFC7011-10-1 and RFC7011-11.1-2 |
| [`RFC7011-11.1-4`](#rfc7011-11.1-4) Exporting and Collecting Processes MUST NOT request, offer, or use any version of the Secure Socket Layer (SSL), or any version of TLS prior to 1.1, due to known security vulnerabilities in prior versions of TLS; see Appendix E of [RFC5246] for more information. (Section 11.1) | no test | no test carries this requirement id; annotated {not-applicable}: the exporter negotiates no SSL, TLS or DTLS of any version (internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket and Send writes cleartext), so it never requests, offers or uses SSL or TLS before 1.1; the missing DTLS is the gap under RFC7011-11.1-2 |
| [`RFC7011-11.3-1`](#rfc7011-11.3-1) Exporting Processes MUST verify the reference identifiers of the Collecting Processes to which they are exporting IPFIX Messages against those stored in the certificates. (Section 11.3) | {gap}, no test | the exporter has no DTLS or TLS and so verifies no certificate reference identifier; internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket to the configured address |
| [`RFC7011-11.3-2`](#rfc7011-11.3-2) Exporting Processes MUST NOT export to non-verified Collecting Processes, and Collecting Processes MUST NOT accept IPFIX Messages from non-verified Exporting Processes (Section 11.3) | {gap}, no test | the exporter exports to any configured Collecting Process without verifying it; internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket with no DTLS or TLS authentication |
| [`RFC7011-11.3-3`](#rfc7011-11.3-3) Exporting Processes and Collecting Processes MUST support the verification of certificates against an explicitly authorized list of peer certificates identified by Common Name (Section 11.3) | {gap}, no test | the exporter has no certificate verification and no authorized-certificate list; internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket with no DTLS or TLS |
| [`RFC7011-11.3-4`](#rfc7011-11.3-4) IPFIX Exporting Processes and Collecting Processes MUST use non-NULL ciphersuites for authentication, integrity, and confidentiality (Section 11.3) | {gap}, no test | the exporter uses no ciphersuite at all; internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket and Send writes cleartext IPFIX Messages |
| [`RFC7011-11-1`](#rfc7011-11-1) To prevent man-in-the-middle attacks from impostor Exporting or Collecting Processes, the acceptance of data from an unauthorized Exporting Process, or the export of data to an unauthorized Collecting Process, mutual authentication MUST be used for both TLS and DTLS. (Section 11) | {gap}, no test | the exporter has no DTLS or TLS and therefore no mutual authentication; internal/plugins/flowexport/sender.go::NewSender opens a plain UDP socket |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7011-3.1-1`](#rfc7011-3.1-1)

The value of this field is 0x000a for the current version (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Message Header whose Version is not 0x000a. TestRFC7011VersionIsIPFIX asserts the first two octets WriteMessageHeader writes are 0x000a and the Version constant is 0x000a, red on any other value. Single-polarity marker: the version is a compile-time constant with no path to another value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7011VersionIsIPFIX`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L102) | unit/verify | unproven |

### [`RFC7011-3.3.1-1`](#rfc7011-3.3.1-1)

For security reasons, the padding octet(s) MUST be composed of octets with value zero (0). (Section 3.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a padding octet that is not zero. TestRFC7011PaddingIsZero asserts the 3 padding octets after a 53-octet IPv4 flow record are 0; TestRFC7011PaddingZeroedOverGarbage pre-fills the buffer with 0xFF and asserts the same octets are 0, red if writeFlowDataSet skips the zeroing. The IPv6 flow Set shares writeFlowDataSet, and the counter Set (32-octet records) never pads.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011PaddingZeroedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L149) | unit/verify | unproven |
| positive | [`TestRFC7011PaddingIsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L137) | unit/verify | unproven |

### [`RFC7011-3.3.1-2`](#rfc7011-3.3.1-2)

The padding length MUST be shorter than any allowable record in this Set. (Section 3.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: padding as long as or longer than a record of the Set. TestRFC7011PaddingShorterThanRecord asserts the padding is > 0, < FlowRecordSize and <= 3 octets, red otherwise. Single-polarity marker: 4-octet alignment padding cannot reach the smallest record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7011PaddingShorterThanRecord`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L161) | unit/verify | unproven |

### [`RFC7011-3.4.1-1`](#rfc7011-3.4.1-1)

Each Template Record is given a unique Template ID in the range 256 to 65535. (Section 3.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity row. TestRFC7011TemplateIDsUniqueInSession reads the Template ID of all three Template Sets one Transport Session carries (counter, IPv4 flow, IPv6 flow) and requires each >= 256 and all three distinct, so setting FlowTemplateID to 256 goes red (recorded). The exporter emits no other Template.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7011TemplateIDsUniqueInSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/integral_rfc7011_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestRFC7011TemplateIDAbove255`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L174) | unit/verify | revert, verified |

### [`RFC7011-3.4.2-1`](#rfc7011-3.4.2-1)

The Scope Field Count MUST NOT be zero. (Section 3.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-3.4.2-1, so no unit is bound to it.

### [`RFC7011-3.2-1`](#rfc7011-3.2-1)

When the Enterprise bit is set to 0, the corresponding Information Element appears in [IANA-IPFIX], and the Enterprise Number MUST NOT be present. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an Enterprise Number following a field specifier whose E bit is 0. TestRFC7011NoEnterpriseNumberWhenEClear asserts each of the counter, flow4 and flow6 templates is exactly 8 + 4 x fieldCount octets (red if any specifier carries 4 more octets) and that bit 15 is clear in every specifier; TestIPFIXTemplateSet pins the counter Template Set. Single-polarity marker: no code path sets E.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPFIXTemplateSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_template_test.go#L9) | unit/verify | revert, verified |
| positive | [`TestRFC7011NoEnterpriseNumberWhenEClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L198) | unit/verify | unproven |

### [`RFC7011-3.2-2`](#rfc7011-3.2-2)

When the Enterprise bit is set to 1, the corresponding Information Element identifier identified an enterprise-specific Information Element; the Enterprise Number MUST be present. (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-3.2-2, so no unit is bound to it.

### [`RFC7011-7-2`](#rfc7011-7-2)

Where an Information Element has a variable length, the following mechanism MUST be used to carry the length information for both the IANA-assigned and enterprise-specific Information Elements. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-7-2, so no unit is bound to it.

### [`RFC7011-x-2`](#rfc7011-x-2)

The length may also be encoded into 3 octets before the Information Element, allowing the length of the Information Element to be greater than or equal to 255 octets. In this case, the first octet of the Length field MUST be 255, and the length is carried in the second and third octets, as shown in Figure S. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-x-2, so no unit is bound to it.

### [`RFC7011-8-1`](#rfc7011-8-1)

Since UDP provides no method for reliable transmission of Templates, Exporting Processes using UDP as the transport protocol MUST periodically retransmit each active Template at regular intervals. (Section 8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7011EachActiveTemplateRetransmittedEveryInterval counts the counter Template and the flow Templates separately over seven half-interval exports (0..1800 s) and requires exactly one send per elapsed 600 s interval, four in all, so a single resend, no resend, or a flow Template never refreshed goes red (recorded). The same exact count forbids a resend at a half interval. The old negative (no resend at +2 s) tests over-sending, not this MUST, and carries no record; the new unit carries both directions. The flow count is of EncodeFlowTemplate calls; that one call sends both 257 and 258 is shown by the 8.2-1 rollback unit, which reads three Template messages.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011TemplateNotRetransmittedBeforeInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestRFC7011EachActiveTemplateRetransmittedEveryInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_template_refresh_test.go#L43) | unit/verify | revert, verified |
| positive | [`TestRFC7011TemplateRetransmittedAtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L25) | unit/verify | revert, verified |

### [`RFC7011-8-2`](#rfc7011-8-2)

The Template retransmission interval MUST be configurable via, for example, the templateRefreshTimeout and optionsTemplateRefreshTimeout parameters as defined in [RFC6728]. (Section 8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7011ConfiguredRefreshDrivesRetransmit sets template-refresh 30 s and requires counter and flow resends at 30 s and not at 29 s; TestRFC7011ConfiguredRefreshOverridesDefault sets 900 s and requires no resend at 600 or 899 s, so an exporter hard-coding 600 s goes red (both recorded). The config parse into CollectorConfig.TemplateRefresh is the existing TestRFC7011TemplateRefreshConfigurable.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011ConfiguredRefreshOverridesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_template_refresh_test.go#L109) | unit/verify | revert, verified |
| negative | [`TestRFC7011TemplateRefreshRangeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestRFC7011ConfiguredRefreshDrivesRetransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_template_refresh_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC7011TemplateRefreshConfigurable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_test.go#L67) | unit/verify | revert, verified |

### [`RFC7011-8-3`](#rfc7011-8-3)

Template Withdrawals (Section 8.1) MUST NOT be sent by Exporting Processes exporting via UDP (§8.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive TestRFC7011UDPExporterSendsNoTemplateWithdrawal: the real IPFIX encoders over a real UDP socket through start, first flows and a 600 s refresh; every Template Record the collector walks has Field Count > 0, 256 and 257 are defined, none has Field Count 0. Negative (R1(b)) TestRFC7011UDPExporterReloadSendsNoTemplateWithdrawal: drives the exporter toward withdrawal (reload to another Observation Domain replacing every Template, in configure's order; then collector removal with straggling snapshot/flows into stopped exporters) and asserts no Field Count 0 record and zero datagrams after removal. Records: revert of BuildCounterTemplate (+) and buildFlowTemplate (-), both panic reverts; the tests replicate configure's start-then-stop order rather than calling the configure closure, which stays untested for this rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011UDPExporterReloadSendsNoTemplateWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_withdrawal_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestRFC7011UDPExporterSendsNoTemplateWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_withdrawal_test.go#L161) | unit/verify | revert, verified |

### [`RFC7011-10-1`](#rfc7011-10-1)

SCTP [RFC4960] using the Partially Reliable SCTP (PR-SCTP) extension as specified in [RFC3758] MUST be implemented by all compliant implementations. (§10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10-1, so no unit is bound to it.

### [`RFC7011-10-2`](#rfc7011-10-2)

Template Sets and Options Template Sets MUST be sent reliably, using SCTP ordered delivery. (§8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10-2, so no unit is bound to it.

### [`RFC7011-6.2-1`](#rfc7011-6.2-1)

Reduced-size encoding MUST NOT be applied to any other data type defined in [RFC7012] that implies a fixed length, as these types either have internal structure (such as ipv4Address or dateTimeMicroseconds) or restricted ranges that are not suitable for reduced-size encoding (such as dateTimeMilliseconds). (Section 6.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a reduced-size length on an address or dateTime IE. TestRFC7011NoReducedSizeForAddressOrTimestamp walks the counter, flow4 and flow6 templates and asserts IPv4 address IEs at 4, IPv6 at 16, dateTimeSeconds at 4 and dateTimeMilliseconds at 8 octets, red on any reduced length; every other exported IE is integral, for which reduced size is permitted. Single-polarity marker: no path applies reduced-size encoding.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7011NoReducedSizeForAddressOrTimestamp`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_test.go#L227) | unit/verify | unproven |

### [`RFC7011-3.3.1-3`](#rfc7011-3.3.1-3)

The record types MUST NOT be mixed within a Set (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011RecordTypesNotMixedInSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L120) | unit/verify | revert, verified |
| positive | [`TestRFC7011OneRecordTypePerSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L88) | unit/verify | revert, verified |

### [`RFC7011-3.4.2.1-1`](#rfc7011-3.4.2.1-1)

If a different order of Scope Fields would result in a Record having a different semantic meaning, then the order of Scope Fields MUST be preserved by the Exporting Process (Section 3.4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-3.4.2.1-1, so no unit is bound to it.

### [`RFC7011-4.1-1`](#rfc7011-4.1-1)

This Information Element MUST be defined as a Scope Field and MUST be present, unless the Observation Domain ID of the enclosing Message is non-zero. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-4.1-1, so no unit is bound to it.

### [`RFC7011-4.1-2`](#rfc7011-4.1-2)

If present, this Information Element MUST be defined as a Scope Field. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-4.1-2, so no unit is bound to it.

### [`RFC7011-4.1-3`](#rfc7011-4.1-3)

Note that if several Metering Processes are available on the Exporter Observation Domain, the Information Element meteringProcessId MUST be specified as an additional Scope Field. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-4.1-3, so no unit is bound to it.

### [`RFC7011-6.1.1-1`](#rfc7011-6.1.1-1)

Integral data types -- unsigned8, unsigned16, unsigned32, unsigned64, signed8, signed16, signed32, and signed64 -- MUST be encoded using the default canonical format in network byte order. (Section 6.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both Data Record paths now proven. Per-flow path: the three existing units (recorded). Counter path writeCounterRecord: TestRFC7011CounterRecordIntegralsNetworkByteOrder compares the whole record to the big-endian concatenation of ingressInterface, octetTotalCount, packetTotalCount, egressInterface and the two dateTimeSeconds, each built from distinct octets; TestRFC7011CounterRecordIntegralsNeverLittleEndian requires no byte-reversed field (both recorded, writeCounterRecord).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011CounterRecordIntegralsNeverLittleEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/integral_rfc7011_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestRFC7011IntegralNeverLittleEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L187) | unit/verify | revert, verified |
| positive | [`TestRFC7011CounterRecordIntegralsNetworkByteOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/integral_rfc7011_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC7011IntegralNetworkByteOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L163) | unit/verify | revert, verified |
| positive | [`TestIPFIXFlowData`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_flow_data_test.go#L10) | unit/verify | revert, verified |

### [`RFC7011-6.1.2-1`](#rfc7011-6.1.2-1)

Address types -- macAddress, ipv4Address, and ipv6Address -- MUST be encoded the same way as the integral data types, as six, four, and sixteen octets in network byte order, respectively. (Section 6.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an ipv4Address or ipv6Address not written as 4 or 16 octets in network byte order. TestRFC7011AddressOctetsNetworkByteOrder asserts the IPv4 record opens with 0a010203 0a040506 and the IPv6 record with the 16-octet source then destination in network order, and that each Set payload is the record size; TestRFC7011IPv4NeverWidenedToSixteenOctets asserts an IPv4 flow is neither encoded at the IPv6 width nor carries an IPv4-mapped 16-octet form. Ze exports no macAddress IE, so that clause has no producer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011IPv4NeverWidenedToSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L231) | unit/verify | revert, verified |
| positive | [`TestRFC7011AddressOctetsNetworkByteOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L202) | unit/verify | revert, verified |

### [`RFC7011-6.1.3-1`](#rfc7011-6.1.3-1)

The float32 data type MUST be encoded as an IEEE binary32 floating point type as specified in [IEEE.754.2008], in network byte order as specified in Section 3.6 of [RFC1014]. (Section 6.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.1.3-1, so no unit is bound to it.

### [`RFC7011-6.1.4-1`](#rfc7011-6.1.4-1)

The float64 data type MUST be encoded as an IEEE binary64 floating point type as specified in [IEEE.754.2008], in network byte order as specified in Section 3.7 of [RFC1014]. (Section 6.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.1.4-1, so no unit is bound to it.

### [`RFC7011-6.1.6-1`](#rfc7011-6.1.6-1)

The string data type MUST be encoded in UTF-8 [RFC3629] format. (Section 6.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.1.6-1, so no unit is bound to it.

### [`RFC7011-6.1.6-2`](#rfc7011-6.1.6-2)

IPFIX Exporting Processes MUST NOT send IPFIX Messages containing ill-formed UTF-8 string values for Information Elements of the string data type (Section 6.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.1.6-2, so no unit is bound to it.

### [`RFC7011-6.2-3`](#rfc7011-6.2-3)

The signed versus unsigned property of the reported value MUST be preserved. (Section 6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-6.2-3, so no unit is bound to it.

### [`RFC7011-7-1`](#rfc7011-7-1)

The octets carrying the length (either the first or the first three octets) MUST NOT be included in the length of the Information Element. (Section 7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-7-1, so no unit is bound to it.

### [`RFC7011-8.1-1`](#rfc7011-8.1-1)

Figure T: Template Withdrawal Format The Set ID field MUST contain the value 2 for Template Set Withdrawal or the value 3 for Options Template Set Withdrawal. (Section 8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-8.1-1, so no unit is bound to it.

### [`RFC7011-8.2-1`](#rfc7011-8.2-1)

Since there is no guarantee of the ordering of exported IPFIX Messages across SCTP Streams or over UDP, an Exporting Process MUST sequence all Template management actions (i.e., Template Records defining new Templates and Template Withdrawals withdrawing them) using the Export Time field in the IPFIX Message Header. (Section 8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Counter Template: TestRFC7011CounterTemplateExportTimeIsSendTime pins its Export Time to the send second. Ordering: TestRFC7011TemplateRefreshAfterClockRollbackKeepsOrder steps the clock back with synctest and requires the refreshed counter 256 and flow 257, 258 Templates to carry an Export Time not before the earlier action, so removing either max(now, templateExportTime) clamp goes red (both recorded). The exporter sends no Template Withdrawals.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011TemplateExportTimeNeverRegresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L265) | unit/verify | revert, verified |
| negative | [`TestRFC7011TemplateRefreshAfterClockRollbackKeepsOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_template_time_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC7011TemplateExportTimeIsSendTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L246) | unit/verify | revert, verified |
| positive | [`TestRFC7011CounterTemplateExportTimeIsSendTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_template_time_test.go#L19) | unit/verify | revert, verified |

### [`RFC7011-8.2-2`](#rfc7011-8.2-2)

An Exporting Process MUST NOT export a Data Set described by a new Template in an IPFIX Message with an Export Time before the Export Time of the IPFIX Message containing that Template (Section 8.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011StaleSnapshotClampedToTemplateTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L308) | unit/verify | revert, verified |
| positive | [`TestRFC7011DataExportTimeNotBeforeTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L286) | unit/verify | revert, verified |

### [`RFC7011-8.2-3`](#rfc7011-8.2-3)

If a new Template and a Data Set described by it appear in the same IPFIX Message, the Template Set containing the Template MUST appear before the Data Set in the Message (Section 8.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011DataSetNeverPrecedesTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L500) | unit/verify | revert, verified |
| positive | [`TestRFC7011TemplateSetPrecedesDataSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7011_export_test.go#L483) | unit/verify | revert, verified |

### [`RFC7011-8.2-4`](#rfc7011-8.2-4)

An Exporting Process MUST NOT export any Data Sets described by a withdrawn Template in IPFIX Messages with an Export Time after the Export Time of the IPFIX Message containing the Template Withdrawal withdrawing that Template. (Section 8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-8.2-4, so no unit is bound to it.

### [`RFC7011-10-5`](#rfc7011-10-5)

Transport Session state MUST NOT be migrated by an Exporting Process or Collecting Process among Transport Sessions using different transport protocols between the same Exporting Process and Collecting Process pair. (Section 10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10-5, so no unit is bound to it.

### [`RFC7011-10.1-1`](#rfc7011-10.1-1)

It MUST be possible to configure both the Exporting and Collecting Processes to use different ports than the default (Section 10.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011CollectorPortOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_config_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestRFC7011CollectorPortConfigurable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_config_test.go#L20) | unit/verify | revert, verified |

### [`RFC7011-10.2.2-1`](#rfc7011-10.2.2-1)

If Data Records are discarded, the IPFIX Sequence Numbers used for export MUST reflect the loss of data. (Section 10.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.2.2-1, so no unit is bound to it.

### [`RFC7011-10.3.2-1`](#rfc7011-10.3.2-1)

UDP MUST NOT be used unless the application can tolerate some loss of IPFIX Messages (Section 10.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.3.2-1, so no unit is bound to it.

### [`RFC7011-10.3.2-2`](#rfc7011-10.3.2-2)

Exporting Processes exporting IPFIX Messages via UDP MUST include a valid UDP checksum [UDP] in UDP datagrams including IPFIX Messages. (Section 10.3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an IPFIX UDP datagram sent without a valid UDP checksum. TestRFC7011UDPChecksumEnabledOnSenderSocket (Linux) reads the socket NewSender dials and asserts SO_NO_CHECK clear for IPv4 and UDP_NO_CHECK6_TX clear for IPv6, red if either is set, so the kernel writes the checksum. Single-polarity marker: NewSender never sets either option.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7011UDPChecksumEnabledOnSenderSocket`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_sender_linux_test.go#L38) | unit/verify | unproven |

### [`RFC7011-10.3.3-1`](#rfc7011-10.3.3-1)

The maximum size of exported messages MUST be configured such that the total packet size does not exceed the PMTU (Section 10.3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7011MaxDatagramSizeOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_datagram_test.go#L56) | unit/verify | unproven |
| positive | [`TestRFC7011MaxDatagramSizeConfigured`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc7011_datagram_test.go#L23) | unit/verify | unproven |

### [`RFC7011-10.4.1-1`](#rfc7011-10.4.1-1)

The dropped Data Records MUST be accounted for, so that the number of lost records can later be reported as described in Section 4.3. (Section 10.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.4.1-1, so no unit is bound to it.

### [`RFC7011-10.4.4-1`](#rfc7011-10.4.4-1)

In the default configuration, an Exporting Process MUST NOT attempt to establish a connection more frequently than once per minute (Section 10.4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-10.4.4-1, so no unit is bound to it.

### [`RFC7011-11.1-1`](#rfc7011-11.1-1)

IPFIX Exporting Processes and Collecting Processes using TCP MUST support TLS version 1.1 and SHOULD support TLS version 1.2 [RFC5246], including the mandatory ciphersuite(s) specified in each version. (Section 11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.1-1, so no unit is bound to it.

### [`RFC7011-11.1-2`](#rfc7011-11.1-2)

IPFIX Exporting Processes and Collecting Processes using UDP or SCTP MUST support DTLS version 1.0 and SHOULD support DTLS version 1.2 [RFC6347], including the mandatory ciphersuite(s) specified in each version. (Section 11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.1-2, so no unit is bound to it.

### [`RFC7011-11.1-3`](#rfc7011-11.1-3)

When using DTLS over SCTP, the Exporting Process MUST ensure that each IPFIX Message is sent over the same SCTP Stream that would be used when sending the same IPFIX Message directly over SCTP. (Section 11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.1-3, so no unit is bound to it.

### [`RFC7011-11.1-4`](#rfc7011-11.1-4)

Exporting and Collecting Processes MUST NOT request, offer, or use any version of the Secure Socket Layer (SSL), or any version of TLS prior to 1.1, due to known security vulnerabilities in prior versions of TLS; see Appendix E of [RFC5246] for more information. (Section 11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7011-11.1-4, so no unit is bound to it.

### [`RFC7011-11.3-1`](#rfc7011-11.3-1)

Exporting Processes MUST verify the reference identifiers of the Collecting Processes to which they are exporting IPFIX Messages against those stored in the certificates. (Section 11.3)

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

To prevent man-in-the-middle attacks from impostor Exporting or Collecting Processes, the acceptance of data from an unauthorized Exporting Process, or the export of data to an unauthorized Collecting Process, mutual authentication MUST be used for both TLS and DTLS. (Section 11)

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
