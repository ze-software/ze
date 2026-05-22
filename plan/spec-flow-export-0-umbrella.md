# Spec: Flow Export Umbrella

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
3. `rfc/short/sflow-v5.md` - sFlow v5 counter/flow sample format
4. `rfc/short/rfc3954.md` - NetFlow v9 template and export format
5. `rfc/short/rfc7011.md` - IPFIX protocol
6. `rfc/short/rfc7012.md` - IPFIX Information Elements
7. `internal/component/iface/rate.go` - existing counter collection

## Task

Add flow export capability to Ze, supporting sFlow v5, NetFlow v9, and IPFIX.
Ze already collects interface counters and has conntrack accounting enabled,
but has no mechanism to export this data in standard flow protocols.

The work splits into two child specs:

| Spec | Scope | Data Source |
|------|-------|-------------|
| `spec-flow-export-1-counter-export.md` | sFlow counter samples, IPFIX/NetFlow v9 interface counter records | Existing `InterfaceStats` + extended kernel stats |
| `spec-flow-export-2-flow-records.md` | sFlow flow samples, IPFIX/NetFlow v9 per-flow records | Conntrack + eBPF packet sampling |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component registration pattern
  → Constraint: components register via init() in register.go, core discovers through registries
- [ ] `ai/patterns/registration.md` - how to register a new component
  → Constraint: plugin registry carries name, config roots, YANG, metrics callback

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/sflow-v5.md` - sFlow v5 datagram format, counter sample structure, XDR encoding
  → Constraint: counter-only export is valid; requires 19 if_counters fields per interface
- [ ] `rfc/short/rfc3954.md` - NetFlow v9 template system, field types, export packet format
  → Constraint: templates scoped per (exporter IP, source ID, template ID); must refresh periodically
- [ ] `rfc/short/rfc7011.md` - IPFIX wire protocol, template management, transport bindings
  → Constraint: IPFIX version 0x000a; template IDs 256+; enterprise IEs supported
- [ ] `rfc/short/rfc7012.md` - IPFIX Information Element definitions and data types
  → Constraint: IE IDs are IANA-assigned; reduced-size encoding permitted for specific types

**Key insights:**
- sFlow counter-only export is a complete, valid implementation (no packet sampling required)
- Ze's existing `InterfaceStats` covers 8 of 19 sFlow if_counters fields; the rest are available from sysfs/netlink
- Ze already enables `nf_conntrack_acct` (per-flow byte/packet counters in kernel)
- VPP has native `flowprobe` IPFIX export; VPP backend should delegate rather than reimplement
- All three protocols use UDP to collectors; shared sender infrastructure

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/rate.go` - rateTracker polls Backend.ListInterfaces() every 1s, computes per-interface deltas, updates Prometheus gauges
  → Constraint: collect() is called only from ticker goroutine; rates map guarded by RWMutex
- [ ] `internal/component/iface/iface.go` - InterfaceStats struct with 8 counters (rx/tx bytes/packets/errors/dropped), InterfaceRate with rates + stats pointer
  → Constraint: InterfaceInfo.Index is kernel ifIndex (used as sFlow/NetFlow ifIndex)
- [ ] `internal/component/iface/counters.go` - baselineStore for clear counters; applyBaseline subtracts from raw stats
  → Constraint: flow export needs RAW counters (pre-baseline), not baseline-adjusted values
- [ ] `internal/component/iface/backend.go` - Backend interface with GetStats/ListInterfaces
  → Constraint: VPP backend returns different stat granularity than netlink
- [ ] `internal/component/host/nic_linux.go` - reads link speed (sysfs /speed) and duplex (sysfs /duplex)
  → Constraint: speed/duplex already collected but in host component, not iface
- [ ] `internal/component/config/system/conntrack.go` - enables nf_conntrack_acct when accounting=true
  → Constraint: per-flow byte/packet counters available via /proc/net/nf_conntrack or netlink
- [ ] `internal/component/telemetry/collector/netdev_linux.go` - procfs.NetDev has multicast/broadcast counters
  → Constraint: telemetry collector reads /proc/net/dev independently from iface rate tracker

**Behavior to preserve:**
- Rate tracker 1s polling interval and Prometheus metric names unchanged
- InterfaceStats JSON field names (kebab-case) unchanged for existing 8 fields
- Baseline-delta model for `clear interface counters` unaffected
- No additional latency on the rate tracker's collect() hot path

**Behavior to change:**
- Extend InterfaceStats with multicast/broadcast/speed/duplex/promisc/type/status fields
- Add notification hook in rateTracker.collect() for flow export consumers
- New component `flowexport` in component registry

## Data Flow (MANDATORY)

### Entry Point
- Config: YANG `flow-export` container parsed at startup and reload
- Data: kernel counters via existing `iface.rateTracker.collect()` every 1s

### Transformation Path
1. Kernel counters read by `Backend.ListInterfaces()` (existing)
2. `rateTracker.collect()` computes rates, stores snapshot (existing)
3. `collect()` calls `flowexport.NotifySnapshot()` with counter snapshot (new)
4. Each enabled exporter (sflow/netflow9/ipfix) encodes into pooled buffer
5. `sender.Send()` transmits UDP datagram to configured collector

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface -> flowexport | Function call with snapshot value type (no pointers across boundary) | [ ] |
| flowexport -> network | UDP sendto with pre-encoded buffer | [ ] |
| config -> flowexport | YANG tree parsed into FlowExportConfig struct | [ ] |

### Integration Points
- `iface.rateTracker.collect()` - hook point for snapshot notification
- `iface.InterfaceStats` - extended with additional counter fields
- `host.NICInfo` - source for speed/duplex (or move reading into iface backend)
- `metrics.Registry` - export component registers its own Prometheus counters
- `ze.EventBus` - not used for data path (too expensive); direct function call instead

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG `flow-export` config | -> | `flowexport.OnConfigure()` | `TestFlowExportConfigWiring` |
| `rateTracker.collect()` tick | -> | `flowexport.NotifySnapshot()` | `TestSnapshotNotification` |
| `show flow-export` CLI | -> | `flowexport.ShowStatus()` | `test-flow-export-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `flow-export` stanza in config | Component starts, opens UDP socket to collector |
| AC-2 | Config reload adds/removes collector | Exporter added/removed without daemon restart |
| AC-3 | sFlow collector configured, interfaces exist | Counter sample datagrams arrive at collector every polling-interval |
| AC-4 | NetFlow v9 collector configured | Template FlowSet sent on startup and every template-refresh seconds |
| AC-5 | IPFIX collector configured | Template Set sent, Data Sets follow with interface counter records |
| AC-6 | `show flow-export` | Displays active exporters, collector addresses, datagrams/bytes sent, errors |
| AC-7 | `ze doctor` with flow-export configured | Checks collector IP reachability |
| AC-8 | Prometheus `/metrics` | `ze_flowexport_datagrams_total`, `ze_flowexport_errors_total`, `ze_flowexport_bytes_total` |
| AC-9 | VPP backend with IPFIX configured | Delegates to VPP flowprobe plugin via GoVPP binapi |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSFlowCounterEncode` | `internal/component/flowexport/sflow/encoder_test.go` | sFlow v5 datagram with counter samples matches expected bytes | |
| `TestNetflow9TemplateEncode` | `internal/component/flowexport/netflow9/template_test.go` | Template FlowSet encoding matches RFC 3954 wire format | |
| `TestNetflow9DataEncode` | `internal/component/flowexport/netflow9/data_test.go` | Data FlowSet with interface counters matches expected layout | |
| `TestIPFIXTemplateEncode` | `internal/component/flowexport/ipfix/template_test.go` | IPFIX Template Set encoding matches RFC 7011 wire format | |
| `TestIPFIXDataEncode` | `internal/component/flowexport/ipfix/data_test.go` | IPFIX Data Set with counter IEs matches expected bytes | |
| `TestBufferPoolReuse` | `internal/component/flowexport/sender_test.go` | Buffer get/put cycle, no leaks | |
| `TestBatchPacking` | `internal/component/flowexport/sender_test.go` | Multiple samples fit in one datagram up to MTU | |
| `TestExtendedStats` | `internal/component/iface/iface_test.go` | Extended InterfaceStats fields populated from procfs/sysfs | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| polling-interval | 1-3600 | 3600 | 0 | 3601 |
| template-refresh | 1-86400 | 86400 | 0 | 86401 |
| collector port | 1-65535 | 65535 | 0 | 65536 |
| sub-agent-id | 0-4294967295 | 4294967295 | N/A | 4294967296 |
| observation-domain | 0-4294967295 | 4294967295 | N/A | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-flow-export-sflow` | `test/flow-export/sflow.ci` | Configure sFlow collector, verify counter datagrams arrive with correct ifIndex and counters | |
| `test-flow-export-netflow9` | `test/flow-export/netflow9.ci` | Configure NetFlow v9, verify template then data FlowSets arrive | |
| `test-flow-export-ipfix` | `test/flow-export/ipfix.ci` | Configure IPFIX, verify template then data Sets arrive | |
| `test-flow-export-show` | `test/flow-export/show.ci` | `show flow-export` returns JSON with collector status and counters | |
| `test-flow-export-reload` | `test/flow-export/reload.ci` | Add collector via config reload, verify export starts | |

### Future (if deferring any tests)
- VPP flowprobe integration test (requires VPP test environment, deferred to spec 2 VPP work)
- sFlow flow samples (spec 2)
- NetFlow v9/IPFIX per-flow records from conntrack (spec 2)

## Files to Modify
- `internal/component/iface/iface.go` - extend InterfaceStats with multicast/broadcast/speed/duplex/status/type/promisc
- `internal/component/iface/rate.go` - add snapshot notification hook in collect()
- `internal/component/iface/dispatch.go` - expose raw (pre-baseline) stats for flow export

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
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | N/A - this is a component, not a plugin |
| 6 | Has a user guide page? | [x] | `docs/guide/flow-export.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | RFC summaries already in `rfc/short/` |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - flow export support |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - new component |

## Files to Create
- `internal/component/flowexport/register.go` - component registration
- `internal/component/flowexport/config.go` - YANG config parsing
- `internal/component/flowexport/exporter.go` - shared exporter lifecycle
- `internal/component/flowexport/sender.go` - UDP sender with buffer pool
- `internal/component/flowexport/snapshot.go` - counter snapshot types
- `internal/component/flowexport/sflow/encoder.go` - sFlow v5 datagram encoding
- `internal/component/flowexport/sflow/counter.go` - counter sample construction
- `internal/component/flowexport/netflow9/encoder.go` - NetFlow v9 packet encoding
- `internal/component/flowexport/netflow9/template.go` - template FlowSet management
- `internal/component/flowexport/netflow9/data.go` - data FlowSet encoding
- `internal/component/flowexport/ipfix/encoder.go` - IPFIX message encoding
- `internal/component/flowexport/ipfix/template.go` - template management
- `internal/component/flowexport/ipfix/data.go` - data Set encoding
- `internal/component/flowexport/ipfix/ie.go` - Information Element constants
- `internal/component/cmd/show/flow_export.go` - show command
- `internal/yang/modules/flow-export.yang` - YANG schema
- `test/flow-export/*.ci` - functional tests

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
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
| 14. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - register component, write failing wiring tests
   - Tests: `TestFlowExportConfigWiring`, `TestSnapshotNotification`
   - Files: `register.go`, `exporter.go`, `config.go`
   - Verify: component registers and receives config; wiring test fails because encoding is a stub

2. **Phase: Extended stats** - add missing fields to InterfaceStats
   - Tests: `TestExtendedStats`
   - Files: `iface/iface.go`, netlink backend stats reader
   - Verify: multicast/broadcast/speed/duplex/promisc populated from kernel

3. **Phase: Shared sender** - buffer pool + UDP sender
   - Tests: `TestBufferPoolReuse`, `TestBatchPacking`
   - Files: `sender.go`, `snapshot.go`
   - Verify: buffers pooled, multiple records pack into single datagram

4. **Phase: sFlow encoder** - counter sample datagram encoding
   - Tests: `TestSFlowCounterEncode`
   - Files: `sflow/encoder.go`, `sflow/counter.go`
   - Verify: XDR-encoded datagram matches expected bytes for known input

5. **Phase: NetFlow v9 encoder** - template + data FlowSet encoding
   - Tests: `TestNetflow9TemplateEncode`, `TestNetflow9DataEncode`
   - Files: `netflow9/encoder.go`, `netflow9/template.go`, `netflow9/data.go`
   - Verify: template and data FlowSets match RFC 3954 wire format

6. **Phase: IPFIX encoder** - template + data Set encoding
   - Tests: `TestIPFIXTemplateEncode`, `TestIPFIXDataEncode`
   - Files: `ipfix/encoder.go`, `ipfix/template.go`, `ipfix/data.go`, `ipfix/ie.go`
   - Verify: template and data Sets match RFC 7011 wire format

7. **Phase: CLI + YANG** - show command and config schema
   - Tests: `test-flow-export-show.ci`
   - Files: `flow_export.go`, `flow-export.yang`
   - Verify: `show flow-export` returns JSON with collector status

8. **Functional tests** - end-to-end with mock collector
9. **RFC refs** - Add `// RFC 3954 Section X.Y` / `// RFC 7011 Section X.Y` / `// sFlow v5 Section X` comments
10. **Full verification** - `make ze-verify`
11. **Complete spec** - Fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | sFlow XDR encoding is big-endian, 4-byte aligned; NetFlow v9 Count field correct; IPFIX sequence counts data records only |
| Naming | JSON keys use kebab-case; YANG uses kebab-case; Go types CamelCase |
| Data flow | Snapshot passes as value type across iface->flowexport boundary; no pointers shared |
| CLI grammar | `show flow-export detail <name>` (action before identifier) |
| Doctor checks | Collector reachability check registered |
| Rule: buffer-first | All encoding uses WriteTo(buf, off) pattern, no append/make in hot path |
| Rule: no-sprintf | No fmt.Sprintf in encoding paths; use textbuf or binary.BigEndian |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| flowexport component registers | `grep -rn 'flowexport' internal/component/` |
| sFlow counter datagrams encode correctly | `go test ./internal/component/flowexport/sflow/...` |
| NetFlow v9 template + data encode correctly | `go test ./internal/component/flowexport/netflow9/...` |
| IPFIX template + data encode correctly | `go test ./internal/component/flowexport/ipfix/...` |
| show flow-export works | `test/flow-export/show.ci` passes |
| YANG schema validates | `make ze-lint` |
| Extended InterfaceStats populated | `go test ./internal/component/iface/... -run TestExtended` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Collector address must be valid IP; port in range; polling-interval positive |
| UDP amplification | Flow export is outbound-only; no inbound processing of collector responses |
| Information disclosure | Counter data is not sensitive, but collector address config should be access-controlled |
| Resource exhaustion | Buffer pool bounded; datagram size capped at MTU; one goroutine per protocol, not per interface |
| Untrusted network | sFlow/NetFlow/IPFIX datagrams are unencrypted UDP; document that dedicated management VLAN is recommended |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Performance Architecture (Detail)

### Single Collection, Multiple Consumers

The rate tracker's `collect()` method (rate.go:101) is the sole kernel reader.
Today it feeds Prometheus. The design adds a second consumer without a second
poll loop. After computing rates, `collect()` calls a registered notification
function. The notification passes a snapshot (value type, not pointer) so
exporters work on an immutable copy while the next collection proceeds.

### Buffer-First Datagram Encoding

Following `ai/rules/buffer-first.md`:

| Protocol | Header size | Record size (counter) | Encoding pattern |
|----------|-------------|----------------------|------------------|
| sFlow v5 | 28 bytes | ~120 bytes/interface (if_counters) | XDR big-endian, skip-and-backfill for sample count |
| NetFlow v9 | 20 bytes | ~52 bytes/interface (template-dependent) | binary.BigEndian into offset, skip-and-backfill for Count |
| IPFIX | 16 bytes | ~52 bytes/interface (template-dependent) | binary.BigEndian into offset, skip-and-backfill for Length |

All encode via `WriteTo(buf []byte, off int) int`. Buffer from `sync.Pool` of
1400-byte slices (safe UDP payload below typical MTU). Get, encode, send, put.

### Pre-Computed Templates

NetFlow v9 and IPFIX templates describe record layout. Templates change only
on config reload. At config time:
- Encode template FlowSet/Set once into `[]byte`
- Build field offset table (byte offset in data record for each counter)
- Per export cycle: `copy()` template bytes into datagram when refresh fires,
  then write data records by direct binary.BigEndian writes at known offsets

### Batched Packing

Multiple counter samples (sFlow) or data records (NetFlow v9/IPFIX) are packed
into a single UDP datagram. The encoder tracks remaining space and flushes when
the next record would overflow. This minimizes syscalls: a 100-interface device
produces ~3 sFlow datagrams per poll, not 100.

### Raw vs Baseline-Adjusted Counters

sFlow counter samples and IPFIX/NetFlow v9 interface counters report raw
monotonic values (matching SNMP ifTable semantics). Flow export MUST use raw
kernel counters, NOT baseline-adjusted values from `applyBaseline()`. The
snapshot notification will carry raw stats (from `Backend.ListInterfaces()`
before baseline subtraction).

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

- Ze's rate tracker architecture (single goroutine, snapshot-and-swap) maps
  naturally to flow export: same snapshot, multiple consumers
- Conntrack accounting is already enabled, which means per-flow data for
  NetFlow/IPFIX is available without eBPF (spec 2 can start with conntrack)
- The 8-field InterfaceStats gap vs sFlow's 19-field if_counters is fillable
  from existing kernel data sources already read elsewhere in Ze

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
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
- [ ] Write learned summary to `plan/learned/NNN-flow-export-0-umbrella.md`
- [ ] Summary included in commit
