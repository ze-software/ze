# Spec: Flow Export - Sampled Scale (NetFlow v9 / IPFIX at high rate)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-flow-export-2-flow-records (closed, learned 819) |
| Phase | - |
| Updated | 2026-05-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-flow-export-0-umbrella.md` - umbrella architecture
4. `plan/spec-flow-export-1-counter-export.md` - shared component / sender / encoder factory pattern
5. `plan/spec-flow-export-2-flow-records.md` - sampling + conntrack + per-flow encoders this spec extends
6. `plan/learned/818-flow-export-1-counter-export.md`, `819-flow-export-2-flow-records.md`, `820-flow-export-0-umbrella.md`
7. `internal/component/flowexport/conntrack/delta.go` - the SCALING RISK / TOMBSTONE INVARIANT / GRACE BOUNDS doc blocks
8. `internal/component/flowexport/sampling_worker.go` - the existing psample read loop -> ExportFlowSample
9. `docs/guide/flow-export.md` - the "Scale: conntrack vs sampling" section this spec implements against

## Task

Scale NetFlow v9 / IPFIX flow export to high-rate / 100Gbps links by adding a
**sampled flow-record path** that builds on the packet-sampling plumbing already
delivered in spec 2. Today (spec 2) the only per-flow NetFlow v9 / IPFIX records
come from a full conntrack table dump whose cost scales with flow **churn**;
sFlow already exports sampled `flow_sample` records from the psample reader, but
NetFlow v9 and IPFIX have no sampled path at all. This spec makes the
sampling worker drive NetFlow v9 / IPFIX **PSAMP-style sampled flow records**
(per-packet sampled records carrying the sampling interval), so flow visibility
on a 100G link costs the same per-second work regardless of whether the link
carries 1k or 1M new flows per second.

**Primary scope (this spec):** sampled NetFlow v9 / IPFIX export.
- Route the existing `samplingWorker` psample stream (currently sFlow-only) to
  NetFlow v9 / IPFIX flow-record encoders as sampled records.
- Carry the sampling rate as the standard sampling Information Element so a
  collector can scale the observed counts back up (`samplingInterval` IE 34 for
  IPFIX; the NetFlow v9 SAMPLING_INTERVAL field 34 for v9).
- Make the rate-independence property observable and testable: a `.ci`
  functional test that drives N packets at two very different flow-churn levels
  and asserts export work (datagrams/sec, CPU budget) does not change with churn.
- Config surface to select sampled vs conntrack per-flow export per collector,
  metrics, and `ze doctor` / health coverage for the new path.

**Deferred / out-of-scope track:** exact unsampled per-flow conntrack export at
100G (sharded delta tracker, conntrack-flow sampling, event-driven dump removal,
bounded backpressure, in-datapath aggregation). Captured below under
"Alternatives / Future / Out of Scope" with its five requirements and an explicit
decision to defer pending a real requirement for exact unsampled accounting.

→ Decision: 100G flow visibility is delivered by SAMPLING, not by per-flow
conntrack, because sampling cost is independent of flow rate while conntrack cost
scales with churn. (Confirmed by the SCALING RISK block in `conntrack/delta.go`
and the "Scale" section in `docs/guide/flow-export.md`.)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component lifecycle, registration, in-process SDK
  → Constraint: flowexport runs in-process; the core never imports it; encoders register via factory
- [ ] `ai/patterns/registration.md` - factory registration for encoders
  → Constraint: protocol packages import `flowexport`, so `flowexport` cannot import them; new encoders register via `RegisterFlowRecordEncoderFactory`
- [ ] `plan/spec-flow-export-0-umbrella.md` - cross-cutting architecture
  → Decision: single collection, multiple consumers; buffer-first; in-process SDK
- [ ] `plan/spec-flow-export-2-flow-records.md` - sampling/conntrack/enrich + per-flow encoders
  → Decision: tc sample + psample for packet sampling; conntrack dump for exact per-flow
  → Constraint: `FlowSample` and `ConntrackFlow` are neutral value types crossing the boundary; adapters convert to internal record types and time bases

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3954.md` - NetFlow v9 template + data FlowSet, field types
  → Constraint: SAMPLING_INTERVAL is field type 34, SAMPLING_ALGORITHM is field type 35; a flow record exported from a sampled stream MUST advertise the interval so the collector can scale counts
  → Constraint: Count field = total records across all FlowSets; Sequence Number increments per export packet (per the spec-2 fix, per datagram not per record)
- [ ] `rfc/short/rfc7011.md` - IPFIX message + template + data Set framing
  → Constraint: template Set ID = 2; data Set ID = template ID (256+); sequence counts data records only
- [ ] `rfc/short/rfc7012.md` - IPFIX Information Elements
  → Constraint: IE 34 = samplingInterval (the 1-in-N value); IE 35 = samplingAlgorithm; these are the IPFIX equivalents of the v9 sampling fields and let the collector reconstruct unsampled volume
  → Constraint: 5-tuple IEs (8/12/27/28/7/11/4) and counter IEs (1/2 delta, 85/86 total) already used by the spec-2 per-flow templates
- [ ] `rfc/short/sflow-v5.md` - flow_sample already carries sampling_rate
  → Constraint: sFlow needs no change; its flow_sample already advertises sampling_rate. NetFlow v9 / IPFIX are the gap.

**Note:** PSAMP (RFC 5476, "Packet Sampling exporting") defines the formal
sampled-record model for IPFIX. No `rfc/short/rfc5476.md` summary exists yet; the
sampling IEs this spec uses (34/35) are defined in RFC 7012, which does have a
summary. Creating the RFC 5476 short summary is the first implementation step
(`/ze-rfc 5476`) before any IPFIX sampled-record encoding is written.

**Key insights:**
- The psample read loop and `ExportFlowSample` already exist and are rate-independent (1-in-N). The missing piece is a NetFlow v9 / IPFIX sampled-record encoder fed from the same stream, not new kernel plumbing.
- sFlow flow samples already carry the rate; the work is to give NetFlow v9 / IPFIX the same self-describing-rate property via IE 34 / field 34.
- Conntrack per-flow export stays as-is for exact accounting at modest churn; this spec adds a parallel sampled path, it does not remove conntrack.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/component/flowexport/sampling_worker.go` - `samplingWorker` installs tc sample on configured interfaces and runs ONE long-lived goroutine reading `PsampleReader.Read()`; each sample is dispatched ONLY via `w.exp.ExportFlowSample(FlowSample{...})`; `incSamples(name)` is the per-interface metric
  → Constraint: the sample stream is already rate-independent (1-in-N) and already crosses the boundary as the neutral `FlowSample` value type; routing it to NetFlow v9 / IPFIX needs no new kernel code
  → Constraint: a single goroutine + single psample socket today; at very high sample volume this is a budget to watch, but rate is operator-controlled via `rate`
- [ ] `internal/component/flowexport/exporter.go` - `ExportFlowSample` fans a `FlowSample` only to collectors with a non-nil `flowSample` (sFlow) encoder; `ExportFlows` fans `ConntrackFlow` to collectors with a `flowRecord` (NetFlow v9 / IPFIX) encoder; both take `e.mu`; datagram/byte/error metrics computed from `sender.Stats()` deltas
  → Constraint: there is NO path today from a sampled packet to a NetFlow v9 / IPFIX record. `flowRecord` is fed only by the conntrack worker.
  → Constraint: `collectorState` already carries `flowRecord FlowRecordEncoder` and `flowTemplateLast`; the sampled path can reuse template refresh logic
- [ ] `internal/component/flowexport/conntrack/delta.go` - `DeltaTracker` holds one `map[flowKey]lastExported` entry per recently-seen flow under a SINGLE `sync.Mutex`; the `SCALING RISK` doc block states ~175 bytes/entry, Go maps never shrink on delete, and explicitly "does NOT scale to 100G internet-mix churn (10k-1M new flows/sec)"; tombstone reclaim (`SweepTombstones`) bounds residency to `conntrackTombstoneGrace`
  → Constraint: the conntrack path's own code names sampling as the 100G answer ("For 100G flow export use SAMPLING ... not per-flow conntrack export")
  → Constraint: the single mutex serializes the periodic-dump goroutine and the destroy-listener goroutine; this is bottleneck #2 in the analysis and a prerequisite-shard target for the deferred exact-100G track
- [ ] `internal/component/flowexport/conntrack_worker.go` - `run()` dumps the full table every `active-timeout` via `reader.Dump()`; `runDestroy()` reads `DestroyListener` events; `conntrackTombstoneGrace = 5s`; `Cleanup(2*active-timeout)` is the missed-event backstop
  → Constraint: the destroy listener IS implemented (spec-2 had it deferred as C4; the code now has `destroy_linux.go`). It is a SINGLE socket + SINGLE parse goroutine -- bottleneck #4 in the analysis (ENOBUFS / dropped destroy events at ~1M events/sec)
  → Constraint: `reader.Dump()` calls `ConntrackTableList` once per address family -- the full-table serialization, bottleneck #3
- [ ] `internal/component/flowexport/conntrack/reader_linux.go` - `Dump()` lists the whole table for AF_INET and AF_INET6 via `handle.ConntrackTableList`; uses `vishvananda/netlink`
  → Constraint: a 100G internet-mix table is millions of entries serialized over netlink every dump
- [ ] `internal/component/flowexport/conntrack/destroy_linux.go` - `DestroyListener` opens ONE `NETLINK_NETFILTER` socket via `mdlayher/netlink`, joins `NFNLGRP_CONNTRACK_DESTROY`, `Read()` is one blocking `conn.Receive()`; no `SO_RCVBUF` tuning, no multi-goroutine parse
  → Constraint: at ~1M destroy events/sec a single socket with a single parse goroutine drops events (ENOBUFS) -> lost accounting; this is the exact-100G prerequisite, not the sampled path
- [ ] `internal/component/flowexport/netflow9/flow_template.go`, `flow_data.go`, `flow_adapter.go` - per-flow IPv4 (257) / IPv6 (258) templates + data; `flow_adapter.go` advances seqNum per datagram (spec-2 fix); NO sampling field (34/35) in the templates today
  → Constraint: adding the SAMPLING_INTERVAL field is a template change (new template IDs to avoid clashing with the unsampled conntrack templates) plus a data-record field
- [ ] `internal/component/flowexport/ipfix/flow_template.go`, `flow_data.go`, `flow_adapter.go`, `ie.go` - per-flow IPv4 / IPv6 templates; `ie.go` defines the IE constants; NO samplingInterval IE (34) today
  → Constraint: same: a sampled template variant carrying IE 34, plus data encoding of the rate
- [ ] `internal/component/flowexport/sflow/flow.go`, `flow_adapter.go` - sFlow flow_sample already encodes `sampling_rate`; reference implementation for "self-describing rate"
  → Constraint: sFlow is the model; do not change it
- [ ] `internal/component/flowexport/config.go` - `CollectorConfig` (protocol, polling/template intervals), `SamplingConfig` (interface/rate/trunc-size/group), `ConntrackConfig` (enabled/active-timeout); validation ranges; coercion helpers `cfg*` accept both JSON string and number
  → Constraint: no per-collector selection of "sampled vs conntrack" flow-record source today; conntrack is global, sampling is per-interface
- [ ] `internal/component/flowexport/metrics.go` - `ze_flowexport_samples_total{interface}`, `ze_flowexport_flows_total{collector}`, `ze_flowexport_flows_active`, plus datagrams/bytes/errors `{collector,protocol}`
  → Constraint: no metric distinguishes sampled vs unsampled flow records today
- [ ] `docs/guide/flow-export.md` "Scale: conntrack vs sampling" - states conntrack is for modest churn, sampling is the scalable mechanism for line-rate flow visibility
  → Constraint: this spec must keep the doc accurate: after this spec, "use sampling at 100G" is true for NetFlow v9 / IPFIX as well, not only sFlow

**Behavior to preserve:**
- sFlow flow_sample path unchanged (already carries sampling_rate).
- Conntrack per-flow export path unchanged for exact accounting at modest churn (the SCALING RISK / TOMBSTONE INVARIANT / GRACE BOUNDS invariants in `delta.go` are NOT touched by this spec).
- Existing per-flow templates (257/258, unsampled, conntrack-sourced) keep their IDs and field sets; the sampled variant uses NEW template IDs so a collector can tell the two streams apart.
- Counter export (spec 1), config reload behaviour, exporter Stop ordering, metric names already shipped.
- NetFlow v9 sequence-per-datagram and IPFIX sequence-per-data-record semantics (the spec-2 fixes).

**Behavior to change:**
- The sampling worker, when a collector is NetFlow v9 / IPFIX and configured for sampled flow records, also dispatches the psample stream to that collector's flow-record encoder as a sampled record (carrying the rate).
- NetFlow v9 / IPFIX gain a sampled per-flow template variant (with SAMPLING_INTERVAL field 34 / IE 34) and the data encoding for it.
- Config gains a per-collector selector for the flow-record source (sampled vs conntrack) and validation that sampled requires a configured sampling interface.

## Data Flow (MANDATORY)

### Entry Point
- Config: YANG `flow-export` -> a NetFlow v9 / IPFIX collector marked to take SAMPLED flow records, plus an existing `sampling { interface ... rate N }` stanza (spec 2).
- Data: the same psample generic-netlink multicast stream spec 2 already reads in `samplingWorker.run()` (1-in-N sampled packets with ifIndex, rate, orig_size, truncated header).

### Transformation Path
1. tc sample action (spec 2, `sampling/tc_linux.go`) copies 1-in-N packets to the psample group; cost is independent of flow rate.
2. `samplingWorker.run()` reads each `SampledPacket` and builds the neutral `FlowSample{IfIndex, Rate, OrigSize, Header}` value (spec 2, unchanged).
3. NEW: the worker dispatches the `FlowSample` not only to `ExportFlowSample` (sFlow) but, for collectors configured for sampled NetFlow v9 / IPFIX, to a sampled-flow-record dispatch on the exporter.
4. NEW: the NetFlow v9 / IPFIX sampled adapter parses the 5-tuple from the sampled header bytes (Ethernet/IP/L4 the way `show capture` already decodes), builds a per-flow data record with the sampling interval field (34 / IE 34), and writes it buffer-first into a pooled datagram.
5. Template refresh reuses the existing `flowTemplateLast` cadence; the sampled template carries the new template IDs.
6. `Sender.Send` transmits via the shared UDP socket (spec 1); datagram/byte metrics computed from `sender.Stats()` deltas; a NEW metric labels records as sampled.
7. The collector scales observed counts by the advertised sampling interval to reconstruct unsampled volume.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| kernel tc sample -> psample -> Ze | generic netlink multicast (owned header copy) -- spec 2, unchanged | [ ] |
| samplingWorker -> exporter (sampled NF9/IPFIX) | NEW dispatch of the neutral `FlowSample` value to a sampled-flow-record encoder via `e.mu` | [ ] |
| flowexport -> protocol packages | factory registration (`RegisterFlowRecordEncoderFactory` or a new sampled variant); flowexport cannot import the protocol packages | [ ] |
| flowexport -> network | UDP sendto with pre-encoded buffer (spec 1 sender) | [ ] |

### Integration Points
- `internal/component/flowexport/sampling_worker.go` - extend the dispatch in `run()` to also feed sampled NF9/IPFIX collectors.
- `internal/component/flowexport/exporter.go` - add a sampled-flow-record dispatch alongside `ExportFlowSample` / `ExportFlows`, reusing `collectorState` template-refresh fields.
- `internal/component/flowexport/netflow9/`, `ipfix/` - sampled template variant + data encoding + factory registration.
- `internal/component/flowexport/config.go` + `schema/ze-flowexport-conf.yang` - per-collector flow-record source selector + validation.
- `internal/component/flowexport/metrics.go` - sampled-record metric / label.
- `internal/component/cmd/show/flow_export.go` - surface sampled-vs-conntrack source in `show flow-export`.
- Reuses spec-2 5-tuple header decoding (the same Ethernet/IP/L4 parse used by `show capture interface`).

### Integration Points
- `samplingWorker` (spec 2) - the rate-independent psample stream this spec routes to NF9/IPFIX.
- `Exporter` `collectorState.flowRecord` and `flowTemplateLast` - reused for the sampled path.
- `FlowRecordEncoder` factory registration - the sampled encoders register the same way as the conntrack ones.
- Shared `Sender` + buffer pool (spec 1) - reused unchanged.

### Architectural Verification
- [ ] No bypassed layers (sampled records flow through the existing FlowRecordEncoder factory + Sender)
- [ ] No unintended coupling (header 5-tuple decode is shared, not duplicated)
- [ ] No duplicated functionality (reuses spec-2 psample reader and spec-1 sender/template machinery; does not re-implement sampling)
- [ ] Zero-copy preserved where applicable (sampled header bytes parsed in place; record written buffer-first into the pooled datagram)

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| collector configured `protocol ipfix; flow-source sampled` + `sampling {}` | → | sampled-flow-record encoder wired to collector | `TestSampledFlowEncoderWiring` |
| psample event with a sampled NF9/IPFIX collector present | → | sampled NetFlow v9 / IPFIX data record dispatched | `TestSampledDispatchWiring` |
| sampled IPFIX template refresh fires | → | sampled template (new ID) with IE 34 emitted | `TestIPFIXSampledTemplateWiring` |
| `show flow-export` with a sampled collector | → | `Exporter.Status()` reports flow-source=sampled | `test/flow-export/sampled-scale-show.ci` |
| N packets at low vs high churn through a sampled collector | → | export datagram rate independent of churn | `test/flow-export/sampled-rate-independence.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Collector `protocol ipfix; flow-source sampled` with no `sampling {}` stanza | Config validation rejects with a clear error (sampled requires a sampling interface) |
| AC-2 | Collector `protocol ipfix; flow-source sampled` + `sampling { interface eth0; rate 1024 }` | psample stream produces IPFIX sampled per-flow data records; no conntrack dump is started for that flow path |
| AC-3 | IPFIX sampled records | Each record carries samplingInterval (IE 34) = configured `rate`; template Set advertises IE 34; sampled template uses a NEW template ID distinct from the unsampled (257/258) IDs |
| AC-4 | NetFlow v9 sampled records | Records carry SAMPLING_INTERVAL (field type 34) = `rate`; data FlowSet references the sampled template; Count field accurate; sequence per datagram |
| AC-5 | Two runs: same total packets, flow churn 1k/s vs ~100k/s, same `rate` | Export datagrams/sec and per-second CPU budget do not materially change with churn (rate-independence), proving the sampled path scales where conntrack does not |
| AC-6 | sFlow collector + sampling, unchanged | sFlow flow_sample still carries sampling_rate; no regression |
| AC-7 | Collector `flow-source conntrack` (default) | Existing spec-2 conntrack per-flow export unchanged; DeltaTracker invariants untouched |
| AC-8 | `show flow-export` | Per-collector status reports the flow-record source (sampled / conntrack / none) alongside existing fields |
| AC-9 | Prometheus `/metrics` | A sampled-record metric (or a `source` label on `ze_flowexport_flows_total`) distinguishes sampled from conntrack-sourced flow records |
| AC-10 | `ze doctor` / health with a sampled collector | Reports psample/tc-sample readiness (CAP_NET_ADMIN, psample module) for the sampled path, reusing the spec-2 sampling dependency checks |
| AC-11 | Sampled header too short to parse a 5-tuple (e.g. trunc-size cut mid-header) | Record is skipped (or emitted with zeroed L4) without crash; a drop counter increments; no panic, no slice out-of-range |
| AC-12 | Config reload toggles a collector between `sampled` and `conntrack` | Exporter swaps cleanly; the dropped path's workers stop; no leaked goroutine or socket |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIPFIXSampledTemplate` | `flowexport/ipfix/flow_template_test.go` | sampled template carries IE 34 (samplingInterval) and a new template ID distinct from 257/258 | |
| `TestIPFIXSampledData` | `flowexport/ipfix/flow_data_test.go` | sampled data record encodes IE 34 = rate, 5-tuple from header, counters | |
| `TestNetflow9SampledTemplate` | `flowexport/netflow9/flow_template_test.go` | sampled template carries SAMPLING_INTERVAL field 34, new template ID | |
| `TestNetflow9SampledData` | `flowexport/netflow9/flow_data_test.go` | sampled data record encodes field 34 = rate; sequence per datagram preserved | |
| `TestSampledHeaderDecode` | `flowexport/sampling/decode_test.go` or shared decode | 5-tuple parsed from Ethernet/IPv4/IPv6/TCP/UDP header bytes; short-header path returns "not parseable" not a panic | |
| `TestExportSampledFlowDispatch` | `flowexport/exporter_test.go` | a `FlowSample` reaches NF9/IPFIX sampled encoders for collectors marked sampled, sFlow encoders for sFlow collectors, and not the conntrack `flowRecord` path | |
| `TestConfigSampledRequiresSampling` | `flowexport/config_test.go` | `flow-source sampled` without a sampling interface fails validation (AC-1) | |
| `TestConfigFlowSourceDefault` | `flowexport/config_test.go` | absent `flow-source` defaults to conntrack; unknown value rejected | |
| `TestSampledMetricLabeled` | `flowexport/metrics_test.go` | sampled records counted distinctly from conntrack records | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| sampling rate (reused, spec 2) | 1-1000000 | 1000000 | 0 | 1000001 |
| samplingInterval IE 34 value (= rate) | 1-1000000 | 1000000 | 0 | 1000001 |
| sampled template ID | 256-65535 | 65535 | 255 | 65536 |
| flow-source enum | {sampled, conntrack} | conntrack | empty -> default conntrack | unknown -> reject |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-flow-export-sampled-ipfix` | `test/flow-export/sampled-ipfix.ci` | Configure IPFIX collector `flow-source sampled` + sampling on an interface; send packets; verify IPFIX data records arrive carrying IE 34 = rate | |
| `test-flow-export-sampled-netflow9` | `test/flow-export/sampled-netflow9.ci` | Same for NetFlow v9; verify SAMPLING_INTERVAL field 34 present and template referenced | |
| `test-flow-export-sampled-rate-independence` | `test/flow-export/sampled-rate-independence.ci` | Drive equal packet counts at low vs high flow churn; assert export datagram rate is churn-independent (AC-5) | |
| `test-flow-export-sampled-scale-show` | `test/flow-export/sampled-scale-show.ci` | `show flow-export` reports flow-source=sampled for the collector (AC-8) | |
| `test-flow-export-sampled-reload` | `test/flow-export/sampled-reload.ci` | Reload toggles a collector sampled<->conntrack; verify clean swap, no leaked workers (AC-12) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-sampled-ipfix-collector` | `test/flow-export/` (collector-side) | nfdump / nfcapd or a goflow2-style IPFIX collector | a real IPFIX collector accepts the sampled template + IE 34 and scales counts by the interval | |

Note: this is collector-interop (the peer is a flow collector, not a routing daemon), so it lives with the flow-export functional tests rather than `test/interop/scenarios/`. Justification: the wire peer is a passive UDP collector, not an interactive protocol session.

### Future (if deferring any tests)
- Exact unsampled per-flow at 100G (the deferred track below) -- its own future spec, not tested here.
- SCTP/TCP transport for IPFIX (UDP-only, inherited from spec 1).
- PSAMP selectorAlgorithm / samplerMode IEs (RFC 5476 full model) beyond the minimal samplingInterval -- minimal IE 34 ships first.

## Files to Modify
- `internal/component/flowexport/sampling_worker.go` - dispatch the psample stream to sampled NF9/IPFIX collectors, not only sFlow.
- `internal/component/flowexport/exporter.go` - sampled-flow-record dispatch alongside `ExportFlowSample` / `ExportFlows`.
- `internal/component/flowexport/config.go` - per-collector `flow-source` (sampled/conntrack) parse + validation (sampled requires a sampling interface).
- `internal/component/flowexport/yang/ze-flowexport-conf.yang` - `flow-source` leaf (enumeration sampled/conntrack) on the collector.
- `internal/component/flowexport/netflow9/flow_template.go`, `flow_data.go`, `flow_adapter.go`, `register.go` - sampled template variant (field 34) + data + factory.
- `internal/component/flowexport/ipfix/flow_template.go`, `flow_data.go`, `flow_adapter.go`, `ie.go`, `register.go` - sampled template variant (IE 34) + data + factory.
- `internal/component/flowexport/metrics.go` - sampled-record metric/label.
- `internal/component/cmd/show/flow_export.go` - surface flow-source in status output.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [x] | `internal/component/flowexport/yang/ze-flowexport-conf.yang` (add `flow-source` leaf) |
| YANG validation constraints | [x] | `flow-source` as `enumeration { sampled; conntrack; }` (native YANG enum); rate already constrained (spec 2) |
| YANG custom validators | [x] | Cross-field rule (sampled requires a sampling interface) enforced in `config.go Validate()` (not expressible as a single-leaf YANG constraint) |
| CLI commands/flags | [x] | `internal/component/cmd/show/flow_export.go` -- add flow-source to status; no new command verb |
| CLI grammar (action before identifier) | [ ] | No new CLI verb; `show flow-export [<collector>]` unchanged |
| Editor autocomplete | [x] | YANG enum -> automatic completion for `flow-source` |
| Functional test for new RPC/API | [x] | `test/flow-export/sampled-*.ci` |
| Pipe completeness | [x] | `show flow-export` already routes through pipe processing (spec 1); status gains a field, no new command |
| Env var registration | [ ] | No `environment/` leaves added |
| Doctor check for runtime dependencies | [x] | Reuse/extend spec-2 sampling readiness (psample module, CAP_NET_ADMIN) in `cmd/ze/doctor/` / health; gate when a sampled collector is configured |
| Prometheus counters/metrics | [x] | `metrics.go` -- sampled-record metric or `source` label |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- sampled NetFlow v9 / IPFIX at high rate |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- `flow-source` on a collector |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` -- `show flow-export` flow-source field |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` -- ze-show:flow-export status field |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/flow-export.md` -- update "Scale: conntrack vs sampling" so sampled NF9/IPFIX is documented as the 100G path |
| 7 | Wire format changed? | [x] | flow-export guide (sampled template + IE 34); `docs/architecture/wire/*` if a flow-export wire doc exists |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | Create `rfc/short/rfc5476.md` (PSAMP) via `/ze-rfc 5476`; reference RFC 7012 IE 34/35 |
| 10 | Test infrastructure changed? | [x] | `docs/functional-tests.md` if the rate-independence harness needs a churn generator |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- sampled IPFIX/NetFlow column |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- sampled flow-record path |
| 13 | Route metadata keys added/changed? | [ ] | N/A |
| 14 | Prometheus counters added/changed? | [x] | telemetry doc for the new sampled-record metric/label |

## Files to Create
- `internal/component/flowexport/netflow9/flow_sampled_template.go` (or extend `flow_template.go`) - sampled NetFlow v9 template (field 34).
- `internal/component/flowexport/ipfix/flow_sampled_template.go` (or extend `flow_template.go`) - sampled IPFIX template (IE 34).
- `internal/component/flowexport/sampling/decode.go` (or a shared header-decode helper) - 5-tuple extraction from sampled header bytes (reusing the `show capture` decode logic), if not already shared.
- `rfc/short/rfc5476.md` - PSAMP summary (via `/ze-rfc 5476`).
- `test/flow-export/sampled-ipfix.ci`
- `test/flow-export/sampled-netflow9.ci`
- `test/flow-export/sampled-rate-independence.ci`
- `test/flow-export/sampled-scale-show.ci`
- `test/flow-export/sampled-reload.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella + spec 1 + spec 2 |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - add the `flow-source` config leaf, the sampled-encoder factory hooks, and the worker dispatch branch as stubs; write failing wiring tests.
   - Tests: `TestSampledFlowEncoderWiring`, `TestSampledDispatchWiring`
   - Files: `config.go`, `schema/ze-flowexport-conf.yang`, `sampling_worker.go`, `exporter.go`, `netflow9/register.go`, `ipfix/register.go`
   - Verify: a sampled collector gets a sampled encoder assigned; the worker dispatch branch is reached; wiring tests fail because the encoder is a stub.
2. **Phase: RFC summary** - `/ze-rfc 5476` (PSAMP) so IPFIX sampled-record encoding has a referenced summary.
   - Files: `rfc/short/rfc5476.md`
3. **Phase: Header 5-tuple decode** - parse the sampled header into a 5-tuple, reusing `show capture` decode; short-header safety.
   - Tests: `TestSampledHeaderDecode`
   - Files: `sampling/decode.go` (or shared helper)
4. **Phase: IPFIX sampled template + data** - new template ID carrying IE 34; data encoding of the rate.
   - Tests: `TestIPFIXSampledTemplate`, `TestIPFIXSampledData`
   - Files: `ipfix/flow_template.go`/`flow_sampled_template.go`, `ipfix/flow_data.go`, `ipfix/ie.go`, `ipfix/flow_adapter.go`
5. **Phase: NetFlow v9 sampled template + data** - field 34; sequence per datagram preserved.
   - Tests: `TestNetflow9SampledTemplate`, `TestNetflow9SampledData`
   - Files: `netflow9/flow_template.go`/`flow_sampled_template.go`, `netflow9/flow_data.go`, `netflow9/flow_adapter.go`
6. **Phase: Exporter dispatch + worker routing** - route the psample stream to sampled NF9/IPFIX collectors.
   - Tests: `TestExportSampledFlowDispatch`
   - Files: `exporter.go`, `sampling_worker.go`
7. **Phase: Config + metrics + show** - `flow-source` validation, sampled metric/label, status field.
   - Tests: `TestConfigSampledRequiresSampling`, `TestConfigFlowSourceDefault`, `TestSampledMetricLabeled`
   - Files: `config.go`, `metrics.go`, `cmd/show/flow_export.go`
8. **Phase: Doctor / health** - gate sampling readiness when a sampled collector is configured.
   - Files: `cmd/ze/doctor/` / `health.go`
9. **Functional + interop tests** - the `.ci` files including rate-independence and collector interop.
10. **RFC refs** - `// RFC 7012 IE 34`, `// RFC 3954` field-type comments; PSAMP references.
11. **Full verification** - `make ze-verify`.
12. **Complete spec** - audit tables, learned summary, closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has implementation with file:line |
| Correctness | IE 34 / field 34 = configured rate; sampled template IDs distinct from 257/258; NF9 sequence per datagram; IPFIX sequence per data record |
| Naming | YANG `flow-source` kebab-case enum; JSON kebab-case; Prometheus `ze_flowexport_*` snake_case |
| Data flow | Sampled records flow through the FlowRecordEncoder factory + Sender; conntrack DeltaTracker invariants untouched |
| Rate independence | The sampled path does NO per-flow map insert; cost is per-sampled-packet (1-in-N), proven by AC-5 |
| Header safety | Short / truncated sampled header never panics (AC-11); bounds-checked slice access |
| Doctor checks | Sampling readiness gated when a sampled collector is configured |
| Rule: buffer-first | Sampled records written via WriteTo(buf, off); buffers from the shared pool |
| Rule: no-sprintf | No fmt.Sprintf in the per-sample encode path |
| Rule: no-layering | Reuses spec-2 psample reader + spec-1 sender/template machinery; no duplicate sampling or sender code |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| sampled IPFIX template carries IE 34 | `go test ./internal/component/flowexport/ipfix/... -run Sampled -v` |
| sampled NetFlow v9 template carries field 34 | `go test ./internal/component/flowexport/netflow9/... -run Sampled -v` |
| sampled dispatch reaches NF9/IPFIX, not conntrack | `go test ./internal/component/flowexport/... -run TestExportSampledFlowDispatch -v` |
| flow-source validation | `go test ./internal/component/flowexport/... -run TestConfigSampled -v` |
| rate independence | `test/flow-export/sampled-rate-independence.ci` passes |
| show reports flow-source | `test/flow-export/sampled-scale-show.ci` passes |
| sampled metric distinct | `grep -rn 'sampled' internal/component/flowexport/metrics.go` |
| RFC 5476 summary exists | `ls rfc/short/rfc5476.md` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Sampled header bytes come from the kernel but are attacker-influenced packet contents; bounds-check every offset before reading the 5-tuple (AC-11) |
| Resource exhaustion | Sampled path must NOT allocate per-flow state; one record per sampled packet, no map growth; rate is operator-bounded |
| Information disclosure | Sampled headers may contain payload bytes; trunc-size and the unencrypted-UDP warning from spec 1/2 still apply; document management-VLAN guidance |
| Reload safety | Toggling flow-source must stop the dropped path's workers (no leaked goroutine/socket) (AC-12) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Collector rejects sampled template | Capture with tcpdump; compare IE 34 / field 34 encoding against RFC 7012 / RFC 3954 |
| Rate-independence test shows churn dependence | The sampled path is touching per-flow state -- find and remove the map/lookup |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Alternatives / Future / Out of Scope -- Exact Unsampled Per-Flow at 100G

This track is **deferred** and explicitly NOT in scope for this spec. It is the
answer ONLY if exact, unsampled per-flow accounting is genuinely required at 100G
(the sampled path above covers visibility and volume estimation for the common
case). Documented here so a future requirement has a starting design.

→ Decision: defer exact unsampled per-flow at 100G pending a concrete requirement,
because sampling already delivers rate-independent 100G flow visibility and the
exact path requires substantial kernel-path and concurrency work with no current
demand. Each requirement below maps to a numbered bottleneck verified against
`conntrack/delta.go`, `conntrack_worker.go`, `reader_linux.go`, `destroy_linux.go`.

| # | Requirement | Bottleneck it removes | Verified against |
|---|-------------|-----------------------|------------------|
| F-1 | Shard the delta tracker (hash(5-tuple) -> N shards / per-CPU maps); the tombstone reclaim is a prerequisite cleanup | The single `sync.Mutex` serializing dump + destroy at ~1M lock ops/sec (bottleneck #2) | `conntrack/delta.go` `DeltaTracker.mu` |
| F-2 | Sample the conntrack flows themselves (track 1-in-N flows) to bound the map and the dump | Delta-tracker memory (~175 B/entry, ~875 MB at 1M flows/s x ~5s residency; Go maps never shrink) (bottleneck #1) | `conntrack/delta.go` SCALING RISK block |
| F-3 | Drop the full-table dump for purely event-driven (NEW + DESTROY multicast) with large `SO_RCVBUF` + multi-goroutine parse | Full-table `ConntrackTableList` serialization every active-timeout (bottleneck #3) AND the single-socket/single-goroutine destroy listener dropping events (ENOBUFS) at ~1M events/sec (bottleneck #4) | `reader_linux.go` `Dump()`, `destroy_linux.go` single `conn.Receive()` |
| F-4 | Bounded export with backpressure + explicit drop counters; never stall the datapath | Export bandwidth: 1M records/s x ~50 B ~ 400 Mbps plus a collector that can ingest it (bottleneck #5) | `exporter.go` `ExportFlows` fan-out under `e.mu` |
| F-5 | Reconsider the source: in-datapath aggregation (XDP/eBPF flow map or a VPP flow node) emitting pre-aggregated records, instead of userspace conntrack dumps | All of the above at once: moves aggregation off the userspace dump/event path | Whole conntrack path; aligns with the gokrazy/VPP appliance direction |

These are recorded as a future track only. No code, config, or test for F-1..F-5
is part of this spec. If picked up, F-1..F-5 become their own spec
(`spec-flow-export-4-exact-100g` or similar) with its own ACs and QEMU tests.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
Specific constraints to document in code:
- RFC 7012: samplingInterval is IE 34; samplingAlgorithm is IE 35. The sampled flow record advertises the interval so the collector can scale observed counts.
- RFC 3954: SAMPLING_INTERVAL is field type 34, SAMPLING_ALGORITHM is field type 35.
- RFC 3954: Sequence Number increments per export packet (datagram), Count = total records across FlowSets.
- RFC 7011: IPFIX sequence counts data records only; template Set ID = 2, data Set ID = template ID.
- RFC 5476 (PSAMP): the sampled-record model that justifies a distinct sampled template; once `rfc/short/rfc5476.md` exists, cite its relevant sections.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

- The sampling worker and `ExportFlowSample` already deliver a rate-independent stream; the entire primary scope is "feed that stream to NetFlow v9 / IPFIX as well, with the rate advertised", not new kernel plumbing.
- sFlow already solved self-describing rate (flow_sample.sampling_rate); NetFlow v9 / IPFIX need the equivalent field (34 / IE 34). That parity is the crux of the spec.
- The exact-100G track is genuinely a different problem (exact accounting), driven by the bottlenecks the conntrack code itself documents. Keeping it separate prevents over-engineering the common case.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| 100G flow visibility via sampling, not per-flow conntrack | Scale conntrack to 100G (sharding, flow-sampling, event-only dump) | Sampling cost is flow-rate-independent; conntrack cost scales with churn (its own SCALING RISK block says so). Sampling reuses shipped spec-2 plumbing. |
| Sampled NF9/IPFIX records carry IE 34 / field 34 | Export sampled records without advertising the rate | Without the interval the collector cannot reconstruct unsampled volume; the record would be misleading. sFlow already advertises rate; this gives NF9/IPFIX parity. |
| New template IDs for sampled records (not reuse 257/258) | Reuse the unsampled conntrack template IDs | A collector must distinguish sampled (scale-by-interval) from exact (conntrack) records; same template ID would conflate them. |
| Per-collector `flow-source` selector | Global mode; or auto-detect | Operators may run an exact-accounting collector and a high-rate sampled collector simultaneously; the source belongs per collector. |
| Defer exact unsampled 100G to a future spec | Implement F-1..F-5 now | No current requirement for exact unsampled accounting at 100G; sampling covers visibility. Avoids speculative kernel-path work. |

## Known Limitations
- Sampled records estimate volume (observed x interval); they are NOT exact per-flow byte/packet accounting. Exact accounting at modest churn stays on the conntrack path; exact accounting at 100G is the deferred F-1..F-5 track.
- The psample read path is a single goroutine + single socket (spec 2); the operator-set `rate` bounds its load. Multi-socket psample is part of the deferred scaling work, not this spec.
- Minimal PSAMP: only samplingInterval (IE 34 / field 34) is emitted; the fuller PSAMP selector/sampler IEs (RFC 5476) are future.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Sampled NF9/IPFIX export at high rate | functional test | `test/flow-export/sampled-ipfix.ci`, `sampled-netflow9.ci` |
| Rate-independence (the 100G claim) | functional test / benchmark | `test/flow-export/sampled-rate-independence.ci` (AC-5) |
| Self-describing rate (collector can scale) | unit + interop | IE 34 / field 34 tests + `NN-sampled-ipfix-collector` interop |
| Conntrack path unchanged | unit test | DeltaTracker invariant tests still pass; AC-7 |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (RFC 7012 IE 34/35, RFC 3954 field 34/35, RFC 5476)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-flow-export-3-sampled-scale.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-flow-export-3-sampled-scale.md`
