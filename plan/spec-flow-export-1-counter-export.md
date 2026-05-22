# Spec: Flow Export - Counter Export

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-flow-export-0-umbrella.md` - umbrella architecture
4. `rfc/short/sflow-v5.md` - sFlow v5 counter sample format (especially "Implementation Guidance: Counter-Only Exporter")
5. `rfc/short/rfc3954.md` - NetFlow v9 template + data FlowSet format
6. `rfc/short/rfc7011.md` - IPFIX message + template + data Set format
7. `rfc/short/rfc7012.md` - IPFIX Information Elements (counter IEs)
8. `internal/component/iface/rate.go` - rateTracker.collect() at line 101
9. `internal/component/iface/iface.go` - InterfaceStats at line 74, InterfaceRate at line 177

## Task

Implement counter-based flow export: sFlow v5 counter samples, NetFlow v9 interface
counter records, and IPFIX interface counter records. This is spec 1 of the
`spec-flow-export` set. It creates the `flowexport` component with shared
infrastructure (buffer pool, UDP sender, config) and the three protocol encoders
for interface counter data.

Counter export repackages data Ze already collects (interface bytes, packets,
errors, drops) into standard protocols that network monitoring tools consume
(LibreNMS, Observium, ntopng, Kentik, sFlow-RT, nfdump, Elastic).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component lifecycle and registration
  → Constraint: components register via init() with OnConfigure/OnStart/OnStop callbacks
- [ ] `ai/patterns/registration.md` - full registration pattern
  → Constraint: blank import in all.go triggers init(), make generate updates all.go
- [ ] `ai/patterns/config-option.md` - how config options wire through YANG
  → Constraint: config parsed from YANG tree in OnConfigure
- [ ] `ai/patterns/cli-command.md` - how CLI commands register
  → Constraint: show commands return map[string]any for pipe processing
- [ ] `plan/spec-flow-export-0-umbrella.md` - umbrella architecture decisions
  → Decision: shared component at internal/component/flowexport/
  → Decision: single collection point, buffer-first encoding, pre-computed templates
  → Constraint: raw counters (pre-baseline), not baseline-adjusted

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/sflow-v5.md` - sFlow v5 datagram format
  → Constraint: XDR encoding, big-endian, 4-byte aligned
  → Constraint: if_counters record is 88 bytes fixed (enterprise 0, format 1)
  → Constraint: counter_sample uses sflow_data_source (high 8 bits = type, low 24 = index)
  → Constraint: datagram header is 28 bytes (version=5, agent_address, sub_agent_id, sequence, uptime, num_samples)
- [ ] `rfc/short/rfc3954.md` - NetFlow v9 format
  → Constraint: export packet header is 20 bytes (version=9, count, sysUpTime, unix_secs, sequence, source_id)
  → Constraint: template FlowSet ID = 0; data FlowSet ID = template ID (256+)
  → Constraint: template must be sent before data that references it
  → Constraint: Count field = total records across all FlowSets
- [ ] `rfc/short/rfc7011.md` - IPFIX protocol
  → Constraint: message header is 16 bytes (version=0x000a, length, export_time, sequence, observation_domain)
  → Constraint: template Set ID = 2; data Set ID = template ID (256+)
  → Constraint: sequence counts data records only (not template records)
  → Constraint: Length field = total message length including header
- [ ] `rfc/short/rfc7012.md` - IPFIX Information Elements
  → Constraint: IE 1 = octetDeltaCount (u64), IE 2 = packetDeltaCount (u64)
  → Constraint: IE 10 = ingressInterface (u32), IE 14 = egressInterface (u32)
  → Constraint: reduced-size encoding permitted (u64 counter in 4 bytes if value fits)

**Key insights:**
- sFlow counter-only is a valid, complete implementation; collectors handle it natively
- sFlow uses XDR encoding (big-endian, padded to 4 bytes); NetFlow v9 and IPFIX use raw big-endian
- For counter-only export, NetFlow v9 and IPFIX templates are identical in concept (list of field IDs + sizes); the wire framing differs
- All three protocols batch multiple interface records per datagram
- sFlow if_counters needs 19 fields; Ze has 8 today; remaining 11 come from sysfs/netlink/procfs

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/rate.go` - rateTracker runs a 1s ticker calling collect(), which reads Backend.ListInterfaces(), computes rate deltas, stores snapshot under RWMutex, calls updateIfaceMetrics()
  → Constraint: collect() is goroutine-confined; only the ticker goroutine writes prev/prevAt/knownNames
  → Constraint: rates map is guarded by mu (RWMutex); snapshot() copies under RLock
- [ ] `internal/component/iface/iface.go:74` - InterfaceStats has RxBytes, RxPackets, RxErrors, RxDropped, TxBytes, TxPackets, TxErrors, TxDropped (all uint64)
  → Constraint: JSON keys are kebab-case; must not rename existing fields
- [ ] `internal/component/iface/iface.go:177` - InterfaceRate has Name, RxBps, TxBps, RxPps, TxPps, Stats pointer
  → Constraint: Stats is pointer to InterfaceStats; may be nil
- [ ] `internal/component/iface/iface.go:99` - InterfaceInfo has Name, Index (kernel ifIndex), Type, State, MTU, MAC, Addresses, Stats, ParentIndex, VlanID
  → Constraint: Index is the kernel ifIndex used by sFlow/NetFlow as the interface identifier
- [ ] `internal/component/iface/counters.go` - baselineStore.applyBaseline() subtracts baseline from raw stats for "clear counters" display
  → Constraint: flow export MUST use raw counters, NOT baseline-adjusted; read from ListInterfaces() before applyBaseline
- [ ] `internal/component/iface/dispatch.go:273` - ListRates() returns snapshot of all interface rates; GetRate(name) returns single
  → Constraint: these return baseline-adjusted stats; flow export needs a separate raw-stats path
- [ ] `internal/component/iface/backend.go` - Backend.ListInterfaces() returns []InterfaceInfo with raw kernel stats
  → Constraint: ListInterfaces() is the raw data source; called by collect() already every 1s
- [ ] `internal/component/host/nic_linux.go:83-84` - reads sysfs speed and duplex per NIC
  → Constraint: speed in Mbps (int), duplex as string ("full"/"half"/"unknown")
- [ ] `internal/component/host/inventory.go:220` - NICInfo.LinkSpeedMbps, NICInfo.Duplex
  → Constraint: host component collects this on demand, not every second
- [ ] `internal/component/telemetry/collector/netdev_linux.go` - reads /proc/net/dev via procfs.NetDev, has multicast/compressed fields
  → Constraint: separate collection loop from iface rate tracker; not shared

**Behavior to preserve:**
- Rate tracker 1s interval, Prometheus gauge names, InterfaceStats JSON field names
- Baseline-delta model for CLI display unaffected
- No additional latency on collect() (notification must be non-blocking)
- No new fields in InterfaceStats JSON output unless the consumer requests them

**Behavior to change:**
- Add ExtendedInterfaceStats type with additional sFlow/SNMP-compatible fields
- Add snapshot notification callback in rateTracker for flow export consumers
- Expose raw (pre-baseline) stats through a new function for flow export

## Data Flow (MANDATORY)

### Entry Point
- Config: YANG `flow-export` container parsed at startup and reload
- Data: kernel counters via `Backend.ListInterfaces()` in `rateTracker.collect()` every 1s

### Transformation Path
1. `Backend.ListInterfaces()` returns `[]InterfaceInfo` with raw kernel stats and ifIndex (existing)
2. `rateTracker.collect()` computes rate deltas, stores `map[string]InterfaceRate` (existing)
3. `collect()` calls registered notification function with `[]InterfaceInfo` snapshot (new)
4. `flowexport.Exporter` receives snapshot, checks polling interval timers per protocol
5. When timer fires: protocol encoder writes datagram into pooled buffer using `WriteTo(buf, off) int`
6. `sender.Send(buf[:n])` transmits via UDP socket to configured collector
7. Exporter increments Prometheus counters (datagrams, bytes, errors)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface -> flowexport | Callback with `[]InterfaceInfo` value slice (deep copy of stats) | [ ] |
| flowexport -> network | UDP sendto() with pre-encoded `[]byte` buffer | [ ] |
| config -> flowexport | YANG tree parsed into `FlowExportConfig` struct in OnConfigure | [ ] |
| CLI -> flowexport | RPC handler calls `flowexport.Status()` returning `map[string]any` | [ ] |

### Integration Points
- `iface.rateTracker` - register notification callback; called from collect() goroutine
- `iface.InterfaceInfo` - source of ifIndex, type, state, stats
- `metrics.Registry` - flowexport registers its own Prometheus counters
- `ze.EventBus` - NOT used for counter data (too expensive); direct callback instead

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (flowexport depends on iface types, not internals)
- [ ] No duplicated functionality (reuses existing counter collection)
- [ ] Zero-copy preserved where applicable (buffer pool, no intermediate allocations)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG `flow-export` config | -> | `flowexport.OnConfigure()` | `TestFlowExportConfigWiring` |
| `rateTracker.collect()` tick | -> | notification callback | `TestSnapshotNotification` |
| sFlow timer fires | -> | `sflow.WriteCounterDatagram()` | `TestSFlowCounterWiring` |
| NetFlow v9 timer fires | -> | `netflow9.WriteExportPacket()` | `TestNetflow9CounterWiring` |
| IPFIX timer fires | -> | `ipfix.WriteMessage()` | `TestIPFIXCounterWiring` |
| `show flow-export` RPC | -> | `flowexport.Status()` | `test-flow-export-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `flow-export { collector c1 { address 10.0.0.1; port 6343; protocol sflow; } }` | Component starts, opens UDP socket to 10.0.0.1:6343 |
| AC-2 | sFlow collector configured, 3 interfaces exist | Counter sample datagram arrives every polling-interval (default 20s) with 3 counters_sample records, each containing if_counters (88 bytes, 19 fields) |
| AC-3 | sFlow if_counters fields | ifIndex matches kernel index; ifSpeed matches sysfs; ifInOctets/ifOutOctets match raw kernel counters; ifInDiscards matches rx_dropped; ifInErrors matches rx_errors |
| AC-4 | NetFlow v9 collector configured | Template FlowSet (ID=0) sent on first export and every template-refresh seconds; data FlowSet follows with interface counter records |
| AC-5 | IPFIX collector configured | Template Set (ID=2) sent on first export and every template-refresh seconds; data Set follows with interface counter records using IANA IE IDs |
| AC-6 | Config reload adds second collector | New exporter starts without affecting existing; removing collector stops its exporter |
| AC-7 | `show flow-export` | Returns JSON: per-collector status (address, port, protocol, state), per-protocol stats (datagrams-sent, bytes-sent, errors, last-export-time) |
| AC-8 | Interface goes down then up | Next counter sample reflects updated ifStatus bits; no crash or stale data |
| AC-9 | Datagram exceeds MTU (many interfaces) | Records overflow into additional datagrams; sequence numbers increment per datagram |
| AC-10 | `ze doctor` with flow-export configured | Reports collector reachability status |
| AC-11 | Prometheus `/metrics` | Exposes `ze_flowexport_datagrams_total{collector,protocol}`, `ze_flowexport_errors_total`, `ze_flowexport_bytes_total` |
| AC-12 | No flow-export in config | Component does not start; no UDP socket opened; no CPU overhead |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSFlowDatagramHeader` | `flowexport/sflow/encoder_test.go` | 28-byte sFlow v5 header: version=5, agent IP, sub_agent_id, sequence, uptime, num_samples | |
| `TestSFlowCounterSample` | `flowexport/sflow/counter_test.go` | counters_sample record with source_id encoding ifIndex, sequence, one if_counters record | |
| `TestSFlowIfCounters` | `flowexport/sflow/counter_test.go` | 88-byte if_counters XDR encoding: all 19 fields big-endian, correct offsets | |
| `TestSFlowMultiInterface` | `flowexport/sflow/encoder_test.go` | Multiple counter samples packed in single datagram; overflow creates second datagram | |
| `TestNetflow9Header` | `flowexport/netflow9/encoder_test.go` | 20-byte header: version=9, count, sysUpTime, unix_secs, sequence, source_id | |
| `TestNetflow9Template` | `flowexport/netflow9/template_test.go` | Template FlowSet (ID=0) with counter fields: field types and lengths match RFC 3954 | |
| `TestNetflow9DataFlowSet` | `flowexport/netflow9/data_test.go` | Data FlowSet with template ID, interface counter records, padding | |
| `TestNetflow9TemplateRefresh` | `flowexport/netflow9/template_test.go` | Template re-sent after configured interval; not re-sent before | |
| `TestIPFIXHeader` | `flowexport/ipfix/encoder_test.go` | 16-byte header: version=0x000a, length, export_time, sequence, observation_domain_id | |
| `TestIPFIXTemplateSet` | `flowexport/ipfix/template_test.go` | Template Set (ID=2) with IANA IE field specifiers, correct lengths | |
| `TestIPFIXDataSet` | `flowexport/ipfix/data_test.go` | Data Set with template ID, counter records, padding | |
| `TestIPFIXSequenceCounting` | `flowexport/ipfix/encoder_test.go` | Sequence increments by data record count, not template records | |
| `TestBufferPoolGetPut` | `flowexport/sender_test.go` | Get returns 1400-byte buffer; Put returns to pool; no leak | |
| `TestSenderUDP` | `flowexport/sender_test.go` | Sends buffer to loopback UDP socket; received bytes match | |
| `TestExtendedStats` | `iface/extended_stats_test.go` | ExtendedInterfaceStats populated with speed, duplex, multicast, broadcast, promisc | |
| `TestNotificationCallback` | `iface/rate_test.go` | Registered callback receives []InterfaceInfo on each collect() tick | |
| `TestRawVsBaselineStats` | `iface/rate_test.go` | Notification delivers raw stats; ListRates still delivers baseline-adjusted | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| polling-interval | 1-3600 | 3600 | 0 | 3601 |
| template-refresh | 1-86400 | 86400 | 0 | 86401 |
| collector port | 1-65535 | 65535 | 0 | 65536 |
| sub-agent-id | 0-4294967295 | 4294967295 | N/A | 4294967296 |
| observation-domain | 0-4294967295 | 4294967295 | N/A | 4294967296 |
| sFlow agent-address | valid IPv4/IPv6 | any valid addr | empty string | invalid format |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-flow-export-sflow` | `test/flow-export/sflow.ci` | Configure sFlow collector on loopback, wait for polling-interval, verify datagram has correct version (5), sample count, if_counters with matching ifIndex | |
| `test-flow-export-netflow9` | `test/flow-export/netflow9.ci` | Configure NetFlow v9 collector, verify template FlowSet arrives first, then data FlowSet with matching template ID | |
| `test-flow-export-ipfix` | `test/flow-export/ipfix.ci` | Configure IPFIX collector, verify template Set (ID=2) arrives, then data Set with correct IE values | |
| `test-flow-export-show` | `test/flow-export/show.ci` | Configure collector, run `show flow-export`, verify JSON output has collector status and stats | |
| `test-flow-export-reload` | `test/flow-export/reload.ci` | Start with no collectors, add one via config reload, verify export starts; remove via reload, verify export stops | |
| `test-flow-export-multi` | `test/flow-export/multi.ci` | Configure two collectors (sFlow + IPFIX), verify both receive datagrams independently | |

### Future (if deferring any tests)
- VPP flowprobe delegation (requires VPP test environment)
- sFlow ethernet_counters record (ethtool stats not universally available)
- SCTP/TCP transport for IPFIX (UDP-only in spec 1)

## Files to Modify
- `internal/component/iface/iface.go` - add ExtendedInterfaceStats type
- `internal/component/iface/rate.go` - add notification callback registration and invocation in collect()
- `internal/component/iface/dispatch.go` - add ListRawStats() for pre-baseline counter access
- Netlink backend stats reader - populate extended stats fields (multicast, broadcast, speed, duplex, promisc, type, status)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/yang/modules/flow-export.yang` |
| CLI commands/flags | [x] | `internal/component/cmd/show/flow_export.go` |
| CLI grammar (action before identifier) | [x] | `show flow-export detail <name>` |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/flow-export/*.ci` |
| Doctor check for runtime dependencies | [x] | `cmd/ze/doctor/` - collector reachability |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - add flow export |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - flow-export stanza |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - show flow-export |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` - ze-show:flow-export |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/flow-export.md` - new page |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | RFC summaries already in rfc/short/ |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - flow export column |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - flowexport component |

## Files to Create
- `internal/component/flowexport/register.go` - component registration via init()
- `internal/component/flowexport/config.go` - YANG config parsing into FlowExportConfig
- `internal/component/flowexport/exporter.go` - Exporter lifecycle: start/stop/reload, timer management, snapshot dispatch
- `internal/component/flowexport/sender.go` - UDP sender with sync.Pool buffer management
- `internal/component/flowexport/snapshot.go` - CounterSnapshot type ([]InterfaceInfo value copy)
- `internal/component/flowexport/metrics.go` - Prometheus counters for export stats
- `internal/component/flowexport/sflow/encoder.go` - sFlow v5 datagram: header + sample array
- `internal/component/flowexport/sflow/counter.go` - counters_sample + if_counters XDR encoding
- `internal/component/flowexport/netflow9/encoder.go` - NetFlow v9 export packet: header + FlowSets
- `internal/component/flowexport/netflow9/template.go` - template FlowSet for interface counters
- `internal/component/flowexport/netflow9/data.go` - data FlowSet encoding
- `internal/component/flowexport/ipfix/encoder.go` - IPFIX message: header + Sets
- `internal/component/flowexport/ipfix/template.go` - template Set for interface counters
- `internal/component/flowexport/ipfix/data.go` - data Set encoding
- `internal/component/flowexport/ipfix/ie.go` - IANA IE ID constants
- `internal/component/cmd/show/flow_export.go` - show flow-export RPC handler
- `internal/yang/modules/flow-export.yang` - YANG schema
- `internal/component/iface/extended_stats.go` - ExtendedInterfaceStats and population logic
- `test/flow-export/sflow.ci` - sFlow functional test
- `test/flow-export/netflow9.ci` - NetFlow v9 functional test
- `test/flow-export/ipfix.ci` - IPFIX functional test
- `test/flow-export/show.ci` - show command functional test
- `test/flow-export/reload.ci` - config reload functional test
- `test/flow-export/multi.ci` - multi-collector functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - register component, create skeleton, write failing wiring tests
   - Tests: `TestFlowExportConfigWiring`, `TestSnapshotNotification`
   - Files: `register.go`, `exporter.go`, `config.go`, `sender.go`
   - Verify: component registers; OnConfigure receives config; notification callback fires; wiring tests fail on encoding stubs

2. **Phase: Extended stats** - add missing interface counter fields for sFlow compatibility
   - Tests: `TestExtendedStats`, `TestRawVsBaselineStats`
   - Files: `iface/extended_stats.go`, `iface/rate.go` (notification hook), netlink backend
   - Verify: ExtendedInterfaceStats has all 19 sFlow fields populated from kernel; raw stats delivered via callback

3. **Phase: Buffer pool + sender** - shared UDP send infrastructure
   - Tests: `TestBufferPoolGetPut`, `TestSenderUDP`
   - Files: `sender.go`
   - Verify: pool recycles 1400-byte buffers; UDP send to loopback works

4. **Phase: sFlow encoder** - counter sample datagram encoding
   - Tests: `TestSFlowDatagramHeader`, `TestSFlowCounterSample`, `TestSFlowIfCounters`, `TestSFlowMultiInterface`
   - Files: `sflow/encoder.go`, `sflow/counter.go`
   - Verify: byte-exact match against known-good sFlow v5 datagrams; multi-interface batching; overflow to second datagram

5. **Phase: NetFlow v9 encoder** - template + data FlowSet encoding
   - Tests: `TestNetflow9Header`, `TestNetflow9Template`, `TestNetflow9DataFlowSet`, `TestNetflow9TemplateRefresh`
   - Files: `netflow9/encoder.go`, `netflow9/template.go`, `netflow9/data.go`
   - Verify: template FlowSet has ID=0 and correct field types; data FlowSet references template ID; Count field accurate; template refresh timer works

6. **Phase: IPFIX encoder** - template + data Set encoding
   - Tests: `TestIPFIXHeader`, `TestIPFIXTemplateSet`, `TestIPFIXDataSet`, `TestIPFIXSequenceCounting`
   - Files: `ipfix/encoder.go`, `ipfix/template.go`, `ipfix/data.go`, `ipfix/ie.go`
   - Verify: message header has version=0x000a; template Set ID=2 with IANA IE field specifiers; sequence counts data records only

7. **Phase: Config + YANG** - configuration schema and parsing
   - Tests: config validation boundary tests
   - Files: `config.go`, `flow-export.yang`
   - Verify: valid config accepted; invalid values rejected with clear errors; reload adds/removes collectors

8. **Phase: CLI + metrics** - show command and Prometheus counters
   - Tests: `test-flow-export-show.ci`
   - Files: `flow_export.go` (show command), `metrics.go`
   - Verify: `show flow-export` returns JSON with all fields from AC-7; Prometheus counters exist

9. **Phase: Doctor check** - collector reachability
   - Tests: doctor check registration verified
   - Files: `cmd/ze/doctor/` integration
   - Verify: `ze doctor` reports collector status

10. **Functional tests** - end-to-end with loopback UDP receiver
11. **RFC refs** - add `// sFlow v5 Section X`, `// RFC 3954 Section X.Y`, `// RFC 7011 Section X.Y` comments
12. **Full verification** - `make ze-verify`
13. **Complete spec** - audit tables, learned summary, spec closure

### Encoding Details

#### sFlow v5 Counter Datagram Layout

```
Offset  Size  Field
0       4     version (5)
4       4     address_type (1=IPv4, 2=IPv6)
8       4/16  agent_address
12/24   4     sub_agent_id
16/28   4     sequence_number (per-agent, increments per datagram)
20/32   4     uptime (milliseconds since agent start)
24/36   4     num_samples
28/40   ...   sample_record[] (counter samples)
```

Per counter sample:
```
Offset  Size  Field
0       4     data_format (enterprise=0, format=2 -> 0x00000002)
4       4     sample_length (bytes following this field)
8       4     sequence_number (per-source, increments per sample)
12      4     source_id (type=0 in high 8 bits, ifIndex in low 24 bits)
16      4     num_records (1 for if_counters only)
20      4     record_data_format (enterprise=0, format=1 -> 0x00000001)
24      4     record_length (88)
28      88    if_counters (19 fields, see sFlow v5 spec)
```

Total per-interface: 116 bytes. A 1400-byte datagram fits ~11 interfaces (after 28-byte header).

#### NetFlow v9 Counter Template

Template ID 256 (first available), interface counter fields:

| Field Type | IE Name | Length |
|------------|---------|--------|
| 10 | IN_BYTES (octetDeltaCount mapped) | 8 |
| 14 | IN_PKTS (packetDeltaCount mapped) | 8 |
| 1 | IN_BYTES | 8 |
| 2 | IN_PKTS | 8 |
| 10 | INPUT_SNMP | 4 |
| 14 | OUTPUT_SNMP | 4 |

Note: for counter-only export (no per-flow data), the template defines
per-interface aggregate counters. The exact field set will be finalized
during implementation based on what collectors expect for interface counters.

#### IPFIX Counter Template

Template ID 256, using IANA IEs from RFC 7012:

| IE ID | Name | Length | Source |
|-------|------|--------|--------|
| 10 | ingressInterface | 4 | InterfaceInfo.Index |
| 1 | octetDeltaCount | 8 | delta of RxBytes + TxBytes |
| 2 | packetDeltaCount | 8 | delta of RxPackets + TxPackets |
| 82 | interfaceName | variable | InterfaceInfo.Name |
| 150 | flowStartSeconds | 4 | start of polling interval |
| 151 | flowEndSeconds | 4 | end of polling interval |

The IPFIX template supports additional IEs; the exact set will be refined
during implementation to match what common collectors (ntopng, nfdump, Elastic)
expect for interface counter records.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-12 has implementation with file:line |
| Correctness | sFlow XDR: big-endian, 4-byte aligned, if_counters = 88 bytes; NetFlow v9 Count = total records; IPFIX sequence = data records only |
| Naming | JSON keys kebab-case; YANG kebab-case; Go CamelCase; Prometheus snake_case with ze_ prefix |
| Data flow | Raw stats from callback, not baseline-adjusted; no pointer sharing across iface/flowexport boundary |
| CLI grammar | `show flow-export detail <name>` (action before identifier) |
| Doctor checks | Collector reachability check registered |
| Rule: buffer-first | All encoding uses WriteTo(buf, off) pattern; buffers from sync.Pool |
| Rule: no-sprintf | No fmt.Sprintf in encoding paths; binary.BigEndian.PutUintNN for wire encoding |
| Rule: no-layering | No duplicate counter reading (reuses rateTracker's existing ListInterfaces call) |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| flowexport component registers | `grep -rn 'RegisterComponent.*flowexport\|flowexport.*Register' internal/component/` |
| sFlow counter datagrams correct | `go test ./internal/component/flowexport/sflow/... -v` |
| NetFlow v9 template + data correct | `go test ./internal/component/flowexport/netflow9/... -v` |
| IPFIX template + data correct | `go test ./internal/component/flowexport/ipfix/... -v` |
| Extended interface stats populated | `go test ./internal/component/iface/... -run TestExtended -v` |
| Notification callback fires | `go test ./internal/component/iface/... -run TestNotification -v` |
| show flow-export works | `test/flow-export/show.ci` passes |
| Config reload works | `test/flow-export/reload.ci` passes |
| YANG schema validates | `make ze-lint` |
| Prometheus metrics registered | `grep -rn 'ze_flowexport' internal/component/flowexport/` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Collector address parsed via netip.ParseAddr (not net.ParseIP); port range 1-65535; intervals positive |
| UDP source spoofing | Outbound-only; exporter never processes inbound data from collector socket |
| Resource exhaustion | Buffer pool bounded by sync.Pool (GC reclaims); one goroutine per collector, not per interface |
| Information disclosure | Interface counters are not sensitive, but config should be access-controlled |
| Collector trust | No authentication on UDP (inherent protocol limitation); document management VLAN recommendation |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| XDR encoding mismatch | Compare byte-by-byte against sFlow v5 spec struct definitions |
| Collector rejects datagram | Capture with tcpdump, compare against RFC wire format diagrams |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

- sFlow v5 counter-only export is the simplest path: XDR is mechanical, all data
  is available, collectors support it universally
- The notification callback from rateTracker must be non-blocking; use a channel
  with TrySend or direct function call that the exporter goroutine processes
- NetFlow v9 and IPFIX template management is nearly identical in concept;
  the shared pattern is "pre-encode template bytes at config time, copy into
  datagram when refresh timer fires"
- sFlow uses raw monotonic counters (like SNMP); NetFlow v9/IPFIX can use either
  delta or total counters depending on IE choice (deltaCounter vs totalCounter semantic)

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

Specific RFC constraints to document in code:
- sFlow v5: sequence_number per-agent for datagrams, per-source for counter samples
- RFC 3954 Section 5.1: "The Sequence Number field ... is incremented per export packet"
- RFC 3954 Section 5.2: "Template ID ... in the range of 256 to 65535"
- RFC 7011 Section 3.1: "Sequence Number ... cumulative number of IPFIX Data Records"
- RFC 7011 Section 3.3.2: template must be sent before or in same message as referencing data

## Implementation Summary

### What Was Implemented
- [Pending]

### Bugs Found/Fixed
- [Pending]

### Documentation Updates
- [Pending]

### Deviations from Plan
- [Pending]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [Pending]

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
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-flow-export-1-counter-export.md`
- [ ] Summary included in commit
