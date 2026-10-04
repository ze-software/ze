# SFLOW-V5 - sFlow: A Method for Monitoring Traffic in Switched and Routed Networks

Experimental. Every requirement this repository extracted from SFLOW-V5, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 40.6% | 13 of 32 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 28.1% | 9 of 32 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 32 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 32 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 57 of 57 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 32 | of 43 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 32 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 3.1% | 1 of 32 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 3.1% | 1 of 32 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 3.1% | 1 of 32 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 21.9% | 7 of 32 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 26 | of 32 gated MUSTs judged | 3 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 32 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 43 |
| Gated MUST-level | 32 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 8 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 57 |
| Tagged units | 57 |
| Recorded audit verdicts | 26 |
| Discrimination records | 57 |
| Summary | `rfc/short/sflow-v5.md` |
| Requirement shard | `rfc/requirements/sflow-v5.md` |
| RFC text | `rfc/full/sflow-v5.txt` |

## Enrolment

Enrolled: sFlow Version 5 exporter/agent. The checklist includes transport, sample encoding, counter availability and sampling obligations from the 2026-09-21 extraction walk. Tests cover parts of these obligations; the checklist retains the remaining gaps and uncovered requirements.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- Flow export alongside NetFlow v9 and IPFIX. Unavailable counters carry the maximum-value sentinel. Counter and flow encoders share a datagram sequence
- all sources use expanded sample formats with full-width interface indexes. Counter generation changes restart source sample sequences. Ze runs no SNMP agent, so the agent address provides no SNMP connectivity ([`SFLOW-V5-4.3-1`](#sflow-v5-4.3-1), gap), and nothing checks that the configured agent address is a loopback address ([`SFLOW-V5-4.3-2`](#sflow-v5-4.3-2), gap). Other uncovered obligations remain in the checklist. Statistical sampling requirements SFLOW-V5-x-37 through x-41 remain unverified. Linux filter installation/readback tests cover configuration and lifecycle, not RNG range, packet-selection probability, one consideration per packet, independent sampling draws or long-term convergence. Current exporter and privileged kernel scenarios must be run before publishing verified coverage.


**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated (including scoped evidence) | 19 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **32** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`SFLOW-V5-x-9`](#sflow-v5-x-9), [`SFLOW-V5-x-12`](#sflow-v5-x-12), [`SFLOW-V5-x-14`](#sflow-v5-x-14), [`SFLOW-V5-x-25`](#sflow-v5-x-25), [`SFLOW-V5-x-26`](#sflow-v5-x-26), [`SFLOW-V5-x-27`](#sflow-v5-x-27), [`SFLOW-V5-x-28`](#sflow-v5-x-28), [`SFLOW-V5-x-29`](#sflow-v5-x-29), [`SFLOW-V5-x-30`](#sflow-v5-x-30), [`SFLOW-V5-x-31`](#sflow-v5-x-31), [`SFLOW-V5-x-32`](#sflow-v5-x-32), [`SFLOW-V5-x-33`](#sflow-v5-x-33), [`SFLOW-V5-x-34`](#sflow-v5-x-34)

**Annotated (including scoped evidence) (19):** [`SFLOW-V5-x-1`](#sflow-v5-x-1), [`SFLOW-V5-x-3`](#sflow-v5-x-3), [`SFLOW-V5-x-4`](#sflow-v5-x-4), [`SFLOW-V5-x-5`](#sflow-v5-x-5), [`SFLOW-V5-x-6`](#sflow-v5-x-6), [`SFLOW-V5-x-7`](#sflow-v5-x-7), [`SFLOW-V5-x-10`](#sflow-v5-x-10), [`SFLOW-V5-x-11`](#sflow-v5-x-11), [`SFLOW-V5-x-13`](#sflow-v5-x-13), [`SFLOW-V5-x-15`](#sflow-v5-x-15), [`SFLOW-V5-x-35`](#sflow-v5-x-35), [`SFLOW-V5-x-36`](#sflow-v5-x-36), [`SFLOW-V5-x-37`](#sflow-v5-x-37), [`SFLOW-V5-x-38`](#sflow-v5-x-38), [`SFLOW-V5-x-39`](#sflow-v5-x-39), [`SFLOW-V5-x-40`](#sflow-v5-x-40), [`SFLOW-V5-x-41`](#sflow-v5-x-41), [`SFLOW-V5-x-42`](#sflow-v5-x-42), [`SFLOW-V5-4.3-1`](#sflow-v5-4.3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `SFLOW-V5-x-1` | enum datagram_version { VERSION5 = 5 } union sample_datagram_type (datagram_version version) { case VERSION5: sample_datagram_v5 datagram; } (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowDatagramHeaderIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_encoder_test.go#L11). **negative:** no negative test. **{single-polarity}:** WriteDatagramHeader unconditionally writes the compile-time constant Version=5 into every datagram, so there is no other-version code path to reject (internal/plugins/flowexport/sflow/encoder.go:40, :14) |
| `SFLOW-V5-x-2` | The address should be an invariant that does not change as interfaces are reconfigured, enabled, disabled, added or removed. A manager should be able to use the sFlowAgentAddress as a unique key that will identify this agent over extended periods of time so that a history can be maintained. (§4.3) | SHOULD | 4.3 - Definitions | **positive:** `unit/verify` [`TestSFlowDatagramHeaderIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_encoder_test.go#L12). **positive:** `unit/verify` [`TestSFlowV5AgentAddressIsTheAgentKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_config_test.go#L56). **positive:** `unit/verify` [`TestSFlowV5AgentAddressSpellingsNameOneAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_config_test.go#L107). **negative:** `unit/verify` [`TestSFlowV5AgentAddressIsTheAgentKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_config_test.go#L60) |
| `SFLOW-V5-4.3-2` | In the case of a multi-homed agent, this should be the loopback address of the agent. (§4.3) | SHOULD | 4.3 - Definitions | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze writes the configured agent-address leaf into every datagram header and checks only that it is a specified address, the same on every sflow collector (internal/plugins/flowexport/sflow/register.go::validateSFlowCollectors). Nothing checks that the address belongs to a loopback interface; docs/guide/flow-export.md recommends one to the operator |
| `SFLOW-V5-x-3` | address agent_address /* IP address of sampling agent, sFlowAgentAddress. */ unsigned int sub_agent_id; /* Used to distinguishing between datagram streams from separate agent sub entities within an device. */ (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowDatagramHeaderIPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_encoder_test.go#L53). **negative:** no negative test. **{single-polarity}:** both agent_address and sub_agent_id are emitted in every datagram header by construction, and tuple uniqueness is an operator-config obligation (internal/plugins/flowexport/sflow/encoder.go:59, :43-57) |
| `SFLOW-V5-x-4` | Incremented with each sample datagram generated by a sub-agent within an agent. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowMultiInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_encoder_test.go#L93). **negative:** no negative test. **{single-polarity}:** each collector owns its UDP Sender sequence shared by counter and flow encoders, while per-source sample sequences belong to its encoders (internal/plugins/flowexport/sender.go Sender.Sequence, internal/plugins/flowexport/sflow/adapter.go CounterEncoder, internal/plugins/flowexport/sflow/flow_adapter.go FlowEncoder) |
| `SFLOW-V5-x-5` | The maximum number of data bytes that can be sent in a single sample datagram. The manager should set this value to avoid fragmentation of the sFlow datagrams. (§4.3) | MUST | 4.3 - Definitions | **positive:** `unit/verify` [`TestSFlowMultiInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_encoder_test.go#L94). **positive:** `unit/verify` [`TestSFlowV5ConfiguredMaxDatagramBoundsEveryDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_datagram_test.go#L43). **negative:** no negative test. **{single-polarity}:** encoders bound the payload, including padding, to the collector's max-datagram-size and Sender.Send refuses larger payloads; the operator must configure the bound for the path MTU (internal/plugins/flowexport/sender.go Sender.Send, internal/plugins/flowexport/sflow/encoder.go writeCounterDatagrams, internal/plugins/flowexport/sflow/flow_adapter.go EncodeFlowSample) |
| `SFLOW-V5-x-6` | The sFlow Agent may at most delay a sample by 1 second before it is required to send the datagram. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestExportFlowSampleDispatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_exporter_test.go#L99). **positive:** `unit/verify` [`TestSFlowV5FlowSampleSentWithinOneSecond`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_datagram_test.go#L85). **negative:** no negative test. **{single-polarity}:** counter and flow samples are encoded and sent synchronously with no buffering queue, so a sample is never held beyond a sub-millisecond encode and there is no holding timer to test negatively (internal/plugins/flowexport/exporter.go:204, internal/plugins/flowexport/sflow/adapter.go:47-51) |
| `SFLOW-V5-x-7` | The format of the sFlow datagram is specified using the XDR standard [32]. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowIfCounters`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counter_test.go#L35). **positive:** `unit/verify` [`TestSFlowV5CounterSampleWrapperIsXDR`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_xdr_test.go#L33). **positive:** `unit/verify` [`TestSFlowV5ExtendedGatewayIsXDR`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_xdr_test.go#L66). **positive:** `unit/verify` [`TestSFlowV5FlowDatagramIsXDR`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_flow_sampling_test.go#L77). **positive:** `unit/verify` [`TestSFlowV5FlowSampleWordsAreXDR`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_xdr_test.go#L50). **negative:** no negative test. **{single-polarity}:** every field is written via binary.BigEndian with 4-byte-aligned opaque padding, exporter-only with no decode path to reject a wrong endianness (internal/plugins/flowexport/sflow/counter.go:36, flow.go:123-128) |
| `SFLOW-V5-x-9` | unsigned int sequence_number; /* Incremented with each sample datagram generated by a sub-agent within an agent. */ (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowMixedDatagramSequenceWraps`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L203). **negative:** `unit/verify` [`TestSFlowCounterResetDoesNotResetDatagrams`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L240) |
| `SFLOW-V5-x-10` | unsigned int sampling_rate; /* sFlowPacketSamplingRate */ (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowFlowSample`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_flow_test.go#L9). **positive:** `unit/verify` [`TestSFlowV5SamplingRateIsTheSourceRate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_flow_sampling_test.go#L32). **negative:** no negative test. **{single-polarity}:** EncodeFlowSample writes the kernel-reported actual rate into every flow_sample, emitted unconditionally with no reject path (internal/plugins/flowexport/sflow/flow_adapter.go:78-79, flow.go:52) |
| `SFLOW-V5-x-11` | unsigned int sample_pool; /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */ (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowFlowSamplePoolTracksTotal`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go#L51). **negative:** no negative test. **{single-polarity}:** EncodeFlowSample computes sample_pool as the saturated product of cumulative samples and rate and writes it into every flow_sample, exporter-only with no negative form (internal/plugins/flowexport/sflow/flow_adapter.go:73-79, flow.go:56) |
| `SFLOW-V5-x-12` | If ifIndex numbers may be >= 2^24 then the expanded must be used. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5ExpandedEncodingForEveryInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L242). **negative:** `unit/verify` [`TestSFlowCounterSampleSourceIDOverflow`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counter_test.go#L216) |
| `SFLOW-V5-x-13` | Applications receiving sFlow data must always use the opaque length information when decoding opaque<> structures so that encountering extended structures will not cause decoding errors. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** skipping unknown formats on receive is a collector behavior; ze is an sFlow exporter only with no sFlow decode path, though it does emit the length prefixes that let a collector skip (internal/plugins/flowexport/sflow/counter.go:60-61, flow.go:131-132) |
| `SFLOW-V5-x-14` | The maximum number of seconds between successive samples of the counters associated with this data source. (§4.3) | MUST | 4.3 - Definitions | **positive:** `unit/verify` [`TestSFlowCounterPollAtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_exporter_test.go#L227). **positive:** `unit/verify` [`TestSFlowV5CounterGapNeverExceedsInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_polling_test.go#L83). **negative:** `unit/verify` [`TestSFlowV5CounterGapNeverExceedsInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_polling_test.go#L86) |
| `SFLOW-V5-x-15` | If counter samples are lost then new values will be sent during the next polling interval. The chance of an undetected counter wrap is negligible. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestInterfaceCountersFromCumulative`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_register_test.go#L128). **negative:** no negative test. **{single-polarity}:** interfaceCountersFrom copies the raw cumulative kernel counters straight through with no differencing, so exported if_counters are cumulative by construction (internal/plugins/flowexport/register.go:343-358, snapshot.go:10-13) |
| `SFLOW-V5-x-16` | The following values should be used for fields that are unknown (unless otherwise indicated in the structure definitions). - Unknown integer value. Use a value of 0 to indicate that a value is unknown. - Unknown counter. Use the maximum counter value to indicate that the counter is not available. (§5) | SHOULD | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5UnavailableCountersCarrySentinelEveryPoll`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_counters_test.go#L49). **positive:** `unit/verify` [`TestSFlowV5UnknownEgressInterfaceIsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_flow_sampling_test.go#L56). **negative:** `unit/verify` [`TestSFlowV5AvailableCountersNeverTurnUnavailable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_counters_test.go#L66) |
| `SFLOW-V5-x-25` | While the sFlow Datagram structure permits multiple samples to be included in each datagram, the sFlow Agent must not wait for a buffer to fill with samples before sending the sFlow Datagram. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5SampleSentWithoutWaitingForBuffer`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L147). **negative:** `unit/verify` [`TestSFlowV5NoDatagramHeldForMoreSamples`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L170) |
| `SFLOW-V5-x-26` | If counters must be sent in order to satisfy the maximum sampling interval then a datagram must be sent containing the outstanding counters. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5DueCountersAreSent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_polling_test.go#L113). **positive:** `unit/verify` [`TestSFlowV5OutstandingCountersAllSent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L192). **negative:** `unit/verify` [`TestSFlowV5DueCountersAreSent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_polling_test.go#L116). **negative:** `unit/verify` [`TestSFlowV5TailDatagramNotWithheld`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L215) |
| `SFLOW-V5-x-27` | An agent must not mix compact/expanded encodings. If an agent will never use ifIndex numbers >= 2^24 then it must use compact encodings for all interfaces. Otherwise the expanded formats must be used for all interfaces. (§5) | MUST NOT | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5ExpandedEncodingForEveryInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L241). **negative:** `unit/verify` [`TestSFlowV5CompactFormatsNeverMixedIn`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L278) |
| `SFLOW-V5-x-28` | unsigned int sequence_number; /* Incremented with each flow sample generated by this source_id. Note: If the agent resets the sample_pool then it must also reset the sequence_number.*/ (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5FlowSequencePerSourceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_sequence_test.go#L71). **positive:** `unit/verify` [`TestSFlowV5PoolAndSequenceResetTogether`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L309). **negative:** `unit/verify` [`TestSFlowV5PoolNeverResetsWithoutSequence`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L326) |
| `SFLOW-V5-x-29` | unsigned int sequence_number; /* Incremented with each counter sample generated by this source_id Note: If the agent resets any of the counters then it must also reset the sequence_number. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5SequenceResetOnAnyCounterGoingBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_sequence_test.go#L32). **positive:** `unit/verify` [`TestSFlowV5SequenceResetWithCounters`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L38). **negative:** `unit/verify` [`TestSFlowV5SequenceNeverResetWithoutDiscontinuity`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L57) |
| `SFLOW-V5-x-30` | In the case of ifIndex-based source_id's the sequence number must be reset each time ifCounterDiscontinuityTime changes. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5SequenceResetOnDiscontinuityTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_discontinuity_test.go#L15). **negative:** `unit/verify` [`TestSFlowV5SequenceKeptWhileDiscontinuityTimeSteady`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_discontinuity_test.go#L34) |
| `SFLOW-V5-x-31` | Each sFlowDataSource must be associated with only one sub-agent. The association between sFlowDataSource and sub-agent must remain constant for the entire duration of an sFlow session. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5DataSourceKeepsOneSubAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_subagent_test.go#L20). **positive:** `unit/verify` [`TestSFlowV5DataSourcesCarryTheirSubAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L351). **positive:** `unit/verify` [`TestSFlowV5OneSubAgentPerDataSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_config_test.go#L37). **negative:** `unit/verify` [`TestSFlowV5OneSubAgentPerDataSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_config_test.go#L40). **negative:** `unit/verify` [`TestSFlowV5SubAgentBindingConstantForSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L369) |
| `SFLOW-V5-x-32` | Within any given sFlow session a particular counter must be always available, or always unavailable. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5UnavailableCountersCarrySentinelEveryPoll`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_counters_test.go#L50). **negative:** `unit/verify` [`TestSFlowV5AvailableCountersNeverTurnUnavailable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_counters_test.go#L67) |
| `SFLOW-V5-x-33` | A flow_sample must contain packet header information. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5FlowSampleCarriesPacketHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L388). **negative:** `unit/verify` [`TestSFlowV5FlowSampleNeverWithoutHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L408) |
| `SFLOW-V5-x-34` | Any octets added to the frame_length to compensate for encapsulations removed by the underlying hardware must also be added to the stripped count. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** `unit/verify` [`TestSFlowV5FrameLengthAndStrippedIncludeFCS`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L89). **negative:** `unit/verify` [`TestSFlowV5FrameCompensationNeverUnstripped`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L105) |
| `SFLOW-V5-x-35` | Trailing encapsulation data corresponding to any leading encapsulations that were stripped must also be stripped. Trailing encapsulation data for the outermost protocol layer included in the sampled header must be stripped. In the case of a non-encapsulated 802.3 packet stripped >= 4 since VLAN tag information might have been stripped off in addition to the FCS. Outer encapsulations that are ambiguous, or not one of the standard header_protocol must be stripped. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux NIC driver and psample; internal/plugins/flowexport/sampling/tc_linux.go::buildSampleFilter installs act_sample on the ingress hook, the NIC removes the Ethernet FCS, the only trailer of the outermost layer, before the kernel builds the skb that act_sample copies, and internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample copies that header verbatim and strips no leading encapsulation, so no trailing encapsulation octets reach the sampled header |
| `SFLOW-V5-x-36` | extended_switch data must always be reported to describe the ingress/egress VLAN information for the packet. (§5) | MUST | 5 - sFlow Datagram Format | **positive:** no positive test. **negative:** no negative test. **{gap}:** the agent emits no extended_switch record; internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample writes one flow record per sample, the sampled_header, so ingress and egress VLAN information is never reported |
| `SFLOW-V5-x-37` | The random number generator must ensure that all numbers in the range between its maximum and minimum values of the distribution are possible; a random number generator only capable of generating even numbers, or numbers with any common divisor is unsuitable. (§B) | MUST | B - Appendix B | **positive:** no positive test. **negative:** no negative test. **{gap}:** Linux act_sample owns the random draw; the current source and kernel filter lifecycle test do not prove its output range |
| `SFLOW-V5-x-38` | The Packet Flow Sampling mechanism carried out by each sFlow Instance must ensure that any packet observed at a Data Source has an equal chance of being sampled, irrespective of the Packet Flow(s) to which it belongs. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** MatchAll filter configuration is not behavioral proof of equal packet-selection probability |
| `SFLOW-V5-x-39` | Each packet must only be considered once for sampling, irrespective of the number of ports it will be forwarded to. (§4.3) | MUST | 4.3 - Definitions | **positive:** no positive test. **negative:** no negative test. **{gap}:** ingress filter readback does not observe how often forwarded or replicated packets are considered by the sampler |
| `SFLOW-V5-x-40` | Note: Each sFlow sampler instance must operate independently of all other instances. Setting an attribute of one sampler must not alter the the behavior and settings of other sampler instances. (§4.3) | MUST | 4.3 - Definitions | **positive:** no positive test. **negative:** no negative test. **{gap}:** the kernel lifecycle test checks configuration isolation; independent sampling behavior remains unproven |
| `SFLOW-V5-x-41` | The sampling algorithm must converge so that over time the number of packets sampled approaches 1/Nth of the total number of packets in the monitored flows. (§4.3) | MUST | 4.3 - Definitions | **positive:** no positive test. **negative:** no negative test. **{gap}:** configuring an act_sample rate does not measure long-term convergence |
| `SFLOW-V5-x-42` | The Agent may implement an automated one-way backoff of the Sampling Rate that triggers whenever an excessive number of samples per second is generated. When the triggered the Agent can double the Sampling Rate. If the threshold is exceeded again, the Sampling Rate is doubled again. The Sampling Rate will quickly reach a value that is sustainable. The sampling rate must stay at its new value and never automatically return to the originally configured value. (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The Agent may implement an automated one-way backoff of the Sampling Rate that triggers whenever an excessive number of samples per second is generated."; ze implements no automatic backoff, so the rate never changes by itself. internal/plugins/flowexport/sampling/tc_linux.go::SetupSampling installs the configured rate into act_sample and nothing else writes it |
| `SFLOW-V5-4.3-1` | The sFlowAgent address must provide SNMP connectivity to the agent. (§4.3) | MUST | 4.3 - Definitions | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze runs no SNMP agent, so no address provides SNMP connectivity to it. Ze configures sFlow through YANG (internal/plugins/flowexport/yang) and writes the configured agent-address into every datagram header, but nothing answers SNMP at that address |
| `SFLOW-V5-x-17` | The maximum number of data bytes that can be sent in a single sample datagram. The manager should set this value to avoid fragmentation of the sFlow datagrams." DEFVAL { 1400 } (§4.3) | SHOULD | 4.3 - Definitions | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-18` | The maximum number of bytes that should be copied from a sampled packet. The agent may have an internal maximum and minimum permissible sizes. If an attempt is made to set this value outside the permissible range then the agent should adjust the value to the closest permissible value." DEFVAL { 128 } (§4.3) | SHOULD | 4.3 - Definitions | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-19` | If the sFlow Agent chooses to regularly schedule counter sampling, then it should schedule each counter source at a different start time (preferably randomly) so that counter sampling is not synchronised within an agent or between agents. (§3.2) | SHOULD | 3.2 - Counter Sampling | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-20` | When a sample is taken, the counter indicating how many packets to skip before taking the next sample should be reset. The value of the counter should be set to a random integer where the sequence of random integers used over time should be such that: (1) Total_Packets/Total_Samples = Sampling Rate (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-21` | The assigned port for sFlow (and the default specified in the SFLOW MIB) is port 6343. All sFlow Agents and applications should default to using UDP port 6343. (§5) | SHOULD | 5 - sFlow Datagram Format | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-22` | Each Data Source may be capable of supporting more than one independent sampling process, in which case there will be multiple sFlow Instances associated with each Data Source. (§4.2.2) | MAY | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-23` | Since Packet Flow Sampling will cause a steady, but random, stream of sFlow Datagrams to be sent to the sFlow Collector, counter samples may be taken opportunistically in order to fill these datagrams. (§3.2) | MAY | 3.2 - Counter Sampling | **positive:** no positive test. **negative:** no negative test |
| `SFLOW-V5-x-24` | When the sampling rate is set the agent is free to adjust the value so that it lies between the maximum and minimum values and has the closest achievable value. When read, the agent must return the actual sampling rate it will be using (after the adjustments previously described). (§4.3) | MAY | 4.3 - Definitions | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`SFLOW-V5-4.3-2`](#sflow-v5-4.3-2) In the case of a multi-homed agent, this should be the loopback address of the agent. (§4.3) | {gap} | Ze writes the configured agent-address leaf into every datagram header and checks only that it is a specified address, the same on every sflow collector (internal/plugins/flowexport/sflow/register.go::validateSFlowCollectors). Nothing checks that the address belongs to a loopback interface; docs/guide/flow-export.md recommends one to the operator |
| [`SFLOW-V5-x-13`](#sflow-v5-x-13) Applications receiving sFlow data must always use the opaque length information when decoding opaque<> structures so that encountering extended structures will not cause decoding errors. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: skipping unknown formats on receive is a collector behavior; ze is an sFlow exporter only with no sFlow decode path, though it does emit the length prefixes that let a collector skip (internal/plugins/flowexport/sflow/counter.go:60-61, flow.go:131-132) |
| [`SFLOW-V5-x-35`](#sflow-v5-x-35) Trailing encapsulation data corresponding to any leading encapsulations that were stripped must also be stripped. Trailing encapsulation data for the outermost protocol layer included in the sampled header must be stripped. In the case of a non-encapsulated 802.3 packet stripped >= 4 since VLAN tag information might have been stripped off in addition to the FCS. Outer encapsulations that are ambiguous, or not one of the standard header_protocol must be stripped. (§5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux NIC driver and psample; internal/plugins/flowexport/sampling/tc_linux.go::buildSampleFilter installs act_sample on the ingress hook, the NIC removes the Ethernet FCS, the only trailer of the outermost layer, before the kernel builds the skb that act_sample copies, and internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample copies that header verbatim and strips no leading encapsulation, so no trailing encapsulation octets reach the sampled header |
| [`SFLOW-V5-x-36`](#sflow-v5-x-36) extended_switch data must always be reported to describe the ingress/egress VLAN information for the packet. (§5) | {gap}, no test | the agent emits no extended_switch record; internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample writes one flow record per sample, the sampled_header, so ingress and egress VLAN information is never reported |
| [`SFLOW-V5-x-37`](#sflow-v5-x-37) The random number generator must ensure that all numbers in the range between its maximum and minimum values of the distribution are possible; a random number generator only capable of generating even numbers, or numbers with any common divisor is unsuitable. (§B) | {gap}, no test | Linux act_sample owns the random draw; the current source and kernel filter lifecycle test do not prove its output range |
| [`SFLOW-V5-x-38`](#sflow-v5-x-38) The Packet Flow Sampling mechanism carried out by each sFlow Instance must ensure that any packet observed at a Data Source has an equal chance of being sampled, irrespective of the Packet Flow(s) to which it belongs. (§3.1) | {gap}, no test | MatchAll filter configuration is not behavioral proof of equal packet-selection probability |
| [`SFLOW-V5-x-39`](#sflow-v5-x-39) Each packet must only be considered once for sampling, irrespective of the number of ports it will be forwarded to. (§4.3) | {gap}, no test | ingress filter readback does not observe how often forwarded or replicated packets are considered by the sampler |
| [`SFLOW-V5-x-40`](#sflow-v5-x-40) Note: Each sFlow sampler instance must operate independently of all other instances. Setting an attribute of one sampler must not alter the the behavior and settings of other sampler instances. (§4.3) | {gap}, no test | the kernel lifecycle test checks configuration isolation; independent sampling behavior remains unproven |
| [`SFLOW-V5-x-41`](#sflow-v5-x-41) The sampling algorithm must converge so that over time the number of packets sampled approaches 1/Nth of the total number of packets in the monitored flows. (§4.3) | {gap}, no test | configuring an act_sample rate does not measure long-term convergence |
| [`SFLOW-V5-x-42`](#sflow-v5-x-42) The Agent may implement an automated one-way backoff of the Sampling Rate that triggers whenever an excessive number of samples per second is generated. When the triggered the Agent can double the Sampling Rate. If the threshold is exceeded again, the Sampling Rate is doubled again. The Sampling Rate will quickly reach a value that is sustainable. The sampling rate must stay at its new value and never automatically return to the originally configured value. (§4.2.2) | no test | no test carries this requirement id; annotated {feature-declined}: "The Agent may implement an automated one-way backoff of the Sampling Rate that triggers whenever an excessive number of samples per second is generated."; ze implements no automatic backoff, so the rate never changes by itself. internal/plugins/flowexport/sampling/tc_linux.go::SetupSampling installs the configured rate into act_sample and nothing else writes it |
| [`SFLOW-V5-4.3-1`](#sflow-v5-4.3-1) The sFlowAgent address must provide SNMP connectivity to the agent. (§4.3) | {gap}, no test | Ze runs no SNMP agent, so no address provides SNMP connectivity to it. Ze configures sFlow through YANG (internal/plugins/flowexport/yang) and writes the configured agent-address into every datagram header, but nothing answers SNMP at that address |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`SFLOW-V5-x-1`](#sflow-v5-x-1)

enum datagram_version { VERSION5 = 5 } union sample_datagram_type (datagram_version version) { case VERSION5: sample_datagram_v5 datagram; } (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity: the header's first field decodes big-endian to 5, the only datagram_version the XDR union defines; WriteDatagramHeader writes a constant, so any other value fails the assertion

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowDatagramHeaderIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_encoder_test.go#L11) | unit/verify | revert, verified |

### [`SFLOW-V5-x-2`](#sflow-v5-x-2)

The address should be an invariant that does not change as interfaces are reconfigured, enabled, disabled, added or removed. A manager should be able to use the sFlowAgentAddress as a unique key that will identify this agent over extended periods of time so that a history can be maintained. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Row narrowed to its last two sentences (invariance, unique key); SNMP and loopback moved to gap rows SFLOW-V5-4.3-1/4.3-2. Invariance: TestSFlowV5AgentAddressIsTheAgentKey drives the factory-built counter encoder over three interface sets and requires 192.0.2.1 in every header; TestSFlowDatagramHeaderIPv4 pins the header bytes. No code path derives the address from interfaces, so invariance has no reject path (positive only). Unique key: Config.Validate refuses a missing, 0.0.0.0, :: or second agent address, naming agent-address (parseAgentAddress, differentAgents); TestSFlowV5AgentAddressSpellingsNameOneAgent accepts two spellings of 2001:db8::1 and refuses ::1 vs ::2, which fails if the compare returns to strings (D-8 fixed). All units recorded (revert).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5AgentAddressIsTheAgentKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_config_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestSFlowV5AgentAddressIsTheAgentKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_config_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestSFlowV5AgentAddressSpellingsNameOneAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_config_test.go#L107) | unit/verify | revert, verified |
| positive | [`TestSFlowDatagramHeaderIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_encoder_test.go#L12) | unit/verify | revert, verified |

### [`SFLOW-V5-4.3-2`](#sflow-v5-4.3-2)

In the case of a multi-homed agent, this should be the loopback address of the agent. (§4.3)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. SHOULD: on a multi-homed agent the address should be the loopback address. validateSFlowCollectors checks a specified address, equal on every sflow collector, and nothing checks it belongs to a loopback interface; only docs/guide/flow-export.md recommends one. {gap} annotation and Meta Support coverage disclose it.

No test carries SFLOW-V5-4.3-2, so no unit is bound to it.

### [`SFLOW-V5-x-3`](#sflow-v5-x-3)

address agent_address /* IP address of sampling agent, sFlowAgentAddress. */ unsigned int sub_agent_id; /* Used to distinguishing between datagram streams from separate agent sub entities within an device. */ (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity: agent_address and sub_agent_id 7 both decode to the distinct values written, so the pair the header carries names the sub-agent's datagram stream

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowDatagramHeaderIPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_encoder_test.go#L53) | unit/verify | revert, verified |

### [`SFLOW-V5-x-4`](#sflow-v5-x-4)

Incremented with each sample datagram generated by a sub-agent within an agent. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity (marker: the exporter holds no path that emits a datagram without advancing): writeCounterDatagrams' two datagrams carry sequence 1 and 2 and return 3 as the next, so a sender that did not increment per datagram fails seq2 != 2 and nextSeq != 3

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowMultiInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_encoder_test.go#L93) | unit/verify | revert, verified |

### [`SFLOW-V5-x-5`](#sflow-v5-x-5)

The maximum number of data bytes that can be sent in a single sample datagram. The manager should set this value to avoid fragmentation of the sFlow datagrams. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity row. TestSFlowV5ConfiguredMaxDatagramBoundsEveryDatagram sets a non-default 600-octet bound on a real Sender, requires every counter datagram for 30 interfaces to be <= 600 octets while together carrying all 30 samples (num_samples summed on the collector socket), requires a flow sample with a 1400-octet header to arrive in <= 600 octets, and requires Sender.Send to refuse 601 octets; the older default-bound unit TestSFlowMultiInterface stays. An encoder bounding by the default instead of the configured size goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowV5ConfiguredMaxDatagramBoundsEveryDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_datagram_test.go#L43) | unit/verify | revert, verified |
| positive | [`TestSFlowMultiInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_encoder_test.go#L94) | unit/verify | revert, verified |

### [`SFLOW-V5-x-6`](#sflow-v5-x-6)

The sFlow Agent may at most delay a sample by 1 second before it is required to send the datagram. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity row. TestSFlowV5FlowSampleSentWithinOneSecond hands a sample to the real sflow FlowEncoder with a real Sender and requires the datagram on the collector socket within one second of the call, so an encoder holding a sample longer than 1 s goes red while one flushing sooner passes, matching the quote; TestExportFlowSampleDispatch proves exportFlowSample dispatches synchronously. Counter samples are sent synchronously in the poll (x-26 units).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowV5FlowSampleSentWithinOneSecond`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_datagram_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestExportFlowSampleDispatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_exporter_test.go#L99) | unit/verify | revert, verified |

### [`SFLOW-V5-x-7`](#sflow-v5-x-7)

The format of the sFlow datagram is specified using the XDR standard [32]. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity row, every sflow writer now compared as XDR words with distinct-octet values: datagram header and sampled_header padding (TestSFlowV5FlowDatagramIsXDR), counters_sample wrapper (TestSFlowV5CounterSampleWrapperIsXDR), all flow_sample words (TestSFlowV5FlowSampleWordsAreXDR), extended_gateway IPv4 and IPv6 unions and variable-length arrays (TestSFlowV5ExtendedGatewayIsXDR), if_counters (TestSFlowIfCounters). A little-endian, reordered or unpadded write changes compared bytes. Every unit recorded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowIfCounters`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counter_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestSFlowV5FlowDatagramIsXDR`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_flow_sampling_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestSFlowV5CounterSampleWrapperIsXDR`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_xdr_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestSFlowV5ExtendedGatewayIsXDR`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_xdr_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestSFlowV5FlowSampleWordsAreXDR`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_xdr_test.go#L50) | unit/verify | revert, verified |

### [`SFLOW-V5-x-9`](#sflow-v5-x-9)

unsigned int sequence_number; /* Incremented with each sample datagram generated by a sub-agent within an agent. */ (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: counter and flow datagrams of one sender share one datagram sequence that wraps through 0xFFFFFFFF; negative: a counter generation reset restarts only the source sample sequence, never the datagram sequence

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowCounterResetDoesNotResetDatagrams`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L240) | unit/verify | revert, verified |
| positive | [`TestSFlowMixedDatagramSequenceWraps`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L203) | unit/verify | revert, verified |

### [`SFLOW-V5-x-10`](#sflow-v5-x-10)

unsigned int sampling_rate; /* sFlowPacketSamplingRate */ (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity row. TestSFlowV5SamplingRateIsTheSourceRate drives the real EncodeFlowSample with two sources at 74565 and 1000 and requires each datagram's sampling_rate to equal its source's rate, so a constant, a stale rate or a literal goes red (recorded). sampling_worker.go fills FlowSample.Rate from the psample packet's rate.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowV5SamplingRateIsTheSourceRate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_flow_sampling_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestSFlowFlowSample`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_flow_test.go#L9) | unit/verify | revert, verified |

### [`SFLOW-V5-x-11`](#sflow-v5-x-11)

unsigned int sample_pool; /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */ (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts the code's estimate sample_pool = min(seq*rate, MaxUint32), not the RFC's 'Total number of packets that could have been sampled'; no packet count the data source saw is observed, and the saturation subtest asserts behaviour the RFC does not state (an XDR unsigned counter wraps; a saturated pool stops advancing)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSFlowFlowSamplePoolTracksTotal`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go#L51) | unit/verify | revert, verified |

### [`SFLOW-V5-x-12`](#sflow-v5-x-12)

If ifIndex numbers may be >= 2^24 then the expanded must be used. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: ifIndex 0x01000000 and 0xFFFFFFFF decode unchanged from expanded counter format 4 and flow format 3; negative: indexes at and above 2^24 never alias to their low 24 bits in source_id_index or if_counters.ifIndex

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowCounterSampleSourceIDOverflow`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counter_test.go#L216) | unit/verify | revert, verified |
| positive | [`TestSFlowV5ExpandedEncodingForEveryInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L242) | unit/verify | revert, verified |

### [`SFLOW-V5-x-13`](#sflow-v5-x-13)

Applications receiving sFlow data must always use the opaque length information when decoding opaque<> structures so that encountering extended structures will not cause decoding errors. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-13, so no unit is bound to it.

### [`SFLOW-V5-x-14`](#sflow-v5-x-14)

The maximum number of seconds between successive samples of the counters associated with this data source. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. notifySnapshot now polls at elapsed >= interval - one snapshot tick and waits for the mutex instead of TryLock-dropping. TestSFlowV5CounterGapNeverExceedsInterval drives 30 late 1.01 s snapshots at a 5 s interval through a real Sender and requires no gap over 5 s and at least 6 polls; the old elapsed >= interval gate leaves a 5.05 s gap and goes red (author observed red at HEAD). Its two tags sit on one assertion: the maximum is one upper bound, so this is effectively single-polarity, not a pair. TestSFlowCounterPollAtInterval covers the 1 s interval. The busy-exporter drop is covered under x-26.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5CounterGapNeverExceedsInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_polling_test.go#L86) | unit/verify | revert, verified |
| positive | [`TestSFlowCounterPollAtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_exporter_test.go#L227) | unit/verify | revert, verified |
| positive | [`TestSFlowV5CounterGapNeverExceedsInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_polling_test.go#L83) | unit/verify | revert, verified |

### [`SFLOW-V5-x-15`](#sflow-v5-x-15)

If counter samples are lost then new values will be sent during the next polling interval. The chance of an undetected counter wrap is negligible. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity (marker: interfaceCountersFrom has no differencing path): a second conversion of the same kernel stats must equal the first, so a delta implementation (which loses the traffic of a lost sample) fails 'second conversion differs'; the negligible-wrap clause rests on 64-bit octet counters, and RxBytes 5e9 > 2^32 must come through untruncated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterfaceCountersFromCumulative`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_register_test.go#L128) | unit/verify | revert, verified |

### [`SFLOW-V5-x-16`](#sflow-v5-x-16)

The following values should be used for fields that are unknown (unless otherwise indicated in the structure definitions). - Unknown integer value. Use a value of 0 to indicate that a value is unknown. - Unknown counter. Use the maximum counter value to indicate that the counter is not available. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Integer clause: TestSFlowV5UnknownEgressInterfaceIsZero requires output format 0 and value 0 for an unknown egress. Counter clause: TestSFlowV5UnavailableCountersCarrySentinelEveryPoll requires exactly 0xFFFFFFFF for the four counters the kernel does not expose, and TestSFlowV5AvailableCountersNeverTurnUnavailable requires real values (including zero) for the rest. All three units now carry observed-red records.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5AvailableCountersNeverTurnUnavailable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_counters_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestSFlowV5UnknownEgressInterfaceIsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_flow_sampling_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestSFlowV5UnavailableCountersCarrySentinelEveryPoll`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_counters_test.go#L49) | unit/verify | revert, verified |

### [`SFLOW-V5-x-25`](#sflow-v5-x-25)

While the sFlow Datagram structure permits multiple samples to be included in each datagram, the sFlow Agent must not wait for a buffer to fill with samples before sending the sFlow Datagram. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: one sample yields exactly one datagram sent at once; negative: three samples yield three datagrams, the sender count advancing by one each, and no datagram carries more than one flow_sample

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5NoDatagramHeldForMoreSamples`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L170) | unit/verify | revert, verified |
| positive | [`TestSFlowV5SampleSentWithoutWaitingForBuffer`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L147) | unit/verify | revert, verified |

### [`SFLOW-V5-x-26`](#sflow-v5-x-26)

If counters must be sent in order to satisfy the maximum sampling interval then a datagram must be sent containing the outstanding counters. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Chain now whole. TestSFlowV5DueCountersAreSent requires every due poll over 30 late ticks to send a datagram listing all three interfaces, and requires a due snapshot that meets a held exporter mutex still to send them (the old TryLock drop goes red). The real encoder's part: writeCounterDatagrams units require all 30 sources emitted with the partial tail not held back, and TestSFlowV5ConfiguredMaxDatagramBoundsEveryDatagram counts all 30 samples on the collector socket through CounterEncoder.Encode. Trigger (x-14) now enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5TailDatagramNotWithheld`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L215) | unit/verify | revert, verified |
| negative | [`TestSFlowV5DueCountersAreSent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_polling_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestSFlowV5OutstandingCountersAllSent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L192) | unit/verify | revert, verified |
| positive | [`TestSFlowV5DueCountersAreSent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_polling_test.go#L113) | unit/verify | revert, verified |

### [`SFLOW-V5-x-27`](#sflow-v5-x-27)

An agent must not mix compact/expanded encodings. If an agent will never use ifIndex numbers >= 2^24 then it must use compact encodings for all interfaces. Otherwise the expanded formats must be used for all interfaces. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Ze cannot promise its ifIndex stays below 2^24 (Linux ifindex is a full int), so it falls in the quote's 'Otherwise' branch and the compact-for-all clause does not bind it; mixing: a compact format 1 or 2 among the six records fails 'compact data_format emitted'; expanded for all: alternating low and full-width ifIndex values fail 'want expanded 4' and 'want expanded 3' if any interface drops to compact

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5CompactFormatsNeverMixedIn`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L278) | unit/verify | revert, verified |
| positive | [`TestSFlowV5ExpandedEncodingForEveryInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L241) | unit/verify | revert, verified |

### [`SFLOW-V5-x-28`](#sflow-v5-x-28)

unsigned int sequence_number; /* Incremented with each flow sample generated by this source_id. Note: If the agent resets the sample_pool then it must also reset the sequence_number.*/ (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Per-source clause: TestSFlowV5FlowSequencePerSourceID interleaves sources 3 and 4 and requires 1,2,3 and 1,2, so one sequence shared across source_ids goes red (recorded). Reset clause: TestSFlowV5PoolAndSequenceResetTogether and TestSFlowV5PoolNeverResetsWithoutSequence (both recorded).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5PoolNeverResetsWithoutSequence`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L326) | unit/verify | revert, verified |
| positive | [`TestSFlowV5FlowSequencePerSourceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_sequence_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestSFlowV5PoolAndSequenceResetTogether`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L309) | unit/verify | revert, verified |

### [`SFLOW-V5-x-29`](#sflow-v5-x-29)

unsigned int sequence_number; /* Incremented with each counter sample generated by this source_id Note: If the agent resets any of the counters then it must also reset the sequence_number. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Reset clause: TestSFlowV5SequenceResetOnAnyCounterGoingBack rewinds each of 13 cumulative counters and requires sequence 1; TestSFlowV5SequenceResetWithCounters requires source 3's reset on ifInOctets going back. Increment and no-spurious-reset: TestSFlowV5SequenceNeverResetWithoutDiscontinuity requires 1,2,3 while counters rise. All units now recorded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5SequenceNeverResetWithoutDiscontinuity`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestSFlowV5SequenceResetWithCounters`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestSFlowV5SequenceResetOnAnyCounterGoingBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_sequence_test.go#L32) | unit/verify | revert, verified |

### [`SFLOW-V5-x-30`](#sflow-v5-x-30)

In the case of ifIndex-based source_id's the sequence number must be reset each time ifCounterDiscontinuityTime changes. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: a CounterGeneration change with rising counters resets the sequence to 1; negative: an unchanged generation keeps 1,2,3 even when a counter reads lower

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5SequenceKeptWhileDiscontinuityTimeSteady`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_discontinuity_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestSFlowV5SequenceResetOnDiscontinuityTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_discontinuity_test.go#L15) | unit/verify | revert, verified |

### [`SFLOW-V5-x-31`](#sflow-v5-x-31)

Each sFlowDataSource must be associated with only one sub-agent. The association between sFlowDataSource and sub-agent must remain constant for the entire duration of an sFlow session. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Only-one-sub-agent: TestSFlowV5OneSubAgentPerDataSource requires Config.Validate to accept two sflow collectors with sub-agent-id 7 and to refuse 7 and 8, naming sub-agent-id (validateSFlowCollectors; every sflow collector exports every data source); TestSFlowV5DataSourceKeepsOneSubAgent shows one collector's counter and flow factories use its single sub-agent. Constant-association: the sflow_v5_test.go negative keeps ifIndex 3 on one sub-agent over five datagrams. All units recorded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5OneSubAgentPerDataSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_config_test.go#L40) | unit/verify | revert, verified |
| negative | [`TestSFlowV5SubAgentBindingConstantForSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L369) | unit/verify | revert, verified |
| positive | [`TestSFlowV5OneSubAgentPerDataSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_config_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestSFlowV5DataSourceKeepsOneSubAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_subagent_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestSFlowV5DataSourcesCarryTheirSubAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L351) | unit/verify | revert, verified |

### [`SFLOW-V5-x-32`](#sflow-v5-x-32)

Within any given sFlow session a particular counter must be always available, or always unavailable. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: every counter unavailable on the first poll is still the sentinel on a poll with different statistics; negative: available counters stay available on a later zero-valued poll

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5AvailableCountersNeverTurnUnavailable`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_counters_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestSFlowV5UnavailableCountersCarrySentinelEveryPoll`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow_v5_counters_test.go#L50) | unit/verify | revert, verified |

### [`SFLOW-V5-x-33`](#sflow-v5-x-33)

A flow_sample must contain packet header information. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: the flow_sample carries one sampled_header record with the 14 captured bytes; negative: a 1500-byte capture that cannot fit is truncated but still carried as a non-empty sampled_header prefix

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5FlowSampleNeverWithoutHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L408) | unit/verify | revert, verified |
| positive | [`TestSFlowV5FlowSampleCarriesPacketHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_test.go#L388) | unit/verify | revert, verified |

### [`SFLOW-V5-x-34`](#sflow-v5-x-34)

Any octets added to the frame_length to compensate for encapsulations removed by the underlying hardware must also be added to the stripped count. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: a 60-octet skb gives frame_length 64 and stripped 4, the FCS added to both; negative: over 64, 1500 and 9000 octets frame_length minus stripped is always the skb length

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSFlowV5FrameCompensationNeverUnstripped`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestSFlowV5FrameLengthAndStrippedIncludeFCS`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/sflow/sflow_v5_counters_test.go#L89) | unit/verify | revert, verified |

### [`SFLOW-V5-x-35`](#sflow-v5-x-35)

Trailing encapsulation data corresponding to any leading encapsulations that were stripped must also be stripped. Trailing encapsulation data for the outermost protocol layer included in the sampled header must be stripped. In the case of a non-encapsulated 802.3 packet stripped >= 4 since VLAN tag information might have been stripped off in addition to the FCS. Outer encapsulations that are ambiguous, or not one of the standard header_protocol must be stripped. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-35, so no unit is bound to it.

### [`SFLOW-V5-x-36`](#sflow-v5-x-36)

extended_switch data must always be reported to describe the ingress/egress VLAN information for the packet. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-36, so no unit is bound to it.

### [`SFLOW-V5-x-37`](#sflow-v5-x-37)

The random number generator must ensure that all numbers in the range between its maximum and minimum values of the distribution are possible; a random number generator only capable of generating even numbers, or numbers with any common divisor is unsuitable. (§B)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-37, so no unit is bound to it.

### [`SFLOW-V5-x-38`](#sflow-v5-x-38)

The Packet Flow Sampling mechanism carried out by each sFlow Instance must ensure that any packet observed at a Data Source has an equal chance of being sampled, irrespective of the Packet Flow(s) to which it belongs. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-38, so no unit is bound to it.

### [`SFLOW-V5-x-39`](#sflow-v5-x-39)

Each packet must only be considered once for sampling, irrespective of the number of ports it will be forwarded to. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-39, so no unit is bound to it.

### [`SFLOW-V5-x-40`](#sflow-v5-x-40)

Note: Each sFlow sampler instance must operate independently of all other instances. Setting an attribute of one sampler must not alter the the behavior and settings of other sampler instances. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-40, so no unit is bound to it.

### [`SFLOW-V5-x-41`](#sflow-v5-x-41)

The sampling algorithm must converge so that over time the number of packets sampled approaches 1/Nth of the total number of packets in the monitored flows. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-41, so no unit is bound to it.

### [`SFLOW-V5-x-42`](#sflow-v5-x-42)

The Agent may implement an automated one-way backoff of the Sampling Rate that triggers whenever an excessive number of samples per second is generated. When the triggered the Agent can double the Sampling Rate. If the threshold is exceeded again, the Sampling Rate is doubled again. The Sampling Rate will quickly reach a value that is sustainable. The sampling rate must stay at its new value and never automatically return to the originally configured value. (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries SFLOW-V5-x-42, so no unit is bound to it.

### [`SFLOW-V5-4.3-1`](#sflow-v5-4.3-1)

The sFlowAgent address must provide SNMP connectivity to the agent. (§4.3)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Unconditional MUST of the sFlowAgentAddress DESCRIPTION (4.3), binding the sFlow agent Ze is. parseAgentAddress checks only that the configured agent-address parses and is specified; no Go code in the tree implements an SNMP agent (grep: snmp appears only in ifIndex/ifTable comments), so nothing answers SNMP at the address. {gap} annotation and Meta Support coverage disclose it.

No test carries SFLOW-V5-4.3-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | prose |
| Source | rfc/full/sflow-v5.txt |
| Source fingerprint | cd9e27ebcba6e68b |
| Record | rfc/extraction/sflow-v5.json |
| Mapped sentences | 22 |
| Declined as scope | 36 |
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
| `5` | sFlow Datagram Format | 30 | walked | sFlow Datagram Format. The specification's main normative section: 30 sites, 15 mapped to SFLOW-V5-x-6, x-12, x-13 and x-25 through x-36, and 15 excluded, of which 9 repeat a sentence already mapped, 4 bind another role (three an sFlow Collector, one the author of a new sFlow structure definition) and 2 state no obligation. Nine ids are declared unsourced here. sFlow v5 writes this section as XDR type definitions and explanatory prose, so the capitalised MUST-level scan raises no site for any of them. x-1 rests on 'enum datagram_version { VERSION5 = 5 }' with 'union sample_datagram_type (datagram_version version) { case VERSION5: sample_datagram_v5 datagram; }', which admit one version value and name no obligation. x-3 rests on the sub_agent_id comment, 'Used to distinguishing between datagram streams from separate agent sub entities within an device', read with the agent_address comment 'IP address of sampling agent, sFlowAgentAddress'. x-4 rests on the sample_datagram_v5 sequence_number comment, 'Incremented with each sample datagram generated by a sub-agent within an agent'. x-7 rests on 'The format of the sFlow datagram is specified using the XDR standard [32]', which imports 4-byte alignment and big-endian order from XDR instead of restating them. x-9 rests on the 'unsigned int sequence_number' declarations, one per-agent in sample_datagram_v5 and one per-source in flow_sample and counters_sample, each an XDR unsigned 32-bit value that wraps. x-10 rests on 'unsigned int sampling_rate; /* sFlowPacketSamplingRate */' inside flow_sample. x-11 rests on 'unsigned int sample_pool; /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */'. x-15 rests on 'If counter samples are lost then new values will be sent during the next polling interval. The chance of an undetected counter wrap is negligible.', which holds for counters that accumulate and not for per-interval deltas, read with '/* Generic Interface Counters - see RFC 2233 */' and with 'An available counter may temporarily have the max value just before it rolls to zero.'. x-16 rests on 'Unknown counter. Use the maximum counter value to indicate that the counter is not available.' |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | Counter Sampling | 1 | walked | Counter Sampling. One site, excluded as not a requirement: 'Counters are only added to the datagram if the sources are within a short period, 5 seconds say, of failing to meet the required Sampling Interval', which describes one strategy and prescribes none. One id is declared unsourced here. x-14 rests on 'A maximum Sampling Interval is assigned to each sFlow Instance associated with an interface Data Source, but the sFlow Agent is free to schedule polling in order maximize internal efficiency', read with 'Periodically, say every second, the sFlow Agent examines the list of counter sources and sends any counters that need to be sent to meet the sampling interval requirement'. Both sentences are indicative, so the MUST-level scan raises no site for them. |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 1 | walked | not stated |
| `4.2.2` | not stated | 1 | walked | not stated |
| `4.2.3` | not stated | 0 | walked | not stated |
| `4.3` | Definitions | 18 | walked | Definitions. The SFLOW-MIB module itself: 18 sites, 4 mapped to SFLOW-V5-x-39, x-40, x-41 and SFLOW-V5-4.3-1, and 14 excluded, of which 2 repeat a sentence already mapped and 12 bind an SNMP agent or an SNMP management station. Ze configures sFlow through YANG and ships no SFLOW-MIB, so those 12 bind a configuration interface Ze does not implement. SFLOW-V5-4.3-1 is the sFlowAgentAddress sentence 'The sFlowAgent address must provide SNMP connectivity to the agent.', which binds the sFlow agent Ze is, so it is a gap row rather than an exclusion. Three ids are declared unsourced here, all stated at SHOULD level in an OBJECT-TYPE DESCRIPTION that the MUST-level scan does not raise. SFLOW-V5-4.3-2 rests on the sFlowAgentAddress DESCRIPTION sentence 'In the case of a multi-homed agent, this should be the loopback address of the agent.'. x-2 rests on the same DESCRIPTION: 'The address should be an invariant that does not change as interfaces are reconfigured, enabled, disabled, added or removed.' and 'A manager should be able to use the sFlowAgentAddress as a unique key that will identify this agent over extended periods of time so that a history can be maintained.'. x-5 rests on the sFlowRcvrMaximumDatagramSize DESCRIPTION, 'The maximum number of data bytes that can be sent in a single sample datagram. The manager should set this value to avoid fragmentation of the sFlow datagrams.', with DEFVAL { 1400 }. |
| `6.1` | not stated | 1 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `A` | Appendix A | 0 | walked | Appendix A. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B` | Appendix B | 3 | walked | Appendix B. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 8, where its 3 site(s) were walked; every decision is carried forward by its verbatim quote. |

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
| `3.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates that a threshold-based sampler must still satisfy the equal-chance property the mapped row carries. | Calculation of an appropriate threshold value depends on the characteristics of the random number generator, however, the resulting sample stream must still satisfy (1). |
| `3.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes the counter-polling algorithm's piggyback condition; 'required Sampling Interval' is a noun phrase naming the configured interval, and the sentence states no obligation of its own. | Counters are only added to the datagram if the sources are within a short period, 5 seconds say, of failing to meet the required Sampling Interval (see sFlowCounterSamplingInterval in SFLOW MIB). |
| `4.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | The resulting translated MIB must be semantically equivalent, except where objects or events are omitted because no translation is possible (use of Counter64). |
| `4.2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the receiver of sFlow datagrams. Ze fills the sFlow AGENT role only: internal/plugins/flowexport/sflow encodes samples and sends datagrams to a configured collector, and no package decodes an inbound sFlow datagram. The producer would be an sFlow Collector, a receiving role Ze does not ship. | Before making any configuration changes, an sFlow Collector must first find a free row in the receiver table and then claim it by writing its owner string and a reservation time into a free row in the receiver table. |
| `4.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second sentence of the same sampler-independence rule, which the mapped row states in full. | Setting an attribute of one sampler must not alter the the behavior and settings of other sampler instances." SYNTAX Integer32 (1..65535) |
| `4.3:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | If non-zero the value must correspond to a valid, active sFlowRcvrIndex. |
| `4.3:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | If an entry in the sFlowRcvrTable expires, either because the sFlowRcvrOwner is set to the empty string or because the sFlowRcvrTimeout reaches zero, then the agent must mark all associated resources as available (by setting the associated SFlowReceiver entry to zero) and all values in these records must be restored to their default values. |
| `4.3:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | This sentence is part of the SFLOW-MIB object definition in Section 4.3 (SYNTAX, DEFVAL and OBJECT IDENTIFIER clauses surround it) and binds the SNMP configuration interface of an sFlow agent. Ze configures sFlow export through YANG (internal/plugins/flowexport/yang) and exposes no SNMP write path, so no receiver table, owner string or reservation timeout exists in the tree. The producer would be an SNMP agent implementing the SFLOW-MIB, which Ze does not ship. | The version string must have the following structure: <MIB Version>;<Organization>;<Software Revision> where: <MIB Version> must be '1.3', the version of this MIB. <Organization> the name of the organization responsible for the agent implementation. <Revision> the specific software build of this agent. |
| `4.3:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the management entity that configures an sFlow agent over SNMP, not to the agent. Ze manages no other device's sFlow MIB: its own configuration is YANG (internal/plugins/flowexport/yang). The producer would be an SNMP management station reading the SFLOW-MIB, which Ze does not ship. | Management entities must check the MIB Version and not attempt to manage agents with MIB Versions greater than that for which they were designed. |
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
| `B:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes the property a generator is judged by ('The essential property of the random number generator is that the mean value of the numbers it generates converges to the required sampling rate'). The obligation it introduces is stated in the next sentence, which is classified at 8:2. | The essential property of the random number generator is that the mean value of the numbers it generates converges to the required sampling rate. |
| `B:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes when the algorithm needs a new skip value ('A new skip value is only required every time a sample is taken'). It grants a licence to compute lazily rather than imposing a duty. | A new skip value is only required every time a sample is taken. |

## Superseded

No document obsoletes SFLOW-V5, so its obligations are stated where they were written.
