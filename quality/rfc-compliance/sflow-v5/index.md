# SFLOW-V5 - sFlow: A Method for Monitoring Traffic in Switched and Routed Networks

Experimental. Every requirement this repository extracted from SFLOW-V5, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6.7% | 1 of 15 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 73.3% | 11 of 15 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 15 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 16 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.0% | 3 of 15 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 15 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 24 |
| Gated MUST-level | 16 |
| Obligations that bind Ze | 15 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 3 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/sflow-v5.md` |
| Requirement shard | `rfc/requirements/sflow-v5.md` |
| RFC text | `rfc/full/sflow-v5.txt` |

## Enrolment

Enrolled: sFlow Version 5 export: exporter/agent role. 1 MET (counter poll at configured interval) + 11 single-polarity positive (version=5, agent addr, sub-agent seq space, MTU bound, no >1s hold, XDR big-endian, count-prefixed arrays, actual rate, sample_pool, cumulative counters) + 3 gap (split datagram seq, no expanded types, no unavailable sentinel) + 1 not-applicable (collector skip)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- Flow export protocol alongside NetFlow v9 and IPFIX. Three MUST gaps in [`rfc/short/sflow-v5.md`](https://github.com/ze-software/ze/blob/main/rfc/short/sflow-v5.md): datagram-level sequence numbers split across two independent counters per sub-agent (SFLOW-V5-x-9)
- no expanded sample types, so ifIndex > 2^24-1 is truncated by the 24-bit source_id mask (SFLOW-V5-x-12)
- unavailable if_counters fields are exported as 0 instead of the max-value unavailable sentinel (SFLOW-V5-x-16).


**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 15 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`SFLOW-V5-x-14`](#sflow-v5-x-14)

**Annotated instead of tested (15):** [`SFLOW-V5-x-1`](#sflow-v5-x-1), [`SFLOW-V5-x-2`](#sflow-v5-x-2), [`SFLOW-V5-x-3`](#sflow-v5-x-3), [`SFLOW-V5-x-4`](#sflow-v5-x-4), [`SFLOW-V5-x-5`](#sflow-v5-x-5), [`SFLOW-V5-x-6`](#sflow-v5-x-6), [`SFLOW-V5-x-7`](#sflow-v5-x-7), [`SFLOW-V5-x-8`](#sflow-v5-x-8), [`SFLOW-V5-x-9`](#sflow-v5-x-9), [`SFLOW-V5-x-10`](#sflow-v5-x-10), [`SFLOW-V5-x-11`](#sflow-v5-x-11), [`SFLOW-V5-x-12`](#sflow-v5-x-12), [`SFLOW-V5-x-13`](#sflow-v5-x-13), [`SFLOW-V5-x-15`](#sflow-v5-x-15), [`SFLOW-V5-x-16`](#sflow-v5-x-16)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `SFLOW-V5-x-1` | Datagram version field MUST be set to 5 (Datagram Format) | MUST | x | **positive:** `unit/verify` [`TestSFlowDatagramHeaderIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L11). **negative:** no negative test. **{single-polarity}:** WriteDatagramHeader unconditionally writes the compile-time constant Version=5 into every datagram, so there is no other-version code path to reject (internal/plugins/flowexport/sflow/encoder.go:40, :14) |
| `SFLOW-V5-x-2` | Agent address MUST be a stable IP (e.g., loopback) that uniquely identifies the device across reboots (Agent Architecture) | MUST | x | **positive:** `unit/verify` [`TestSFlowDatagramHeaderIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L12). **negative:** no negative test. **{single-polarity}:** the operator-configured agent address is written verbatim into every datagram header and validated as a well-formed IP; stability/uniqueness is a config-value property with no exporter reject path (internal/plugins/flowexport/sflow/encoder.go:43-57, internal/plugins/flowexport/config.go:379-383) |
| `SFLOW-V5-x-3` | Agent address + sub_agent_id MUST uniquely identify a sampling entity (Agent Architecture) | MUST | x | **positive:** `unit/verify` [`TestSFlowDatagramHeaderIPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L53). **negative:** no negative test. **{single-polarity}:** both agent_address and sub_agent_id are emitted in every datagram header by construction, and tuple uniqueness is an operator-config obligation (internal/plugins/flowexport/sflow/encoder.go:59, :43-57) |
| `SFLOW-V5-x-4` | Each sub-agent MUST maintain its own sequence number space (Agent Architecture) | MUST | x | **positive:** `unit/verify` [`TestSFlowMultiInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L93). **negative:** no negative test. **{single-polarity}:** each encoder instance is bound to one collector/sub_agent_id and owns private datagramSeq and per-source seqNums fields, so distinct sub-agents never share sequence state (internal/plugins/flowexport/sflow/adapter.go:18-19, flow_adapter.go:32-33) |
| `SFLOW-V5-x-5` | Datagram size MUST NOT exceed path MTU (Transport) | MUST | x | **positive:** `unit/verify` [`TestSFlowMultiInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L94). **negative:** no negative test. **{single-polarity}:** every datagram is bounded to MaxDatagramSize=1400 by construction (counter batching flushes before overflow, the flow encoder truncates the captured header to fit), so no code path emits an oversized datagram (internal/plugins/flowexport/sender.go:14, internal/plugins/flowexport/sflow/encoder.go:120, flow_adapter.go:67-71) |
| `SFLOW-V5-x-6` | Samples MUST NOT be held more than 1 second before sending (Transport) | MUST | x | **positive:** `unit/verify` [`TestExportFlowSampleDispatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L148). **negative:** no negative test. **{single-polarity}:** counter and flow samples are encoded and sent synchronously with no buffering queue, so a sample is never held beyond a sub-millisecond encode and there is no holding timer to test negatively (internal/plugins/flowexport/exporter.go:204, internal/plugins/flowexport/sflow/adapter.go:47-51) |
| `SFLOW-V5-x-7` | All structures MUST use XDR encoding: 4-byte alignment, big-endian (Datagram Format) | MUST | x | **positive:** `unit/verify` [`TestSFlowIfCounters`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counter_test.go#L35). **negative:** no negative test. **{single-polarity}:** every field is written via binary.BigEndian with 4-byte-aligned opaque padding, exporter-only with no decode path to reject a wrong endianness (internal/plugins/flowexport/sflow/counter.go:36, flow.go:123-128) |
| `SFLOW-V5-x-8` | Variable-length arrays and opaque data MUST be prefixed by a 4-byte count and padded to 4-byte boundary (XDR Encoding) | MUST | x | **positive:** `unit/verify` [`TestSFlowSampledHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_test.go#L70). **negative:** no negative test. **{single-polarity}:** the sampled_header opaque and the extended_gateway arrays are all written with a 4-byte count prefix and zero-padded to a 4-byte boundary by construction (internal/plugins/flowexport/sflow/flow.go:116-128, :187-208) |
| `SFLOW-V5-x-9` | Sequence numbers MUST be per-agent (datagram-level) and per-source (sample-level), unsigned 32-bit, wrapping (Transport) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** per-source sample-level sequences are correct, but for one sub-agent the counter-datagram and flow-datagram streams keep two independent datagramSeq counters, so a collector sees two overlapping datagram-level sequence spaces instead of one (internal/plugins/flowexport/sflow/adapter.go:18, flow_adapter.go:32) |
| `SFLOW-V5-x-10` | flow_sample MUST include the actual sampling_rate used by the agent (Flow Sample) | MUST | x | **positive:** `unit/verify` [`TestSFlowFlowSample`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_test.go#L9). **negative:** no negative test. **{single-polarity}:** EncodeFlowSample writes the kernel-reported actual rate into every flow_sample, emitted unconditionally with no reject path (internal/plugins/flowexport/sflow/flow_adapter.go:78-79, flow.go:52) |
| `SFLOW-V5-x-11` | sample_pool MUST track total packets seen by the data source (Flow Sample) | MUST | x | **positive:** `unit/verify` [`TestSFlowFlowSamplePoolTracksTotal`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_adapter_test.go#L52). **negative:** no negative test. **{single-polarity}:** EncodeFlowSample computes sample_pool as the saturated product of cumulative samples and rate and writes it into every flow_sample, exporter-only with no negative form (internal/plugins/flowexport/sflow/flow_adapter.go:73-79, flow.go:56) |
| `SFLOW-V5-x-12` | Expanded sample types MUST be used when ifIndex exceeds 2^24-1 or 2^30-1 (Expanded Flow/Counter Sample) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no flow_sample_expanded (format 3) or counters_sample_expanded (format 4); a large ifIndex is silently truncated by a 24-bit mask on source_id rather than switching to the expanded encoding (internal/plugins/flowexport/sflow/counter.go:49, flow.go:48) |
| `SFLOW-V5-x-13` | Unknown record formats MUST be skipped using the opaque length prefix (Record Wrappers) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** skipping unknown formats on receive is a collector behavior; ze is an sFlow exporter only with no sFlow decode path, though it does emit the length prefixes that let a collector skip (internal/plugins/flowexport/sflow/counter.go:60-61, flow.go:131-132) |
| `SFLOW-V5-x-14` | Counter samples MUST be produced at the configured polling interval for each data source (Counter Polling) | MUST | x | **positive:** `unit/verify` [`TestSFlowCounterPollAtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L276). **negative:** `unit/verify` [`TestSFlowCounterPollBeforeInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L295) |
| `SFLOW-V5-x-15` | All counters MUST be cumulative since boot (Counter Polling) | MUST | x | **positive:** `unit/verify` [`TestInterfaceCountersFromCumulative`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/register_test.go#L128). **negative:** no negative test. **{single-polarity}:** interfaceCountersFrom copies the raw cumulative kernel counters straight through with no differencing, so exported if_counters are cumulative by construction (internal/plugins/flowexport/register.go:343-358, snapshot.go:10-13) |
| `SFLOW-V5-x-16` | Unavailable counter fields MUST be set to max value for the type (0xFFFFFFFF for u32, 0xFFFFFFFFFFFFFFFF for u64) (Implementation Guidance) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** interfaceCountersFrom leaves fields the kernel does not expose (ifInUnknownProtos, ifInBroadcastPkts, ifOutMulticastPkts, ifOutBroadcastPkts) at zero rather than the required max-value unavailable sentinel, so a collector cannot distinguish true-zero from unavailable (internal/plugins/flowexport/register.go:343-361) |
| `SFLOW-V5-x-17` | Default datagram max size SHOULD be 1400 bytes (Transport) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-18` | Default max header size SHOULD be 128 bytes for sampled_header (Raw Packet Header) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-19` | Counter polls SHOULD be staggered across sources with randomized initial offset (Counter Polling) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-20` | Per-source skip counter SHOULD be initialized to random value in [1, 2*N-1] for 1-in-N sampling (Packet Sampling) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-21` | UDP port 6343 SHOULD be used (IANA-assigned) (Transport) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-22` | Multiple sFlow instances MAY exist per data source (Agent Architecture) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-23` | Counter samples MAY piggyback on flow sample datagrams when space permits (Counter Polling) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-24` | Agents MAY adjust sampling rates to hardware-supported values; actual rate MUST be reported in each flow sample (Packet Sampling) | MAY | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`SFLOW-V5-x-9`](#sflow-v5-x-9) Sequence numbers MUST be per-agent (datagram-level) and per-source (sample-level), unsigned 32-bit, wrapping (Transport) | {gap}, no test | per-source sample-level sequences are correct, but for one sub-agent the counter-datagram and flow-datagram streams keep two independent datagramSeq counters, so a collector sees two overlapping datagram-level sequence spaces instead of one (internal/plugins/flowexport/sflow/adapter.go:18, flow_adapter.go:32) |
| [`SFLOW-V5-x-12`](#sflow-v5-x-12) Expanded sample types MUST be used when ifIndex exceeds 2^24-1 or 2^30-1 (Expanded Flow/Counter Sample) | {gap}, no test | ze implements no flow_sample_expanded (format 3) or counters_sample_expanded (format 4); a large ifIndex is silently truncated by a 24-bit mask on source_id rather than switching to the expanded encoding (internal/plugins/flowexport/sflow/counter.go:49, flow.go:48) |
| [`SFLOW-V5-x-13`](#sflow-v5-x-13) Unknown record formats MUST be skipped using the opaque length prefix (Record Wrappers) | no test | no test carries this requirement id; annotated {not-applicable}: skipping unknown formats on receive is a collector behavior; ze is an sFlow exporter only with no sFlow decode path, though it does emit the length prefixes that let a collector skip (internal/plugins/flowexport/sflow/counter.go:60-61, flow.go:131-132) |
| [`SFLOW-V5-x-16`](#sflow-v5-x-16) Unavailable counter fields MUST be set to max value for the type (0xFFFFFFFF for u32, 0xFFFFFFFFFFFFFFFF for u64) (Implementation Guidance) | {gap}, no test | interfaceCountersFrom leaves fields the kernel does not expose (ifInUnknownProtos, ifInBroadcastPkts, ifOutMulticastPkts, ifOutBroadcastPkts) at zero rather than the required max-value unavailable sentinel, so a collector cannot distinguish true-zero from unavailable (internal/plugins/flowexport/register.go:343-361) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`SFLOW-V5-x-1`](#sflow-v5-x-1)

Datagram version field MUST be set to 5 (Datagram Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowDatagramHeaderIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L11) | unit/verify | unproven |

### [`SFLOW-V5-x-2`](#sflow-v5-x-2)

Agent address MUST be a stable IP (e.g., loopback) that uniquely identifies the device across reboots (Agent Architecture)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowDatagramHeaderIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L12) | unit/verify | unproven |

### [`SFLOW-V5-x-3`](#sflow-v5-x-3)

Agent address + sub_agent_id MUST uniquely identify a sampling entity (Agent Architecture)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowDatagramHeaderIPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L53) | unit/verify | unproven |

### [`SFLOW-V5-x-4`](#sflow-v5-x-4)

Each sub-agent MUST maintain its own sequence number space (Agent Architecture)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowMultiInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L93) | unit/verify | unproven |

### [`SFLOW-V5-x-5`](#sflow-v5-x-5)

Datagram size MUST NOT exceed path MTU (Transport)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowMultiInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L94) | unit/verify | unproven |

### [`SFLOW-V5-x-6`](#sflow-v5-x-6)

Samples MUST NOT be held more than 1 second before sending (Transport)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExportFlowSampleDispatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L148) | unit/verify | unproven |

### [`SFLOW-V5-x-7`](#sflow-v5-x-7)

All structures MUST use XDR encoding: 4-byte alignment, big-endian (Datagram Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowIfCounters`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counter_test.go#L35) | unit/verify | unproven |

### [`SFLOW-V5-x-8`](#sflow-v5-x-8)

Variable-length arrays and opaque data MUST be prefixed by a 4-byte count and padded to 4-byte boundary (XDR Encoding)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowSampledHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_test.go#L70) | unit/verify | unproven |

### [`SFLOW-V5-x-9`](#sflow-v5-x-9)

Sequence numbers MUST be per-agent (datagram-level) and per-source (sample-level), unsigned 32-bit, wrapping (Transport)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-9, so no unit is bound to it.

### [`SFLOW-V5-x-10`](#sflow-v5-x-10)

flow_sample MUST include the actual sampling_rate used by the agent (Flow Sample)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowFlowSample`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_test.go#L9) | unit/verify | unproven |

### [`SFLOW-V5-x-11`](#sflow-v5-x-11)

sample_pool MUST track total packets seen by the data source (Flow Sample)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowFlowSamplePoolTracksTotal`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_adapter_test.go#L52) | unit/verify | unproven |

### [`SFLOW-V5-x-12`](#sflow-v5-x-12)

Expanded sample types MUST be used when ifIndex exceeds 2^24-1 or 2^30-1 (Expanded Flow/Counter Sample)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-12, so no unit is bound to it.

### [`SFLOW-V5-x-13`](#sflow-v5-x-13)

Unknown record formats MUST be skipped using the opaque length prefix (Record Wrappers)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-13, so no unit is bound to it.

### [`SFLOW-V5-x-14`](#sflow-v5-x-14)

Counter samples MUST be produced at the configured polling interval for each data source (Counter Polling)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowCounterPollBeforeInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L295) | unit/verify | unproven |
| positive | [`TestSFlowCounterPollAtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L276) | unit/verify | unproven |

### [`SFLOW-V5-x-15`](#sflow-v5-x-15)

All counters MUST be cumulative since boot (Counter Polling)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterfaceCountersFromCumulative`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/register_test.go#L128) | unit/verify | unproven |

### [`SFLOW-V5-x-16`](#sflow-v5-x-16)

Unavailable counter fields MUST be set to max value for the type (0xFFFFFFFF for u32, 0xFFFFFFFFFFFFFFFF for u64) (Implementation Guidance)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-16, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for SFLOW-V5, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes SFLOW-V5, so its obligations are stated where they were written.
