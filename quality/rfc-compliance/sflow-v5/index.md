# SFLOW-V5 - sFlow: A Method for Monitoring Traffic in Switched and Routed Networks

Experimental. Every requirement this repository extracted from SFLOW-V5, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 41.2% | 14 of 34 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 32.4% | 11 of 34 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 34 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 41.0% | 16 of 39 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 34 | of 42 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 34 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 2.9% | 1 of 34 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 2.9% | 1 of 34 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 2.9% | 1 of 34 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 17.6% | 6 of 34 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 34 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 42 |
| Gated MUST-level | 34 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 6 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 39 |
| Tagged units | 39 |
| Recorded audit verdicts | 0 |
| Discrimination records | 16 |
| Summary | `rfc/short/sflow-v5.md` |
| Requirement shard | `rfc/requirements/sflow-v5.md` |
| RFC text | `rfc/full/sflow-v5.txt` |

## Enrolment

Enrolled: sFlow Version 5 exporter/agent. The checklist includes transport, sample encoding, counter availability and sampling obligations from the 2026-09-21 extraction walk. Tests cover parts of these obligations; the checklist retains the remaining gaps and uncovered requirements.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- Flow export alongside NetFlow v9 and IPFIX. Unavailable counters carry the maximum-value sentinel. Counter and flow encoders share a datagram sequence
- all sources use expanded sample formats with full-width interface indexes. Counter generation changes restart source sample sequences. Other uncovered obligations remain in the checklist. Statistical sampling requirements SFLOW-V5-x-37 through x-41 remain unverified. Linux filter installation/readback tests cover configuration and lifecycle, not RNG range, packet-selection probability, one consideration per packet, independent sampling draws or long-term convergence. Current exporter and privileged kernel scenarios must be run before publishing verified coverage.


**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 14 | one part of the gated population |
| Annotated instead of tested | 20 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **34** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (14):** [`SFLOW-V5-x-9`](#sflow-v5-x-9), [`SFLOW-V5-x-12`](#sflow-v5-x-12), [`SFLOW-V5-x-14`](#sflow-v5-x-14), [`SFLOW-V5-x-16`](#sflow-v5-x-16), [`SFLOW-V5-x-25`](#sflow-v5-x-25), [`SFLOW-V5-x-26`](#sflow-v5-x-26), [`SFLOW-V5-x-27`](#sflow-v5-x-27), [`SFLOW-V5-x-28`](#sflow-v5-x-28), [`SFLOW-V5-x-29`](#sflow-v5-x-29), [`SFLOW-V5-x-30`](#sflow-v5-x-30), [`SFLOW-V5-x-31`](#sflow-v5-x-31), [`SFLOW-V5-x-32`](#sflow-v5-x-32), [`SFLOW-V5-x-33`](#sflow-v5-x-33), [`SFLOW-V5-x-34`](#sflow-v5-x-34)

**Annotated instead of tested (20):** [`SFLOW-V5-x-1`](#sflow-v5-x-1), [`SFLOW-V5-x-2`](#sflow-v5-x-2), [`SFLOW-V5-x-3`](#sflow-v5-x-3), [`SFLOW-V5-x-4`](#sflow-v5-x-4), [`SFLOW-V5-x-5`](#sflow-v5-x-5), [`SFLOW-V5-x-6`](#sflow-v5-x-6), [`SFLOW-V5-x-7`](#sflow-v5-x-7), [`SFLOW-V5-x-8`](#sflow-v5-x-8), [`SFLOW-V5-x-10`](#sflow-v5-x-10), [`SFLOW-V5-x-11`](#sflow-v5-x-11), [`SFLOW-V5-x-13`](#sflow-v5-x-13), [`SFLOW-V5-x-15`](#sflow-v5-x-15), [`SFLOW-V5-x-35`](#sflow-v5-x-35), [`SFLOW-V5-x-36`](#sflow-v5-x-36), [`SFLOW-V5-x-37`](#sflow-v5-x-37), [`SFLOW-V5-x-38`](#sflow-v5-x-38), [`SFLOW-V5-x-39`](#sflow-v5-x-39), [`SFLOW-V5-x-40`](#sflow-v5-x-40), [`SFLOW-V5-x-41`](#sflow-v5-x-41), [`SFLOW-V5-x-42`](#sflow-v5-x-42)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `SFLOW-V5-x-1` | Datagram version field MUST be set to 5 (Datagram Format) | MUST | x | **positive:** `unit/verify` [`TestSFlowDatagramHeaderIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L11). **negative:** no negative test. **{single-polarity}:** WriteDatagramHeader unconditionally writes the compile-time constant Version=5 into every datagram, so there is no other-version code path to reject (internal/plugins/flowexport/sflow/encoder.go:40, :14) |
| `SFLOW-V5-x-2` | Agent address MUST be a stable IP (e.g., loopback) that uniquely identifies the device across reboots (Agent Architecture) | MUST | x | **positive:** `unit/verify` [`TestSFlowDatagramHeaderIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L12). **negative:** no negative test. **{single-polarity}:** the operator-configured agent address is written verbatim into every datagram header and validated as a well-formed IP; stability/uniqueness is a config-value property with no exporter reject path (internal/plugins/flowexport/sflow/encoder.go:43-57, internal/plugins/flowexport/config.go:379-383) |
| `SFLOW-V5-x-3` | Agent address + sub_agent_id MUST uniquely identify a sampling entity (Agent Architecture) | MUST | x | **positive:** `unit/verify` [`TestSFlowDatagramHeaderIPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L53). **negative:** no negative test. **{single-polarity}:** both agent_address and sub_agent_id are emitted in every datagram header by construction, and tuple uniqueness is an operator-config obligation (internal/plugins/flowexport/sflow/encoder.go:59, :43-57) |
| `SFLOW-V5-x-4` | Each sub-agent MUST maintain its own sequence number space (Agent Architecture) | MUST | x | **positive:** `unit/verify` [`TestSFlowMultiInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L93). **negative:** no negative test. **{single-polarity}:** each collector owns its UDP Sender sequence shared by counter and flow encoders, while per-source sample sequences belong to its encoders (internal/plugins/flowexport/sender.go Sender.Sequence, internal/plugins/flowexport/sflow/adapter.go CounterEncoder, internal/plugins/flowexport/sflow/flow_adapter.go FlowEncoder) |
| `SFLOW-V5-x-5` | Datagram size MUST NOT exceed path MTU (Transport) | MUST | x | **positive:** `unit/verify` [`TestSFlowMultiInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/encoder_test.go#L94). **negative:** no negative test. **{single-polarity}:** encoders bound the payload, including padding, to the collector's max-datagram-size and Sender.Send refuses larger payloads; the operator must configure the bound for the path MTU (internal/plugins/flowexport/sender.go Sender.Send, internal/plugins/flowexport/sflow/encoder.go writeCounterDatagrams, internal/plugins/flowexport/sflow/flow_adapter.go EncodeFlowSample) |
| `SFLOW-V5-x-6` | Samples MUST NOT be held more than 1 second before sending (Transport) | MUST | x | **positive:** `unit/verify` [`TestExportFlowSampleDispatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L99). **negative:** no negative test. **{single-polarity}:** counter and flow samples are encoded and sent synchronously with no buffering queue, so a sample is never held beyond a sub-millisecond encode and there is no holding timer to test negatively (internal/plugins/flowexport/exporter.go:204, internal/plugins/flowexport/sflow/adapter.go:47-51) |
| `SFLOW-V5-x-7` | All structures MUST use XDR encoding: 4-byte alignment, big-endian (Datagram Format) | MUST | x | **positive:** `unit/verify` [`TestSFlowIfCounters`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counter_test.go#L35). **negative:** no negative test. **{single-polarity}:** every field is written via binary.BigEndian with 4-byte-aligned opaque padding, exporter-only with no decode path to reject a wrong endianness (internal/plugins/flowexport/sflow/counter.go:36, flow.go:123-128) |
| `SFLOW-V5-x-8` | Variable-length arrays and opaque data MUST be prefixed by a 4-byte count and padded to 4-byte boundary (XDR Encoding) | MUST | x | **positive:** `unit/verify` [`TestSFlowSampledHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_test.go#L129). **negative:** no negative test. **{single-polarity}:** the sampled_header opaque and the extended_gateway arrays are all written with a 4-byte count prefix and zero-padded to a 4-byte boundary by construction (internal/plugins/flowexport/sflow/flow.go:116-128, :187-208) |
| `SFLOW-V5-x-9` | Sequence numbers MUST be per-agent (datagram-level) and per-source (sample-level), unsigned 32-bit, wrapping (Transport) | MUST | x | **positive:** `unit/verify` [`TestSFlowMixedDatagramSequenceWraps`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L203). **negative:** `unit/verify` [`TestSFlowCounterResetDoesNotResetDatagrams`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L240) |
| `SFLOW-V5-x-10` | flow_sample MUST include the actual sampling_rate used by the agent (Flow Sample) | MUST | x | **positive:** `unit/verify` [`TestSFlowFlowSample`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_test.go#L9). **negative:** no negative test. **{single-polarity}:** EncodeFlowSample writes the kernel-reported actual rate into every flow_sample, emitted unconditionally with no reject path (internal/plugins/flowexport/sflow/flow_adapter.go:78-79, flow.go:52) |
| `SFLOW-V5-x-11` | sample_pool MUST track total packets seen by the data source (Flow Sample) | MUST | x | **positive:** `unit/verify` [`TestSFlowFlowSamplePoolTracksTotal`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_adapter_test.go#L51). **negative:** no negative test. **{single-polarity}:** EncodeFlowSample computes sample_pool as the saturated product of cumulative samples and rate and writes it into every flow_sample, exporter-only with no negative form (internal/plugins/flowexport/sflow/flow_adapter.go:73-79, flow.go:56) |
| `SFLOW-V5-x-12` | Expanded sample types MUST be used when ifIndex exceeds 2^24-1 or 2^30-1 (Expanded Flow/Counter Sample) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5ExpandedEncodingForEveryInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L242). **negative:** `unit/verify` [`TestSFlowCounterSampleSourceIDOverflow`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counter_test.go#L216) |
| `SFLOW-V5-x-13` | Unknown record formats MUST be skipped using the opaque length prefix (Record Wrappers) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** skipping unknown formats on receive is a collector behavior; ze is an sFlow exporter only with no sFlow decode path, though it does emit the length prefixes that let a collector skip (internal/plugins/flowexport/sflow/counter.go:60-61, flow.go:131-132) |
| `SFLOW-V5-x-14` | Counter samples MUST be produced at the configured polling interval for each data source (Counter Polling) | MUST | x | **positive:** `unit/verify` [`TestSFlowCounterPollAtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L227). **negative:** `unit/verify` [`TestSFlowCounterPollBeforeInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L246) |
| `SFLOW-V5-x-15` | All counters MUST be cumulative since boot (Counter Polling) | MUST | x | **positive:** `unit/verify` [`TestInterfaceCountersFromCumulative`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/register_test.go#L128). **negative:** no negative test. **{single-polarity}:** interfaceCountersFrom copies the raw cumulative kernel counters straight through with no differencing, so exported if_counters are cumulative by construction (internal/plugins/flowexport/register.go:343-358, snapshot.go:10-13) |
| `SFLOW-V5-x-16` | Unavailable counter fields MUST be set to max value for the type (0xFFFFFFFF for u32, 0xFFFFFFFFFFFFFFFF for u64) (Implementation Guidance) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5UnavailableCountersCarrySentinelEveryPoll`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/counters_sflow_v5_test.go#L49). **negative:** `unit/verify` [`TestSFlowV5AvailableCountersNeverTurnUnavailable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/counters_sflow_v5_test.go#L66) |
| `SFLOW-V5-x-25` | "the sFlow Agent must not wait for a buffer to fill with samples before sending the sFlow Datagram" (Transport) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5SampleSentWithoutWaitingForBuffer`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L147). **negative:** `unit/verify` [`TestSFlowV5NoDatagramHeldForMoreSamples`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L170) |
| `SFLOW-V5-x-26` | "If counters must be sent in order to satisfy the maximum sampling interval then a datagram must be sent containing the outstanding counters." (Counter Polling) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5OutstandingCountersAllSent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L192). **negative:** `unit/verify` [`TestSFlowV5TailDatagramNotWithheld`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L215) |
| `SFLOW-V5-x-27` | "An agent must not mix compact/expanded encodings." An agent that will never use ifIndex numbers >= 2^24 "must use compact encodings for all interfaces", otherwise "the expanded formats must be used for all interfaces" (Expanded Flow/Counter Sample) | MUST NOT | x | **positive:** `unit/verify` [`TestSFlowV5ExpandedEncodingForEveryInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L241). **negative:** `unit/verify` [`TestSFlowV5CompactFormatsNeverMixedIn`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L278) |
| `SFLOW-V5-x-28` | "If the agent resets the sample_pool then it must also reset the sequence_number" of the flow sample (Flow Sample) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5PoolAndSequenceResetTogether`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L309). **negative:** `unit/verify` [`TestSFlowV5PoolNeverResetsWithoutSequence`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L326) |
| `SFLOW-V5-x-29` | "If the agent resets any of the counters then it must also reset the sequence_number" of the counter sample (Counter Sample) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5SequenceResetWithCounters`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L38). **negative:** `unit/verify` [`TestSFlowV5SequenceNeverResetWithoutDiscontinuity`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L57) |
| `SFLOW-V5-x-30` | "In the case of ifIndex-based source_id's the sequence number must be reset each time ifCounterDiscontinuityTime changes." (Counter Sample) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5SequenceResetOnDiscontinuityTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/discontinuity_sflow_v5_test.go#L15). **negative:** `unit/verify` [`TestSFlowV5SequenceKeptWhileDiscontinuityTimeSteady`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/discontinuity_sflow_v5_test.go#L34) |
| `SFLOW-V5-x-31` | "Each sFlowDataSource must be associated with only one sub-agent", and "The association between sFlowDataSource and sub-agent must remain constant for the entire duration of an sFlow session." (Agent Architecture) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5DataSourcesCarryTheirSubAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L351). **negative:** `unit/verify` [`TestSFlowV5SubAgentBindingConstantForSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L369) |
| `SFLOW-V5-x-32` | "Within any given sFlow session a particular counter must be always available, or always unavailable." (Counter Polling) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5UnavailableCountersCarrySentinelEveryPoll`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/counters_sflow_v5_test.go#L50). **negative:** `unit/verify` [`TestSFlowV5AvailableCountersNeverTurnUnavailable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/counters_sflow_v5_test.go#L67) |
| `SFLOW-V5-x-33` | "A flow_sample must contain packet header information." (Flow Sample) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5FlowSampleCarriesPacketHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L388). **negative:** `unit/verify` [`TestSFlowV5FlowSampleNeverWithoutHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L408) |
| `SFLOW-V5-x-34` | "Any octets added to the frame_length to compensate for encapsulations removed by the underlying hardware must also be added to the stripped count." (Raw Packet Header) | MUST | x | **positive:** `unit/verify` [`TestSFlowV5FrameLengthAndStrippedIncludeFCS`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L89). **negative:** `unit/verify` [`TestSFlowV5FrameCompensationNeverUnstripped`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L105) |
| `SFLOW-V5-x-35` | "Trailing encapsulation data for the outermost protocol layer included in the sampled header must be stripped", as must trailing data "corresponding to any leading encapsulations that were stripped" and "Outer encapsulations that are ambiguous, or not one of the standard header_protocol" (Raw Packet Header) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux NIC driver and psample; internal/plugins/flowexport/sampling/tc_linux.go::buildSampleFilter installs act_sample on the ingress hook, the NIC removes the Ethernet FCS, the only trailer of the outermost layer, before the kernel builds the skb that act_sample copies, and internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample copies that header verbatim and strips no leading encapsulation, so no trailing encapsulation octets reach the sampled header |
| `SFLOW-V5-x-36` | "extended_switch data must always be reported to describe the ingress/egress VLAN information for the packet." (Extended Switch) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** the agent emits no extended_switch record; internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample writes one flow record per sample, the sampled_header, so ingress and egress VLAN information is never reported |
| `SFLOW-V5-x-37` | "The random number generator must ensure that all numbers in the range between its maximum and minimum values of the distribution are possible" (Packet Sampling) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** Linux act_sample owns the random draw; the current source and kernel filter lifecycle test do not prove its output range |
| `SFLOW-V5-x-38` | Packet Flow Sampling "must ensure that any packet observed at a Data Source has an equal chance of being sampled, irrespective of the Packet Flow(s) to which it belongs" (Packet Sampling) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** MatchAll filter configuration is not behavioral proof of equal packet-selection probability |
| `SFLOW-V5-x-39` | "Each packet must only be considered once for sampling, irrespective of the number of ports it will be forwarded to." (Packet Sampling) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** ingress filter readback does not observe how often forwarded or replicated packets are considered by the sampler |
| `SFLOW-V5-x-40` | "Each sFlow sampler instance must operate independently of all other instances", and "Setting an attribute of one sampler must not alter the the behavior and settings of other sampler instances." (Agent Architecture) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** the kernel lifecycle test checks configuration isolation; independent sampling behavior remains unproven |
| `SFLOW-V5-x-41` | "The sampling algorithm must converge so that over time the number of packets sampled approaches 1/Nth of the total number of packets in the monitored flows." (Packet Sampling) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** configuring an act_sample rate does not measure long-term convergence |
| `SFLOW-V5-x-42` | After an agent adjusts a configured sampling rate, "The sampling rate must stay at its new value and never automatically return to the originally configured value." (Packet Sampling) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The Agent may implement an automated one-way backoff of the Sampling Rate that triggers whenever an excessive number of samples per second is generated."; ze implements no automatic backoff, so the rate never changes by itself. internal/plugins/flowexport/sampling/tc_linux.go::SetupSampling installs the configured rate into act_sample and nothing else writes it |
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
| [`SFLOW-V5-x-13`](#sflow-v5-x-13) Unknown record formats MUST be skipped using the opaque length prefix (Record Wrappers) | no test | no test carries this requirement id; annotated {not-applicable}: skipping unknown formats on receive is a collector behavior; ze is an sFlow exporter only with no sFlow decode path, though it does emit the length prefixes that let a collector skip (internal/plugins/flowexport/sflow/counter.go:60-61, flow.go:131-132) |
| [`SFLOW-V5-x-35`](#sflow-v5-x-35) "Trailing encapsulation data for the outermost protocol layer included in the sampled header must be stripped", as must trailing data "corresponding to any leading encapsulations that were stripped" and "Outer encapsulations that are ambiguous, or not one of the standard header_protocol" (Raw Packet Header) | no test | no test carries this requirement id; annotated {lower-layer}: Linux NIC driver and psample; internal/plugins/flowexport/sampling/tc_linux.go::buildSampleFilter installs act_sample on the ingress hook, the NIC removes the Ethernet FCS, the only trailer of the outermost layer, before the kernel builds the skb that act_sample copies, and internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample copies that header verbatim and strips no leading encapsulation, so no trailing encapsulation octets reach the sampled header |
| [`SFLOW-V5-x-36`](#sflow-v5-x-36) "extended_switch data must always be reported to describe the ingress/egress VLAN information for the packet." (Extended Switch) | {gap}, no test | the agent emits no extended_switch record; internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample writes one flow record per sample, the sampled_header, so ingress and egress VLAN information is never reported |
| [`SFLOW-V5-x-37`](#sflow-v5-x-37) "The random number generator must ensure that all numbers in the range between its maximum and minimum values of the distribution are possible" (Packet Sampling) | {gap}, no test | Linux act_sample owns the random draw; the current source and kernel filter lifecycle test do not prove its output range |
| [`SFLOW-V5-x-38`](#sflow-v5-x-38) Packet Flow Sampling "must ensure that any packet observed at a Data Source has an equal chance of being sampled, irrespective of the Packet Flow(s) to which it belongs" (Packet Sampling) | {gap}, no test | MatchAll filter configuration is not behavioral proof of equal packet-selection probability |
| [`SFLOW-V5-x-39`](#sflow-v5-x-39) "Each packet must only be considered once for sampling, irrespective of the number of ports it will be forwarded to." (Packet Sampling) | {gap}, no test | ingress filter readback does not observe how often forwarded or replicated packets are considered by the sampler |
| [`SFLOW-V5-x-40`](#sflow-v5-x-40) "Each sFlow sampler instance must operate independently of all other instances", and "Setting an attribute of one sampler must not alter the the behavior and settings of other sampler instances." (Agent Architecture) | {gap}, no test | the kernel lifecycle test checks configuration isolation; independent sampling behavior remains unproven |
| [`SFLOW-V5-x-41`](#sflow-v5-x-41) "The sampling algorithm must converge so that over time the number of packets sampled approaches 1/Nth of the total number of packets in the monitored flows." (Packet Sampling) | {gap}, no test | configuring an act_sample rate does not measure long-term convergence |
| [`SFLOW-V5-x-42`](#sflow-v5-x-42) After an agent adjusts a configured sampling rate, "The sampling rate must stay at its new value and never automatically return to the originally configured value." (Packet Sampling) | no test | no test carries this requirement id; annotated {feature-declined}: "The Agent may implement an automated one-way backoff of the Sampling Rate that triggers whenever an excessive number of samples per second is generated."; ze implements no automatic backoff, so the rate never changes by itself. internal/plugins/flowexport/sampling/tc_linux.go::SetupSampling installs the configured rate into act_sample and nothing else writes it |

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
| positive | [`TestExportFlowSampleDispatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L99) | unit/verify | unproven |

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
| positive | [`TestSFlowSampledHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_test.go#L129) | unit/verify | unproven |

### [`SFLOW-V5-x-9`](#sflow-v5-x-9)

Sequence numbers MUST be per-agent (datagram-level) and per-source (sample-level), unsigned 32-bit, wrapping (Transport)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowCounterResetDoesNotResetDatagrams`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L240) | unit/verify | unproven |
| positive | [`TestSFlowMixedDatagramSequenceWraps`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L203) | unit/verify | unproven |

### [`SFLOW-V5-x-10`](#sflow-v5-x-10)

flow_sample MUST include the actual sampling_rate used by the agent (Flow Sample)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowFlowSample`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_test.go#L9) | unit/verify | revert, verified |

### [`SFLOW-V5-x-11`](#sflow-v5-x-11)

sample_pool MUST track total packets seen by the data source (Flow Sample)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowFlowSamplePoolTracksTotal`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/flow_adapter_test.go#L51) | unit/verify | unproven |

### [`SFLOW-V5-x-12`](#sflow-v5-x-12)

Expanded sample types MUST be used when ifIndex exceeds 2^24-1 or 2^30-1 (Expanded Flow/Counter Sample)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowCounterSampleSourceIDOverflow`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counter_test.go#L216) | unit/verify | unproven |
| positive | [`TestSFlowV5ExpandedEncodingForEveryInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L242) | unit/verify | revert, verified |

### [`SFLOW-V5-x-13`](#sflow-v5-x-13)

Unknown record formats MUST be skipped using the opaque length prefix (Record Wrappers)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-13, so no unit is bound to it.

### [`SFLOW-V5-x-14`](#sflow-v5-x-14)

Counter samples MUST be produced at the configured polling interval for each data source (Counter Polling)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowCounterPollBeforeInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L246) | unit/verify | unproven |
| positive | [`TestSFlowCounterPollAtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/exporter_test.go#L227) | unit/verify | unproven |

### [`SFLOW-V5-x-15`](#sflow-v5-x-15)

All counters MUST be cumulative since boot (Counter Polling)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterfaceCountersFromCumulative`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/register_test.go#L128) | unit/verify | unproven |

### [`SFLOW-V5-x-16`](#sflow-v5-x-16)

Unavailable counter fields MUST be set to max value for the type (0xFFFFFFFF for u32, 0xFFFFFFFFFFFFFFFF for u64) (Implementation Guidance)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5AvailableCountersNeverTurnUnavailable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/counters_sflow_v5_test.go#L66) | unit/verify | unproven |
| positive | [`TestSFlowV5UnavailableCountersCarrySentinelEveryPoll`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/counters_sflow_v5_test.go#L49) | unit/verify | unproven |

### [`SFLOW-V5-x-25`](#sflow-v5-x-25)

"the sFlow Agent must not wait for a buffer to fill with samples before sending the sFlow Datagram" (Transport)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5NoDatagramHeldForMoreSamples`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L170) | unit/verify | revert, verified |
| positive | [`TestSFlowV5SampleSentWithoutWaitingForBuffer`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L147) | unit/verify | revert, verified |

### [`SFLOW-V5-x-26`](#sflow-v5-x-26)

"If counters must be sent in order to satisfy the maximum sampling interval then a datagram must be sent containing the outstanding counters." (Counter Polling)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5TailDatagramNotWithheld`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L215) | unit/verify | revert, verified |
| positive | [`TestSFlowV5OutstandingCountersAllSent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L192) | unit/verify | revert, verified |

### [`SFLOW-V5-x-27`](#sflow-v5-x-27)

"An agent must not mix compact/expanded encodings." An agent that will never use ifIndex numbers >= 2^24 "must use compact encodings for all interfaces", otherwise "the expanded formats must be used for all interfaces" (Expanded Flow/Counter Sample)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5CompactFormatsNeverMixedIn`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L278) | unit/verify | revert, verified |
| positive | [`TestSFlowV5ExpandedEncodingForEveryInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L241) | unit/verify | revert, verified |

### [`SFLOW-V5-x-28`](#sflow-v5-x-28)

"If the agent resets the sample_pool then it must also reset the sequence_number" of the flow sample (Flow Sample)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5PoolNeverResetsWithoutSequence`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L326) | unit/verify | revert, verified |
| positive | [`TestSFlowV5PoolAndSequenceResetTogether`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L309) | unit/verify | revert, verified |

### [`SFLOW-V5-x-29`](#sflow-v5-x-29)

"If the agent resets any of the counters then it must also reset the sequence_number" of the counter sample (Counter Sample)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5SequenceNeverResetWithoutDiscontinuity`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L57) | unit/verify | unproven |
| positive | [`TestSFlowV5SequenceResetWithCounters`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L38) | unit/verify | unproven |

### [`SFLOW-V5-x-30`](#sflow-v5-x-30)

"In the case of ifIndex-based source_id's the sequence number must be reset each time ifCounterDiscontinuityTime changes." (Counter Sample)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5SequenceKeptWhileDiscontinuityTimeSteady`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/discontinuity_sflow_v5_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestSFlowV5SequenceResetOnDiscontinuityTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/discontinuity_sflow_v5_test.go#L15) | unit/verify | revert, verified |

### [`SFLOW-V5-x-31`](#sflow-v5-x-31)

"Each sFlowDataSource must be associated with only one sub-agent", and "The association between sFlowDataSource and sub-agent must remain constant for the entire duration of an sFlow session." (Agent Architecture)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5SubAgentBindingConstantForSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L369) | unit/verify | revert, verified |
| positive | [`TestSFlowV5DataSourcesCarryTheirSubAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L351) | unit/verify | revert, verified |

### [`SFLOW-V5-x-32`](#sflow-v5-x-32)

"Within any given sFlow session a particular counter must be always available, or always unavailable." (Counter Polling)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5AvailableCountersNeverTurnUnavailable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/counters_sflow_v5_test.go#L67) | unit/verify | unproven |
| positive | [`TestSFlowV5UnavailableCountersCarrySentinelEveryPoll`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/counters_sflow_v5_test.go#L50) | unit/verify | unproven |

### [`SFLOW-V5-x-33`](#sflow-v5-x-33)

"A flow_sample must contain packet header information." (Flow Sample)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5FlowSampleNeverWithoutHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L408) | unit/verify | revert, verified |
| positive | [`TestSFlowV5FlowSampleCarriesPacketHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L388) | unit/verify | revert, verified |

### [`SFLOW-V5-x-34`](#sflow-v5-x-34)

"Any octets added to the frame_length to compensate for encapsulations removed by the underlying hardware must also be added to the stripped count." (Raw Packet Header)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5FrameCompensationNeverUnstripped`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L105) | unit/verify | unproven |
| positive | [`TestSFlowV5FrameLengthAndStrippedIncludeFCS`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/counters_sflow_v5_test.go#L89) | unit/verify | unproven |

### [`SFLOW-V5-x-35`](#sflow-v5-x-35)

"Trailing encapsulation data for the outermost protocol layer included in the sampled header must be stripped", as must trailing data "corresponding to any leading encapsulations that were stripped" and "Outer encapsulations that are ambiguous, or not one of the standard header_protocol" (Raw Packet Header)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-35, so no unit is bound to it.

### [`SFLOW-V5-x-36`](#sflow-v5-x-36)

"extended_switch data must always be reported to describe the ingress/egress VLAN information for the packet." (Extended Switch)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-36, so no unit is bound to it.

### [`SFLOW-V5-x-37`](#sflow-v5-x-37)

"The random number generator must ensure that all numbers in the range between its maximum and minimum values of the distribution are possible" (Packet Sampling)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-37, so no unit is bound to it.

### [`SFLOW-V5-x-38`](#sflow-v5-x-38)

Packet Flow Sampling "must ensure that any packet observed at a Data Source has an equal chance of being sampled, irrespective of the Packet Flow(s) to which it belongs" (Packet Sampling)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-38, so no unit is bound to it.

### [`SFLOW-V5-x-39`](#sflow-v5-x-39)

"Each packet must only be considered once for sampling, irrespective of the number of ports it will be forwarded to." (Packet Sampling)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-39, so no unit is bound to it.

### [`SFLOW-V5-x-40`](#sflow-v5-x-40)

"Each sFlow sampler instance must operate independently of all other instances", and "Setting an attribute of one sampler must not alter the the behavior and settings of other sampler instances." (Agent Architecture)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-40, so no unit is bound to it.

### [`SFLOW-V5-x-41`](#sflow-v5-x-41)

"The sampling algorithm must converge so that over time the number of packets sampled approaches 1/Nth of the total number of packets in the monitored flows." (Packet Sampling)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-41, so no unit is bound to it.

### [`SFLOW-V5-x-42`](#sflow-v5-x-42)

After an agent adjusts a configured sampling rate, "The sampling rate must stay at its new value and never automatically return to the originally configured value." (Packet Sampling)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-42, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/sflow-v5.txt |
| Source fingerprint | cd9e27ebcba6e68b |
| Record | rfc/extraction/sflow-v5.json |
| Mapped sentences | 21 |
| Declined as scope | 37 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | sFlow Datagram Format | 30 | walked | sFlow Datagram Format. The specification's main normative section: 30 sites, 15 mapped to SFLOW-V5-x-6, x-12, x-13 and x-25 through x-36, and 15 excluded, of which 9 repeat a sentence already mapped, 4 bind another role (three an sFlow Collector, one the author of a new sFlow structure definition) and 2 state no obligation. Ten ids are declared unsourced here. sFlow v5 writes this section as XDR type definitions and explanatory prose, so the capitalised MUST-level scan raises no site for any of them. x-1 rests on 'enum datagram_version { VERSION5 = 5 }' with 'union sample_datagram_type (datagram_version version) { case VERSION5: sample_datagram_v5 datagram; }', which admit one version value and name no obligation. x-3 rests on the sub_agent_id comment, 'Used to distinguishing between datagram streams from separate agent sub entities within an device', read with the agent_address comment 'IP address of sampling agent, sFlowAgentAddress'. x-4 rests on the sample_datagram_v5 sequence_number comment, 'Incremented with each sample datagram generated by a sub-agent within an agent'. x-7 rests on 'The format of the sFlow datagram is specified using the XDR standard [32]', which imports 4-byte alignment and big-endian order from XDR instead of restating them. x-8 rests on that same import together with the opaque<> and counters<> array notation the section uses throughout, whose 4-byte count prefix and padding are XDR's. x-9 rests on the 'unsigned int sequence_number' declarations, one per-agent in sample_datagram_v5 and one per-source in flow_sample and counters_sample, each an XDR unsigned 32-bit value that wraps. x-10 rests on 'unsigned int sampling_rate; /* sFlowPacketSamplingRate */' inside flow_sample. x-11 rests on 'unsigned int sample_pool; /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */'. x-15 rests on 'If counter samples are lost then new values will be sent during the next polling interval. The chance of an undetected counter wrap is negligible.', which holds for counters that accumulate and not for per-interval deltas, read with '/* Generic Interface Counters - see RFC 2233 */' and with 'An available counter may temporarily have the max value just before it rolls to zero.'. x-16 rests on 'Unknown counter. Use the maximum counter value to indicate that the counter is not available.' |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 3 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | Counter Sampling | 1 | walked | Counter Sampling. One site, excluded as not a requirement: 'Counters are only added to the datagram if the sources are within a short period, 5 seconds say, of failing to meet the required Sampling Interval', which describes one strategy and prescribes none. One id is declared unsourced here. x-14 rests on 'A maximum Sampling Interval is assigned to each sFlow Instance associated with an interface Data Source, but the sFlow Agent is free to schedule polling in order maximize internal efficiency', read with 'Periodically, say every second, the sFlow Agent examines the list of counter sources and sends any counters that need to be sent to meet the sampling interval requirement'. Both sentences are indicative, so the MUST-level scan raises no site for them. |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 1 | walked | not stated |
| `4.2.2` | not stated | 1 | walked | not stated |
| `4.2.3` | not stated | 0 | walked | not stated |
| `4.3` | Definitions | 18 | walked | Definitions. The SFLOW-MIB module itself: 18 sites, 3 mapped to SFLOW-V5-x-39, x-40 and x-41, and 15 excluded, of which 2 repeat a sentence already mapped and 13 bind an SNMP agent or an SNMP management station. Ze configures sFlow through YANG and ships no SFLOW-MIB, so those 13 bind a configuration interface Ze does not implement. Two ids are declared unsourced here, both stated at SHOULD level in an OBJECT-TYPE DESCRIPTION that the MUST-level scan does not raise. x-2 rests on the sFlowAgentAddress DESCRIPTION: 'In the case of a multi-homed agent, this should be the loopback address of the agent.', 'The address should be an invariant that does not change as interfaces are reconfigured, enabled, disabled, added or removed.' and 'A manager should be able to use the sFlowAgentAddress as a unique key that will identify this agent over extended periods of time so that a history can be maintained.'. x-5 rests on the sFlowRcvrMaximumDatagramSize DESCRIPTION, 'The maximum number of data bytes that can be sent in a single sample datagram. The manager should set this value to avoid fragmentation of the sFlow datagrams.', with DEFVAL { 1400 }. |
| `6.1` | not stated | 1 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | States why UDP was chosen ('The use of UDP reduces the amount of memory required to buffer data'). The word 'required' is part of a noun phrase describing buffering, and the sentence places no obligation on an implementation. | The use of UDP reduces the amount of memory required to buffer data. |
| `5:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds whoever publishes a new or changed sFlow structure format, as versioning discipline for the specification's own registry of structure numbers. Ze defines no sFlow structure of its own: internal/plugins/flowexport/sflow emits the structures this document already numbers. The producer would be an author of a new sFlow structure definition, a role Ze does not fill. | Any changes that would alter or invalidate fields in published structure definitions must be implemented using a new structure number. |
| `5:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the receiver of sFlow datagrams. Ze fills the sFlow AGENT role only: internal/plugins/flowexport/sflow encodes samples and sends datagrams to a configured collector, and no package decodes an inbound sFlow datagram. The producer would be an sFlow Collector, a receiving role Ze does not ship. | An application receiving sFlow must be prepared to accept additional reason codes. |
| `5:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The compact-encoding half of the same no-mixing rule, which the mapped row states in full. | If an agent will never use ifIndex numbers >= 2^24 then it must use compact encodings for all interfaces. |
| `5:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The expanded-encoding half of the same no-mixing rule, which the mapped row states in full. | Otherwise the expanded formats must be used for all interfaces. |
| `5:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The XDR comment repeats the compact-encoding rule the mapped row carries. | /* Compact Format Flow/Counter samples If ifIndex numbers are always < 2^24 then the compact must be used. */ |
| `5:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same sample_pool reset rule, repeated in the expanded flow sample definition. | Note: If the agent resets the sample_pool then it must also reset the sequence_number.*/ sflow_data_source_expanded source_id; /* sFlowDataSource */ unsigned int sampling_rate; /* sFlowPacketSamplingRate */ unsigned int sample_pool; /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */ unsigned int drops; /* Number of times that the sFlow agent detected that a packet marked to be sampled was dropped due to lack of resources. |
| `5:17` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same counter reset rule, repeated in the expanded counter sample definition. | /* Format of a single expanded counter sample */ /* opaque = sample_data; enterprise = 0; format = 4 */ struct counters_sample_expanded { unsigned int sequence_number; /* Incremented with each counter sample generated by this source_id Note: If the agent resets any of the counters then it must also reset the sequence_number. |
| `5:18` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same ifCounterDiscontinuityTime reset rule, repeated in the expanded counter sample definition. | In the case of ifIndex-based source_id's the sequence number must be reset each time ifCounterDiscontinuityTime changes. */ sflow_data_source_expanded source_id; /* sFlowDataSource */ counter_record counters<>; /* Counters polled for this source */ } |
| `5:20` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The constancy half of the same sFlowDataSource-to-sub-agent binding, which the mapped row states in full. | The association between sFlowDataSource and sub-agent must remain constant for the entire duration of an sFlow session. */ |
| `5:21` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the receiver of sFlow datagrams. Ze fills the sFlow AGENT role only: internal/plugins/flowexport/sflow encodes samples and sends datagrams to a configured collector, and no package decodes an inbound sFlow datagram. The producer would be an sFlow Collector, a receiving role Ze does not ship. | Note: While a sub-agents should try and track the global sysUptime value a receiver of sFlow packets must not assume that values are synchronised between sub-agents. */ sample_record samples<>; /* An array of sample records */ } |
| `5:22` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A relaxation, not an obligation: 'an sFlow Agent is not required to support all the different record types, only those applicable to its treatment of the particular packet being reporting on'. | However, an sFlow Agent is not required to support all the different record types, only those applicable to its treatment of the particular packet being reporting on. |
| `5:25` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the receiver of sFlow datagrams. Ze fills the sFlow AGENT role only: internal/plugins/flowexport/sflow encodes samples and sends datagrams to a configured collector, and no package decodes an inbound sFlow datagram. The producer would be an sFlow Collector, a receiving role Ze does not ship. | Applications receiving sFlow must be prepared to receive sampled_header structures with unknown sampled_header values. |
| `5:27` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The leading-encapsulation half of the same stripping rule, which the mapped row states in full. | Trailing encapsulation data corresponding to any leading encapsulations that were stripped must also be stripped. |
| `5:29` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The ambiguous-encapsulation half of the same stripping rule, which the mapped row states in full. | Outer encapsulations that are ambiguous, or not one of the standard header_protocol must be stripped. */ opaque header<>; /* Header bytes */ } |
| `8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes the property a generator is judged by ('The essential property of the random number generator is that the mean value of the numbers it generates converges to the required sampling rate'). The obligation it introduces is stated in the next sentence, which is classified at 8:2. | The essential property of the random number generator is that the mean value of the numbers it generates converges to the required sampling rate. |
| `8:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes when the algorithm needs a new skip value ('A new skip value is only required every time a sample is taken'). It grants a licence to compute lazily rather than imposing a duty. | A new skip value is only required every time a sample is taken. |
| `3.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates that a threshold-based sampler must still satisfy the equal-chance property the mapped row carries. | Calculation of an appropriate threshold value depends on the characteristics of the random number generator, however, the resulting sample stream must still satisfy (1). |
| `3.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes the counter-polling algorithm's piggyback condition; 'required Sampling Interval' is a noun phrase naming the configured interval, and the sentence states no obligation of its own. | Counters are only added to the datagram if the sources are within a short period, 5 seconds say, of failing to meet the required Sampling Interval (see sFlowCounterSamplingInterval in SFLOW MIB). |
| `4.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | The resulting translated MIB must be semantically equivalent, except where objects or events are omitted because no translation is possible (use of Counter64). |
| `4.2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the receiver of sFlow datagrams. Ze fills the sFlow AGENT role only: internal/plugins/flowexport/sflow encodes samples and sends datagrams to a configured collector, and no package decodes an inbound sFlow datagram. The producer would be an sFlow Collector, a receiving role Ze does not ship. | Before making any configuration changes, an sFlow Collector must first find a free row in the receiver table and then claim it by writing its owner string and a reservation time into a free row in the receiver table. |
| `4.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second sentence of the same sampler-independence rule, which the mapped row states in full. | Setting an attribute of one sampler must not alter the the behavior and settings of other sampler instances." SYNTAX Integer32 (1..65535) |
| `4.3:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | If non-zero the value must correspond to a valid, active sFlowRcvrIndex. |
| `4.3:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | If an entry in the sFlowRcvrTable expires, either because the sFlowRcvrOwner is set to the empty string or because the sFlowRcvrTimeout reaches zero, then the agent must mark all associated resources as available (by setting the associated SFlowReceiver entry to zero) and all values in these records must be restored to their default values. |
| `4.3:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | The version string must have the following structure: <MIB Version>;<Organization>;<Software Revision> where: <MIB Version> must be '1.3', the version of this MIB. <Organization> the name of the organization responsible for the agent implementation. <Revision> the specific software build of this agent. |
| `4.3:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the management entity that configures an sFlow agent over SNMP, not to the agent. Ze manages no other device's sFlow MIB: its own configuration is YANG (internal/plugins/flowexport/yang). The producer would be an SNMP management station reading the SFLOW-MIB, which Ze does not ship. | Management entities must check the MIB Version and not attempt to manage agents with MIB Versions greater than that for which they were designed. |
| `4.3:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | The sFlowAgent address must provide SNMP connectivity to the agent. |
| `4.3:9` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | An entity wishing to claim an sFlowRcvrTable entry must ensure that the entry is unclaimed before trying to claim it. |
| `4.3:10` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | The entry must be claimed before any changes can be made to other sampler objects. |
| `4.3:11` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | In order to avoid a race condition, the entity taking control of the sampler must set both the owner and a value for sFlowRcvrTimeout in the same SNMP set request. |
| `4.3:12` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | The agent must restore all other entities this row to their default values when the owner is set to unclaimed. |
| `4.3:13` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | It must also free all other resources associated with this sFlowRcvrTable entry. |
| `4.3:14` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | It must also free all other resources associated with this sFlowRcvrTable entry." DEFVAL { 0 } ::= { sFlowRcvrEntry 3 } |
| `4.3:15` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | When read, the agent must return the actual sampling rate it will be using (after the adjustments previously described). |
| `4.3:17` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | When read, the agent must return the actual sampling interval it will be using (after the adjustments previously described). |
| `4.3:18` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same convergence sentence, repeated in the counter-polling MIB object description. | The sampling algorithm must converge so that over time the number of packets sampled approaches 1/Nth of the total number of packets in the monitored flows." DEFVAL { 0 } ::= { sFlowCpEntry 4 } |
| `6.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduces the list of configuration arguments that follows ('The following arguments are required to configure sFlow sampling on an interface'). The arguments are named in the list, and no behavior is required by this sentence. | The following arguments are required to configure sFlow sampling on an interface. |

## Superseded

No document obsoletes SFLOW-V5, so its obligations are stated where they were written.
