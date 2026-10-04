# RFC 3954 - Cisco Systems NetFlow Services Export Version 9

Experimental. Every requirement this repository extracted from RFC 3954, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 14.3% | 2 of 14 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 14.3% | 2 of 14 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 14 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 14 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 18 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 14 | of 26 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 9 | of 14 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 64.3% | 9 of 14 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 14 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 14 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 7.1% | 1 of 14 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 14 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 26 |
| Gated MUST-level | 14 |
| Not applicable, so out of scope | 9 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 4 |
| Discrimination records | 18 |
| Summary | `rfc/short/rfc3954.md` |
| Requirement shard | `rfc/requirements/rfc3954.md` |
| RFC text | `rfc/full/rfc3954.txt` |

## Enrolment

Enrolled: NetFlow Services Export Version 9 (ze as a v9 exporter): fourteen MUST-level requirements. Four are met: x-1 (never send a Data FlowSet before its Template) and x-8 (the sequence number is cumulative per observation domain) carry positive+negative tags; x-3 (network byte order) and x-9 (a Template ID is constant for the process lifetime) are {single-polarity: positive}. x-2 (refresh templates on both a time interval and a packet-count interval) is {gap}: ze refreshes on a configurable time interval only, with no packet-count-based interval. x-4, x-5, x-6, x-7 are {not-applicable}: they are NetFlow v9 collector requirements and ze is an exporter only. x-22, x-23, x-24, x-25 and x-26 were added by the 2026-09-21 extraction walk from RFC 3954 sections 7 and 9: they are v9 collector obligations (store the Template Record, do not assume Template ID uniqueness across Observation Domains, do not assume Template and Data travel together, do not assume a single Template FlowSet per packet, do not decode with an expired Template) and they are {not-applicable} for the same reason as x-4 to x-7. Disclosed in the docs/features/rfc-status.md RFC 3954 row.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- Flow export templates and records over UDP (exporter): no Data FlowSet before its Template, cumulative per-observation-domain sequence numbers, network byte order, constant Template IDs
- tests bound per requirement in [`rfc/requirements/rfc3954.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc3954.md).


**What the ledger says remains**

Template refresh is time-interval-based only, with no packet-count-based refresh interval (RFC3954-x-2 gap). ze is an exporter only, so the v9 collector requirements x-4 to x-7 are marked not applicable and the five collector obligations added on 2026-09-21 (x-22 to x-26) are marked not applicable too.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated (including scoped evidence) | 12 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **14** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC3954-x-1`](#rfc3954-x-1), [`RFC3954-x-8`](#rfc3954-x-8)

**Annotated (including scoped evidence) (12):** [`RFC3954-x-2`](#rfc3954-x-2), [`RFC3954-x-3`](#rfc3954-x-3), [`RFC3954-x-4`](#rfc3954-x-4), [`RFC3954-x-5`](#rfc3954-x-5), [`RFC3954-x-6`](#rfc3954-x-6), [`RFC3954-x-7`](#rfc3954-x-7), [`RFC3954-x-9`](#rfc3954-x-9), [`RFC3954-x-22`](#rfc3954-x-22), [`RFC3954-x-23`](#rfc3954-x-23), [`RFC3954-x-24`](#rfc3954-x-24), [`RFC3954-x-25`](#rfc3954-x-25), [`RFC3954-x-26`](#rfc3954-x-26)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3954-x-1` | After a NetFlow process restarts, the Exporter MUST NOT send any Data FlowSet without sending the corresponding Template FlowSet and the required Options Template FlowSet in a previous packet or including it in the same Export Packet. (§7) | MUST NOT | 7 | **positive:** `unit/verify` [`TestRFC3954RestartedEncodersSendTemplateBeforeData`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L187). **positive:** `unit/verify` [`TestRFC3954TemplateFlowSetPrecedesItsDataInOnePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L241). **positive:** `unit/verify` [`TestWriteExportPacketWithTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_encoder_test.go#L69). **negative:** `unit/verify` [`TestExporterTemplateFailureRetriesBeforeData`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc3954_exporter_lifecycle_test.go#L62) |
| `RFC3954-x-2` | On a regular basis, the Exporter MUST send all the Template Records and Options Template Records to refresh the Collector. Template IDs have a limited lifetime at the Collector and MUST be periodically refreshed. Two approaches are taken to make sure that Templates get refreshed at the Collector: * Every N number of Export Packets. * On a time basis, so every N number of minutes. Both options MUST be configurable by the user on the Exporter. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze refreshes NetFlow v9 templates on a time interval only (internal/plugins/flowexport/exporter.go:192-199, config template-refresh seconds); it has no packet-count-based template-refresh interval, so the conjoined time-and-packet-count refresh requirement is only partly met |
| `RFC3954-x-3` | The Exporter MUST code all binary integers of the Packet Header and the different FlowSets in network byte order (also known as the big-endian byte ordering). (§4) | MUST | 4 | **positive:** `unit/verify` [`TestNetflow9DataFlowSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_data_test.go#L10). **positive:** `unit/verify` [`TestNetflow9Header`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_encoder_test.go#L10). **positive:** `unit/verify` [`TestRFC3954CounterTemplateFlowSetBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_byteorder_test.go#L33). **positive:** `unit/verify` [`TestRFC3954FlowDataFlowSetBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L135). **positive:** `unit/verify` [`TestRFC3954FlowTailBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_byteorder_test.go#L77). **positive:** `unit/verify` [`TestRFC3954IPv6FlowDataFlowSetBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_byteorder_test.go#L95). **positive:** `unit/verify` [`TestRFC3954IPv6TemplateFlowSetBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_byteorder_test.go#L43). **positive:** `unit/verify` [`TestRFC3954TemplateFlowSetBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L118). **negative:** no negative test. **{single-polarity}:** ze only ENCODES NetFlow v9 (exporter-only, internal/plugins/flowexport/netflow9); every multi-octet field is written big-endian via binary.BigEndian and there is no decode or wrong-endianness code path to reject, so only the positive can be tested |
| `RFC3954-x-4` | The Collector MUST use the FlowSet ID to find the corresponding Template Record and decode the Flow Records from the FlowSet. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a NetFlow v9 collector requirement (map a Data FlowSet ID to its template); ze is a v9 exporter only (internal/plugins/flowexport/netflow9) with no v9 decode/collect code path |
| `RFC3954-x-5` | Because an individual Template FlowSet MAY contain multiple Template Records, the Length value MUST be used to determine the position of the next FlowSet record, which could be any type of FlowSet. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** collector requirement (use FlowSet Length to find the next FlowSet); ze does not collect v9 |
| `RFC3954-x-6` | Finally, note that the Collector MUST accept padding in the Data FlowSet and Options Template FlowSet, which means for the Flow Data Records, the Options Data Records and the Template Records. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** collector requirement (accept padding); ze does not collect v9 (its exporter does emit 4-octet padding at internal/plugins/flowexport/netflow9/data.go:38-44) |
| `RFC3954-x-7` | If a Collector should receive a new definition for an already existing Template ID, it MUST discard the previous template definition and use the new one. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** collector requirement (override a template on redefinition); ze does not collect v9 |
| `RFC3954-x-8` | Incremental sequence counter of all Export Packets sent from the current Observation Domain by the Exporter. This value MUST be cumulative (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestNetflow9FlowSeqNumPerPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_flow_adapter_test.go#L53). **positive:** `unit/verify` [`TestRFC3954SequenceCountsEveryExportPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L53). **negative:** `unit/verify` [`TestNetflow9SeqNumNotAdvancedOnSendError`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_adapter_test.go#L58) |
| `RFC3954-x-9` | The Template IDs must remain constant for the life of the NetFlow process on the Exporter. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestNetflow9FlowTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_flow_template_test.go#L8). **positive:** `unit/verify` [`TestNetflow9Template`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_template_test.go#L8). **positive:** `unit/verify` [`TestRFC3954TemplateIDsConstantAcrossRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L80). **negative:** no negative test. **{single-polarity}:** ze's NetFlow v9 template IDs are compile-time constants (CounterTemplateID=256 in internal/plugins/flowexport/netflow9/template.go, FlowTemplateID=257 and FlowTemplateID6=258 in flow_template.go) that are never reassigned for the life of the process, so there is no ID-change code path to test negatively |
| `RFC3954-x-10` | The Exporter SHOULD insert some padding bytes so that the subsequent FlowSet starts at a 4-byte aligned boundary. It is important to note that the Length field includes the padding bytes. Padding SHOULD be using zeros. (§5.3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-11` | In the event of configuration changes, the Exporter SHOULD send the new template definitions at an accelerated rate. In such a case, it MAY transmit the changed Template Record(s) and Options Template Record(s), without any data, in advance to help ensure that the Collector will have the correct template information before receiving the first data. 3. On a regular basis, the Exporter MUST send all the Template Records and Options Template Records to refresh the Collector. Template IDs have a limited lifetime at the Collector and MUST be periodically refreshed. Two approaches are taken to make sure that Templates get refreshed at the Collector: * Every N number of Export Packets. * On a time basis, so every N number of minutes. Both options MUST be configurable by the user on the Exporter. When one of these expiry conditions is met, the Exporter MUST send the Template FlowSet and Options Template. 4. In the event of a clock configuration change on the Exporter, the Exporter SHOULD send the template definitions at an accelerated rate. (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-12` | NetFlow Collectors SHOULD use the combination of the source IP address and the Source ID field to separate different export streams originating from the same Exporter. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-13` | If the Template Records have not been received at the time Flow Data Records (or Options Data Records) are received, the Collector SHOULD store the Flow Data Records (or Options Data Records) and decode them after the Template Records are received. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-14` | This value MUST be cumulative, and SHOULD be used by the Collector to identify whether any Export Packets have been missed. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-15` | If the template configuration is changed, the current Template ID is abandoned and SHOULD NOT be reused until the NetFlow process or Exporter restarts. (§7) | SHOULD NOT | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-16` | UDP [RFC768] is a non congestion-aware protocol, so when deploying NetFlow version 9 in a congestion-sensitive environment, make the connection between Exporter and NetFlow Collector through a dedicated link. This ensures that any burstiness in the NetFlow traffic affects only this dedicated link. When the NetFlow Collector can not be placed within a one-hop distance from the Exporter or when the export path from the Exporter to the NetFlow Collector can not be exclusively used for the NetFlow Export Packets, the export path should be designed so that it can always sustain the maximum burstiness of NetFlow traffic from the Exporter. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-17` | It MAY transmit the Template FlowSet and Options Template FlowSet, without any Data FlowSets, in advance to help ensure that the Collector will have the correct Template Record before receiving the first Flow or Options Data Record. (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-18` | If the Exporter experiences internal constraints, a Flow MAY be forced to expire prematurely; for example, counters wrapping or low memory. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-19` | Because an individual Template FlowSet MAY contain multiple Template Records (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-20` | Flow Data records that correspond to a Template Record MAY appear in the same and/or subsequent Export Packets. (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-21` | The following value fields are reserved for proprietary field types: 25, 26, 43 to 45, 51 to 54, and 65 to 69. (§8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC3954-x-22` | As such, the NetFlow Collector MUST store the Template Record to interpret the corresponding Flow Data Records that are received in subsequent data packets. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a NetFlow v9 Collector obligation, and ze fills only the Exporter role; internal/plugins/flowexport/sender.go::NewSender dials one connected UDP socket towards the collector, and the flowexport plugin opens no listening socket and decodes no v9 Export Packet |
| `RFC3954-x-23` | A NetFlow Collector that receives Export Packets from several Observation Domains from the same Exporter MUST be aware that the uniqueness of the Template ID is not guaranteed across Observation Domains. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a NetFlow v9 Collector obligation, and ze fills only the Exporter role; internal/plugins/flowexport/sender.go::NewSender dials one connected UDP socket towards the collector, and the flowexport plugin opens no listening socket and decodes no v9 Export Packet |
| `RFC3954-x-24` | A Collector device MUST NOT assume that the Data FlowSet and the associated Template FlowSet (or Options Template FlowSet) are exported in the same Export Packet. (§9) | MUST NOT | 9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a NetFlow v9 Collector obligation, and ze fills only the Exporter role; internal/plugins/flowexport/sender.go::NewSender dials one connected UDP socket towards the collector, and the flowexport plugin opens no listening socket and decodes no v9 Export Packet |
| `RFC3954-x-25` | The Collector MUST NOT assume that one and only one Template FlowSet is present in an Export Packet. (§9) | MUST NOT | 9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a NetFlow v9 Collector obligation, and ze fills only the Exporter role; internal/plugins/flowexport/sender.go::NewSender dials one connected UDP socket towards the collector, and the flowexport plugin opens no listening socket and decodes no v9 Export Packet |
| `RFC3954-x-26` | The Collector MUST NOT attempt to decode the Flow or Options Data Records with an expired Template. (§9) | MUST NOT | 9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a NetFlow v9 Collector obligation, and ze fills only the Exporter role; internal/plugins/flowexport/sender.go::NewSender dials one connected UDP socket towards the collector, and the flowexport plugin opens no listening socket and decodes no v9 Export Packet |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3954-x-2`](#rfc3954-x-2) On a regular basis, the Exporter MUST send all the Template Records and Options Template Records to refresh the Collector. Template IDs have a limited lifetime at the Collector and MUST be periodically refreshed. Two approaches are taken to make sure that Templates get refreshed at the Collector: * Every N number of Export Packets. * On a time basis, so every N number of minutes. Both options MUST be configurable by the user on the Exporter. (§7) | {gap}, no test | ze refreshes NetFlow v9 templates on a time interval only (internal/plugins/flowexport/exporter.go:192-199, config template-refresh seconds); it has no packet-count-based template-refresh interval, so the conjoined time-and-packet-count refresh requirement is only partly met |
| [`RFC3954-x-4`](#rfc3954-x-4) The Collector MUST use the FlowSet ID to find the corresponding Template Record and decode the Flow Records from the FlowSet. (§5.3) | no test | no test carries this requirement id; annotated {not-applicable}: this is a NetFlow v9 collector requirement (map a Data FlowSet ID to its template); ze is a v9 exporter only (internal/plugins/flowexport/netflow9) with no v9 decode/collect code path |
| [`RFC3954-x-5`](#rfc3954-x-5) Because an individual Template FlowSet MAY contain multiple Template Records, the Length value MUST be used to determine the position of the next FlowSet record, which could be any type of FlowSet. (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: collector requirement (use FlowSet Length to find the next FlowSet); ze does not collect v9 |
| [`RFC3954-x-6`](#rfc3954-x-6) Finally, note that the Collector MUST accept padding in the Data FlowSet and Options Template FlowSet, which means for the Flow Data Records, the Options Data Records and the Template Records. (§9) | no test | no test carries this requirement id; annotated {not-applicable}: collector requirement (accept padding); ze does not collect v9 (its exporter does emit 4-octet padding at internal/plugins/flowexport/netflow9/data.go:38-44) |
| [`RFC3954-x-7`](#rfc3954-x-7) If a Collector should receive a new definition for an already existing Template ID, it MUST discard the previous template definition and use the new one. (§7) | no test | no test carries this requirement id; annotated {not-applicable}: collector requirement (override a template on redefinition); ze does not collect v9 |
| [`RFC3954-x-22`](#rfc3954-x-22) As such, the NetFlow Collector MUST store the Template Record to interpret the corresponding Flow Data Records that are received in subsequent data packets. (§7) | no test | no test carries this requirement id; annotated {not-applicable}: a NetFlow v9 Collector obligation, and ze fills only the Exporter role; internal/plugins/flowexport/sender.go::NewSender dials one connected UDP socket towards the collector, and the flowexport plugin opens no listening socket and decodes no v9 Export Packet |
| [`RFC3954-x-23`](#rfc3954-x-23) A NetFlow Collector that receives Export Packets from several Observation Domains from the same Exporter MUST be aware that the uniqueness of the Template ID is not guaranteed across Observation Domains. (§7) | no test | no test carries this requirement id; annotated {not-applicable}: a NetFlow v9 Collector obligation, and ze fills only the Exporter role; internal/plugins/flowexport/sender.go::NewSender dials one connected UDP socket towards the collector, and the flowexport plugin opens no listening socket and decodes no v9 Export Packet |
| [`RFC3954-x-24`](#rfc3954-x-24) A Collector device MUST NOT assume that the Data FlowSet and the associated Template FlowSet (or Options Template FlowSet) are exported in the same Export Packet. (§9) | no test | no test carries this requirement id; annotated {not-applicable}: a NetFlow v9 Collector obligation, and ze fills only the Exporter role; internal/plugins/flowexport/sender.go::NewSender dials one connected UDP socket towards the collector, and the flowexport plugin opens no listening socket and decodes no v9 Export Packet |
| [`RFC3954-x-25`](#rfc3954-x-25) The Collector MUST NOT assume that one and only one Template FlowSet is present in an Export Packet. (§9) | no test | no test carries this requirement id; annotated {not-applicable}: a NetFlow v9 Collector obligation, and ze fills only the Exporter role; internal/plugins/flowexport/sender.go::NewSender dials one connected UDP socket towards the collector, and the flowexport plugin opens no listening socket and decodes no v9 Export Packet |
| [`RFC3954-x-26`](#rfc3954-x-26) The Collector MUST NOT attempt to decode the Flow or Options Data Records with an expired Template. (§9) | no test | no test carries this requirement id; annotated {not-applicable}: a NetFlow v9 Collector obligation, and ze fills only the Exporter role; internal/plugins/flowexport/sender.go::NewSender dials one connected UDP socket towards the collector, and the flowexport plugin opens no listening socket and decodes no v9 Export Packet |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3954-x-1`](#rfc3954-x-1)

After a NetFlow process restarts, the Exporter MUST NOT send any Data FlowSet without sending the corresponding Template FlowSet and the required Options Template FlowSet in a previous packet or including it in the same Export Packet. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both halves now proven. Exporter ordering: TestExporterTemplateFailureRetriesBeforeData (counter and flow subtests) goes red if data leaves after a failed Template or the retry transcript is not template-then-data (recorded, notifySnapshot). Corresponding Template: TestRFC3954RestartedEncodersSendTemplateBeforeData drives fresh real CounterEncoder and FlowEncoder and requires every Data FlowSet 256/257/258 to follow a packet whose Template FlowSet defines that ID, and each ID to be seen, so an encoder omitting 258 goes red (recorded, EncodeFlowTemplate). TestRFC3954TemplateFlowSetPrecedesItsDataInOnePacket pins same-packet order (recorded). The Options Template clause is vacuous: Ze emits no Options Data. TestWriteExportPacketWithTemplate stays a count-only floor and carries no record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExporterTemplateFailureRetriesBeforeData`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/rfc3954_exporter_lifecycle_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestWriteExportPacketWithTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_encoder_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC3954RestartedEncodersSendTemplateBeforeData`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L187) | unit/verify | revert, verified |
| positive | [`TestRFC3954TemplateFlowSetPrecedesItsDataInOnePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L241) | unit/verify | revert, verified |

### [`RFC3954-x-2`](#rfc3954-x-2)

On a regular basis, the Exporter MUST send all the Template Records and Options Template Records to refresh the Collector. Template IDs have a limited lifetime at the Collector and MUST be periodically refreshed. Two approaches are taken to make sure that Templates get refreshed at the Collector: * Every N number of Export Packets. * On a time basis, so every N number of minutes. Both options MUST be configurable by the user on the Exporter. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-2, so no unit is bound to it.

### [`RFC3954-x-3`](#rfc3954-x-3)

The Exporter MUST code all binary integers of the Packet Header and the different FlowSets in network byte order (also known as the big-endian byte ordering). (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity row, every writer now compared byte for byte with distinct-octet values: Packet Header (TestNetflow9Header), counter Data FlowSet (TestNetflow9DataFlowSet), counter Template (TestRFC3954CounterTemplateFlowSetBigEndian), IPv4 and IPv6 flow Templates, IPv4 flow Data FlowSet, the shared tail with non-zero SRC_AS, DST_AS, FIRST_SWITCHED, LAST_SWITCHED (TestRFC3954FlowTailBigEndian), and the IPv6 Data FlowSet header and tail. A little-endian write of any field changes compared bytes. Every unit recorded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3954CounterTemplateFlowSetBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_byteorder_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestRFC3954FlowTailBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_byteorder_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC3954IPv6FlowDataFlowSetBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_byteorder_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestRFC3954IPv6TemplateFlowSetBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_byteorder_test.go#L43) | unit/verify | revert, verified |
| positive | [`TestNetflow9DataFlowSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_data_test.go#L10) | unit/verify | revert, verified |
| positive | [`TestNetflow9Header`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_encoder_test.go#L10) | unit/verify | revert, verified |
| positive | [`TestRFC3954FlowDataFlowSetBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L135) | unit/verify | revert, verified |
| positive | [`TestRFC3954TemplateFlowSetBigEndian`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L118) | unit/verify | revert, verified |

### [`RFC3954-x-4`](#rfc3954-x-4)

The Collector MUST use the FlowSet ID to find the corresponding Template Record and decode the Flow Records from the FlowSet. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-4, so no unit is bound to it.

### [`RFC3954-x-5`](#rfc3954-x-5)

Because an individual Template FlowSet MAY contain multiple Template Records, the Length value MUST be used to determine the position of the next FlowSet record, which could be any type of FlowSet. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-5, so no unit is bound to it.

### [`RFC3954-x-6`](#rfc3954-x-6)

Finally, note that the Collector MUST accept padding in the Data FlowSet and Options Template FlowSet, which means for the Flow Data Records, the Options Data Records and the Template Records. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-6, so no unit is bound to it.

### [`RFC3954-x-7`](#rfc3954-x-7)

If a Collector should receive a new definition for an already existing Template ID, it MUST discard the previous template definition and use the new one. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-7, so no unit is bound to it.

### [`RFC3954-x-8`](#rfc3954-x-8)

Incremental sequence counter of all Export Packets sent from the current Observation Domain by the Exporter. This value MUST be cumulative (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC3954SequenceCountsEveryExportPacket sends template-only, counter Data and flow Data packets from two encoders on one sender over two rounds and requires ten consecutive sequence values, so a template-only packet left out, or a per-encoder counter, goes red (recorded, sendTemplatePacket). Negative TestNetflow9SeqNumNotAdvancedOnSendError: an unsent packet does not advance the count of packets sent (recorded).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNetflow9SeqNumNotAdvancedOnSendError`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_adapter_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestNetflow9FlowSeqNumPerPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_flow_adapter_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC3954SequenceCountsEveryExportPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L53) | unit/verify | revert, verified |

### [`RFC3954-x-9`](#rfc3954-x-9)

The Template IDs must remain constant for the life of the NetFlow process on the Exporter. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity row. TestRFC3954TemplateIDsConstantAcrossRefresh sends two full rounds and requires Templates 256, 257, 258 unchanged in round two and the Data FlowSets to name 256 and 257, so a refresh that reassigns an ID goes red (recorded, BuildFlowTemplate6). The IPv6 Template 258 is now asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNetflow9FlowTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_flow_template_test.go#L8) | unit/verify | revert, verified |
| positive | [`TestNetflow9Template`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_template_test.go#L8) | unit/verify | revert, verified |
| positive | [`TestRFC3954TemplateIDsConstantAcrossRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/netflow9/rfc3954_test.go#L80) | unit/verify | revert, verified |

### [`RFC3954-x-22`](#rfc3954-x-22)

As such, the NetFlow Collector MUST store the Template Record to interpret the corresponding Flow Data Records that are received in subsequent data packets. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-22, so no unit is bound to it.

### [`RFC3954-x-23`](#rfc3954-x-23)

A NetFlow Collector that receives Export Packets from several Observation Domains from the same Exporter MUST be aware that the uniqueness of the Template ID is not guaranteed across Observation Domains. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-23, so no unit is bound to it.

### [`RFC3954-x-24`](#rfc3954-x-24)

A Collector device MUST NOT assume that the Data FlowSet and the associated Template FlowSet (or Options Template FlowSet) are exported in the same Export Packet. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-24, so no unit is bound to it.

### [`RFC3954-x-25`](#rfc3954-x-25)

The Collector MUST NOT assume that one and only one Template FlowSet is present in an Export Packet. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-25, so no unit is bound to it.

### [`RFC3954-x-26`](#rfc3954-x-26)

The Collector MUST NOT attempt to decode the Flow or Options Data Records with an expired Template. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3954-x-26, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc3954.txt |
| Source fingerprint | 4274d093e24b4bad |
| Record | rfc/extraction/rfc3954.json |
| Mapped sentences | 13 |
| Declined as scope | 6 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `5.2` | not stated | 1 | walked | not stated |
| `5.3` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 1 | walked | not stated |
| `6.2` | not stated | 1 | walked | not stated |
| `7` | not stated | 8 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 5 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |
| `10.3` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `11.1` | not stated | 0 | walked | not stated |
| `11.2` | not stated | 0 | walked | not stated |
| `11.3` | not stated | 0 | walked | not stated |
| `11.4` | not stated | 0 | walked | not stated |
| `11.5` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `15` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `6.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the Length-determines-next-FlowSet obligation already mapped at site 5.2:1 | Thus, the Length value MUST be used to determine the position of the next FlowSet record, which could be either a Template FlowSet or Data FlowSet. |
| `6.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the FlowSet-ID-selects-the-template obligation already mapped at site 5.3:1 | The Collector MUST use the FlowSet ID to map the appropriate type and length to any field values that follow. |
| `7:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the periodic template refresh obligation already mapped at site 7:5 | Template IDs have a limited lifetime at the Collector and MUST be periodically refreshed. |
| `7:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the configurability of the two refresh intervals is part of the same refresh obligation mapped at site 7:5 | Both options MUST be configurable by the user on the Exporter. |
| `7:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the send-on-expiry half of the refresh obligation already mapped at site 7:5 | When one of these expiry conditions is met, the Exporter MUST send the Template FlowSet and Options Template. |
| `9:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the override-on-redefinition obligation already mapped at site 7:3 | If the Collector receives a new Template Record (for example, in the case of an Exporter restart) it MUST immediately override the existing Template Record. |

## Superseded

No document obsoletes RFC 3954, so its obligations are stated where they were written.
