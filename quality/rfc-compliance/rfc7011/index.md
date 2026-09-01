# RFC 7011 - Specification of the IP Flow Information Export (IPFIX) Protocol for the Exchange of Flow Information

Experimental. Every requirement this repository extracted from RFC 7011, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 40.0% | 4 of 10 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 50.0% | 5 of 10 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 19 | of 30 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 9 | of 19 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 10.0% | 1 of 10 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 10 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 30 |
| Gated MUST-level | 19 |
| Obligations that bind Ze | 10 |
| Not applicable, so out of scope | 9 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7011.md` |
| Requirement shard | `rfc/requirements/rfc7011.md` |
| RFC text | `rfc/full/rfc7011.txt` |

## Enrolment

Enrolled: IP Flow Information Export / IPFIX (RFC 7011): exporter role. 4 MET (message has >=1 Set, zero-valued Set padding, periodic UDP Template refresh, configurable refresh interval) + 5 single-polarity positive (version 0x000a, padding shorter than a record, Template ID >= 256, no Enterprise Number when E=0, no reduced-size address/timestamp encoding) + 1 gap (no SCTP transport, UDP-only) + 9 not-applicable (no Options Templates, no E=1 enterprise IEs, no variable-length fields, no Template Withdrawals, no SCTP/PR-SCTP, no TCP)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- IPFIX export templates and records over UDP (exporter): version 0x000a, Template IDs >= 256, IANA (E=0) fixed-length field specifiers, zeroed Set padding, and periodic UDP Template refresh at a configurable interval
- tests bound per requirement in [`rfc/requirements/rfc7011.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7011.md).


**What the ledger says remains**

One MUST gap in [`rfc/short/rfc7011.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7011.md) [[`RFC7011-10-1`](#rfc7011-10-1)]: the exporter transmits over UDP only and implements no SCTP transport, so the mandatory SCTP support is absent. ze is an exporter only; IPFIX Collecting Process requirements are not applicable.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 15 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **19** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7011-3-1`](#rfc7011-3-1), [`RFC7011-3.3.1-1`](#rfc7011-3.3.1-1), [`RFC7011-8-1`](#rfc7011-8-1), [`RFC7011-8-2`](#rfc7011-8-2)

**Annotated instead of tested (15):** [`RFC7011-3.1-1`](#rfc7011-3.1-1), [`RFC7011-3.3.1-2`](#rfc7011-3.3.1-2), [`RFC7011-3.4.1-1`](#rfc7011-3.4.1-1), [`RFC7011-3.4.2-1`](#rfc7011-3.4.2-1), [`RFC7011-3.4.2-2`](#rfc7011-3.4.2-2), [`RFC7011-3.2-1`](#rfc7011-3.2-1), [`RFC7011-3.2-2`](#rfc7011-3.2-2), [`RFC7011-x-1`](#rfc7011-x-1), [`RFC7011-x-2`](#rfc7011-x-2), [`RFC7011-8-3`](#rfc7011-8-3), [`RFC7011-10-1`](#rfc7011-10-1), [`RFC7011-10-2`](#rfc7011-10-2), [`RFC7011-x-3`](#rfc7011-x-3), [`RFC7011-x-4`](#rfc7011-x-4), [`RFC7011-6.2-1`](#rfc7011-6.2-1)

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
| `RFC7011-10-3` | Exporting Processes SHOULD support TCP and UDP in addition to SCTP (Section 10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-10-4` | Over UDP, IPFIX Messages SHOULD fit within the path MTU to avoid IP fragmentation (Section 10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-x-5` | Over UDP, the Exporting Process SHOULD implement rate limiting or congestion-avoidance mechanisms (UDP Transport) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-x-6` | The Exporting Process over TCP SHOULD use long-lived connections (TCP Transport) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7011-11-1` | Implementations SHOULD support mutual authentication for TLS/DTLS (Section 11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
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

## Extraction sign-off

No extraction sign-off exists for RFC 7011, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7011, so its obligations are stated where they were written.
