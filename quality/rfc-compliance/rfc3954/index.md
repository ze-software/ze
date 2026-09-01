# RFC 3954 - Cisco Systems NetFlow Services Export Version 9

Experimental. Every requirement this repository extracted from RFC 3954, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 40.0% | 2 of 5 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 40.0% | 2 of 5 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 21 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.0% | 1 of 5 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 5 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 21 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 5 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3954.md` |
| Requirement shard | `rfc/requirements/rfc3954.md` |
| RFC text | `rfc/full/rfc3954.txt` |

## Enrolment

Enrolled: NetFlow Services Export Version 9 (ze as a v9 exporter): nine MUST-level requirements. Four are met: x-1 (never send a Data FlowSet before its Template) and x-8 (the sequence number is cumulative per observation domain) carry positive+negative tags; x-3 (network byte order) and x-9 (a Template ID is constant for the process lifetime) are {single-polarity: positive}. x-2 (refresh templates on both a time interval and a packet-count interval) is {gap}: ze refreshes on a configurable time interval only, with no packet-count-based interval. x-4, x-5, x-6, x-7 are {not-applicable}: they are NetFlow v9 collector requirements and ze is an exporter only. Disclosed in the docs/features/rfc-status.md RFC 3954 row.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- Flow export templates and records over UDP (exporter): no Data FlowSet before its Template, cumulative per-observation-domain sequence numbers, network byte order, constant Template IDs
- tests bound per requirement in [`rfc/requirements/rfc3954.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc3954.md).


**What the ledger says remains:**

Template refresh is time-interval-based only, with no packet-count-based refresh interval (RFC3954-x-2 gap). ze is an exporter only; the v9 collector requirements are not applicable.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC3954-x-1`](#rfc3954-x-1), [`RFC3954-x-8`](#rfc3954-x-8)

**Annotated instead of tested (7):** [`RFC3954-x-2`](#rfc3954-x-2), [`RFC3954-x-3`](#rfc3954-x-3), [`RFC3954-x-4`](#rfc3954-x-4), [`RFC3954-x-5`](#rfc3954-x-5), [`RFC3954-x-6`](#rfc3954-x-6), [`RFC3954-x-7`](#rfc3954-x-7), [`RFC3954-x-9`](#rfc3954-x-9)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3954-x-1` | Exporter MUST NOT send a Data FlowSet without having sent the corresponding Template FlowSet in a previous or the same export packet (Template Lifecycle) | MUST | x | **positive:** `unit/verify` [`TestWriteExportPacketWithTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/encoder_test.go#L69). **negative:** `unit/verify` [`TestExporterSendsTemplateBeforeData`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L86) |
| `RFC3954-x-2` | Exporter MUST periodically refresh templates using both time-based and packet-count-based intervals, both configurable (Template Lifecycle) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze refreshes NetFlow v9 templates on a time interval only (internal/plugins/flowexport/exporter.go:192-199, config template-refresh seconds); it has no packet-count-based template-refresh interval, so the conjoined time-and-packet-count refresh requirement is only partly met |
| `RFC3954-x-3` | All binary integer values MUST be coded in network byte order (big-endian) (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestNetflow9DataFlowSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/data_test.go#L10). **positive:** `unit/verify` [`TestNetflow9Header`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/encoder_test.go#L10). **negative:** no negative test. **{single-polarity}:** ze only ENCODES NetFlow v9 (exporter-only, internal/plugins/flowexport/netflow9); every multi-octet field is written big-endian via binary.BigEndian and there is no decode or wrong-endianness code path to reject, so only the positive can be tested |
| `RFC3954-x-4` | Collector MUST use the FlowSet ID to find the correct template for decoding (Validation) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a NetFlow v9 collector requirement (map a Data FlowSet ID to its template); ze is a v9 exporter only (internal/plugins/flowexport/netflow9) with no v9 decode/collect code path |
| `RFC3954-x-5` | Collector MUST use the Length field to determine the position of the next FlowSet record (Encoding Rules) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** collector requirement (use FlowSet Length to find the next FlowSet); ze does not collect v9 |
| `RFC3954-x-6` | Collector MUST accept padding in Data FlowSets and Options Template FlowSets (Validation) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** collector requirement (accept padding); ze does not collect v9 (its exporter does emit 4-octet padding at internal/plugins/flowexport/netflow9/data.go:38-44) |
| `RFC3954-x-7` | Collector MUST override an existing template when a new definition arrives for the same Template ID (Template Lifecycle) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** collector requirement (override a template on redefinition); ze does not collect v9 |
| `RFC3954-x-8` | Sequence Number MUST be a cumulative counter per observation domain (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestNetflow9FlowSeqNumPerPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/flow_adapter_test.go#L53). **negative:** `unit/verify` [`TestNetflow9SeqNumNotAdvancedOnSendError`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/adapter_test.go#L58) |
| `RFC3954-x-9` | Template ID MUST remain constant for the life of the NetFlow process on the exporter (Template Lifecycle) | MUST | x | **positive:** `unit/verify` [`TestNetflow9FlowTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/flow_template_test.go#L8). **positive:** `unit/verify` [`TestNetflow9Template`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/template_test.go#L8). **negative:** no negative test. **{single-polarity}:** ze's NetFlow v9 template IDs are compile-time constants (CounterTemplateID=256 in internal/plugins/flowexport/netflow9/template.go, FlowTemplateID=257 and FlowTemplateID6=258 in flow_template.go) that are never reassigned for the life of the process, so there is no ID-change code path to test negatively |
| `RFC3954-x-10` | Exporter SHOULD insert padding to 4-octet alignment using zeros (Encoding Rules) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-11` | Exporter SHOULD send templates at an accelerated rate after configuration changes or clock changes (Template Lifecycle) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-12` | Collector SHOULD use (exporter IP, Source ID) to separate different export streams (Wire Format) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-13` | Collector SHOULD buffer data records when the matching template has not yet been received (Validation) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-14` | Collector SHOULD use Sequence Number to detect missing packets (Wire Format) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-15` | Exporter SHOULD NOT reuse a Template ID after configuration changes until the process restarts (Template Lifecycle) | SHOULD NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-16` | Export path SHOULD be a dedicated link or provisioned to handle burst rate without drops (Transport Considerations) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-17` | Exporter MAY send template-only packets (no data) to pre-populate the collector (Template Lifecycle) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-18` | Exporter MAY export flows prematurely due to internal constraints (memory pressure, counter wrap) (Exporter Implementation) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-19` | Exporter MAY include multiple template records in a single Template FlowSet (Template FlowSet) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-20` | Exporter MAY send templates and data in the same packet (Template Lifecycle) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-21` | Exporters MAY use reserved field type IDs for vendor-specific fields (Reserved Fields) | MAY | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3954-x-2`](#rfc3954-x-2) Exporter MUST periodically refresh templates using both time-based and packet-count-based intervals, both configurable (Template Lifecycle) | {gap}, no test | ze refreshes NetFlow v9 templates on a time interval only (internal/plugins/flowexport/exporter.go:192-199, config template-refresh seconds); it has no packet-count-based template-refresh interval, so the conjoined time-and-packet-count refresh requirement is only partly met |
| [`RFC3954-x-4`](#rfc3954-x-4) Collector MUST use the FlowSet ID to find the correct template for decoding (Validation) | no test | no test carries this requirement id; annotated {not-applicable}: this is a NetFlow v9 collector requirement (map a Data FlowSet ID to its template); ze is a v9 exporter only (internal/plugins/flowexport/netflow9) with no v9 decode/collect code path |
| [`RFC3954-x-5`](#rfc3954-x-5) Collector MUST use the Length field to determine the position of the next FlowSet record (Encoding Rules) | no test | no test carries this requirement id; annotated {not-applicable}: collector requirement (use FlowSet Length to find the next FlowSet); ze does not collect v9 |
| [`RFC3954-x-6`](#rfc3954-x-6) Collector MUST accept padding in Data FlowSets and Options Template FlowSets (Validation) | no test | no test carries this requirement id; annotated {not-applicable}: collector requirement (accept padding); ze does not collect v9 (its exporter does emit 4-octet padding at internal/plugins/flowexport/netflow9/data.go:38-44) |
| [`RFC3954-x-7`](#rfc3954-x-7) Collector MUST override an existing template when a new definition arrives for the same Template ID (Template Lifecycle) | no test | no test carries this requirement id; annotated {not-applicable}: collector requirement (override a template on redefinition); ze does not collect v9 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3954-x-1`](#rfc3954-x-1)

Exporter MUST NOT send a Data FlowSet without having sent the corresponding Template FlowSet in a previous or the same export packet (Template Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExporterSendsTemplateBeforeData`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L86) | unit/verify | unproven |
| positive | [`TestWriteExportPacketWithTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/encoder_test.go#L69) | unit/verify | unproven |

### [`RFC3954-x-2`](#rfc3954-x-2)

Exporter MUST periodically refresh templates using both time-based and packet-count-based intervals, both configurable (Template Lifecycle)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-2, so no unit is bound to it.

### [`RFC3954-x-3`](#rfc3954-x-3)

All binary integer values MUST be coded in network byte order (big-endian) (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNetflow9DataFlowSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/data_test.go#L10) | unit/verify | unproven |
| positive | [`TestNetflow9Header`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/encoder_test.go#L10) | unit/verify | unproven |

### [`RFC3954-x-4`](#rfc3954-x-4)

Collector MUST use the FlowSet ID to find the correct template for decoding (Validation)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-4, so no unit is bound to it.

### [`RFC3954-x-5`](#rfc3954-x-5)

Collector MUST use the Length field to determine the position of the next FlowSet record (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-5, so no unit is bound to it.

### [`RFC3954-x-6`](#rfc3954-x-6)

Collector MUST accept padding in Data FlowSets and Options Template FlowSets (Validation)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-6, so no unit is bound to it.

### [`RFC3954-x-7`](#rfc3954-x-7)

Collector MUST override an existing template when a new definition arrives for the same Template ID (Template Lifecycle)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-7, so no unit is bound to it.

### [`RFC3954-x-8`](#rfc3954-x-8)

Sequence Number MUST be a cumulative counter per observation domain (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNetflow9SeqNumNotAdvancedOnSendError`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/adapter_test.go#L58) | unit/verify | unproven |
| positive | [`TestNetflow9FlowSeqNumPerPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/flow_adapter_test.go#L53) | unit/verify | unproven |

### [`RFC3954-x-9`](#rfc3954-x-9)

Template ID MUST remain constant for the life of the NetFlow process on the exporter (Template Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNetflow9FlowTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/flow_template_test.go#L8) | unit/verify | unproven |
| positive | [`TestNetflow9Template`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/template_test.go#L8) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 3954, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 3954, so its obligations are stated where they were written.
