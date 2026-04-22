# Spec: l2tp-10 -- Prometheus Metrics Exposure

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-l2tp-9-observer, spec-l2tp-7-subsystem, spec-l2tp-8-plugins |
| Phase | 7/7 |
| Updated | 2026-04-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-l2tp-0-umbrella.md` -- parent umbrella, "Prometheus metrics" in-scope bullet
4. `plan/spec-l2tp-9-observer.md` -- observer state that feeds metrics
5. `internal/core/metrics/metrics.go`, `prometheus.go`, `server.go` -- registry + backend
6. `internal/plugins/bfd/metrics.go` -- closest precedent (BFD sessions + echo RTT)
7. `internal/component/vpp/telemetry.go` -- subsystem-adjacent telemetry precedent

## Task

Expose L2TP, PPPoE, and RADIUS metrics on ze's Prometheus endpoint, preserving
the semantics of the existing `nas_*` metrics currently in production but
renaming them under ze's `ze_<component>_*` convention. Metrics are registered
by the L2TP subsystem, PPPoE plugin (when present), and RADIUS plugin. Per-session
counter values come from the observer's ring buffers and from kernel socket
accounting.

This spec executes the "Prometheus metrics" in-scope bullet already promised by
`spec-l2tp-0-umbrella.md` and replaces the existing standalone `nas_*` exporter.

## Design Decisions (agreed with user, 2026-04-17 + 2026-04-22)

| # | Decision |
|---|----------|
| D2 | Adopt ze convention: `ze_l2tp_*`, `ze_pppoe_*`, `ze_radius_*`. Drop redundant `target` label. Drop `nas_scrape_duration_seconds` (exporter self-timing; not a real signal). Lean on ze process metrics for build-info / up / uptime / memory / cpu. |
| D3 | Metric emitters live in `internal/component/l2tp/metrics.go` (L2TP + observer-fed metrics) and in the RADIUS plugin / future PPPoE component. Precedent: `internal/plugins/bfd/metrics.go`. |
| D4 | Registry injection: L2TP subsystem calls `registry.GetMetricsRegistry()` in `Subsystem.Start()`, type-asserts to `metrics.Registry`. Same approach as BFD plugin's `OnStarted`. No `ze.Subsystem` interface change. |
| D5 | Per-session byte/packet counters: periodic `statsPoller` calls `iface.GetStats(pppN)` for each active session. Default 30s interval, configurable via `ze.l2tp.metrics.poll-interval` env var. Caches results in Prometheus counters (not per-scrape kernel reads). |
| D6 | RADIUS metrics ownership: `l2tpauthradius` plugin owns `ze_radius_*` family. Wired via existing `ConfigureMetrics` callback in `registry.Registration`. Atomic pointer pattern matching BFD/VPP. |
| D7 | Session state gauge: single numeric gauge `ze_l2tp_session_state`. Values: idle=0, wait-connect=1, wait-reply=2, established=3, closing=4. Matches FSM enum order in `session_fsm.go`. Avoids cardinality multiplication from state-label pattern. |
| D8 | LCP echo RTT histogram buckets (seconds): 1-2-5 per decade: `0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1.0, 2.0, 5.0`. Twelve buckets, 1ms floor, 5s ceiling. |
| D9 | Stale per-session series deletion: piggyback on observer's `SessionDown` handler in `wireObserverSubscriptions`. Calls `Delete(labelValues...)` on all per-session metric vecs. One subscription, no ordering ambiguity. |
| D10 | Stats poller structure: separate `statsPoller` struct in `metrics.go`, owned by Subsystem alongside Observer. Gets active session list from reactor snapshots. Started/stopped in `Subsystem.Start()`/`Stop()`. |

## Scope

### In Scope

| Area | Description |
|------|-------------|
| `ze_l2tp_*` metric family | Aggregate session counts, session-keyed counters, tunnel counts, observer-derived CQM metrics |
| `ze_pppoe_*` metric family | PADI/PADO/PADR/PADS counters, filtered count. Home: future PPPoE component; this spec reserves names and schema |
| `ze_radius_*` metric family | Server up/down, auth/acct/interim counters labeled by server identity. Home: RADIUS plugin |
| Registration wiring | Each family registered by its owning component at subsystem/plugin Start via `internal/core/metrics/` Registry |
| Preservation of monitoring semantics | Every currently-produced `nas_*` metric has a `ze_*` equivalent, or is explicitly dropped as low-value (documented) |

### Proposed Name Mapping (to be finalised during DESIGN phase)

| Existing `nas_*` | Proposed `ze_*` | Notes |
|---|---|---|
| `nas_sessions_active{target}` | `ze_l2tp_sessions_active` | Drop `target` |
| `nas_sessions_starting{target}` | `ze_l2tp_sessions_starting` | |
| `nas_sessions_finishing{target}` | `ze_l2tp_sessions_finishing` | |
| `nas_session_rx_bytes_total{...}` | `ze_l2tp_session_rx_bytes_total{sid,ifname,username,ip,caller_id}` | Drop `target`, `service`; Prometheus label-name convention keeps snake_case |
| `nas_session_tx_bytes_total{...}` | `ze_l2tp_session_tx_bytes_total{...}` | |
| `nas_session_rx_packets_total{...}` | `ze_l2tp_session_rx_packets_total{...}` | |
| `nas_session_tx_packets_total{...}` | `ze_l2tp_session_tx_packets_total{...}` | |
| `nas_session_uptime_seconds{...}` | `ze_l2tp_session_uptime_seconds{...}` | |
| `nas_session_state{...}` | `ze_l2tp_session_state{...}` | State enum values documented |
| `nas_pppoe_padi_received_total{target}` | `ze_pppoe_padi_received_total` | Separate family; drops misleading `target=l2tp` |
| `nas_pppoe_pado_sent_total` | `ze_pppoe_pado_sent_total` | |
| `nas_pppoe_padr_received_total` | `ze_pppoe_padr_received_total` | |
| `nas_pppoe_pads_sent_total` | `ze_pppoe_pads_sent_total` | |
| `nas_pppoe_filtered_total` | `ze_pppoe_filtered_total` | |
| `nas_radius_up{server_id,server_addr}` | `ze_radius_up{server_id,server_addr}` | |
| `nas_radius_auth_sent_total{...}` | `ze_radius_auth_sent_total{...}` | |
| `nas_radius_acct_sent_total{...}` | `ze_radius_acct_sent_total{...}` | |
| `nas_radius_interim_sent_total{...}` | `ze_radius_interim_sent_total{...}` | |
| `nas_build_info`, `nas_up`, `nas_uptime_seconds` | Ze process metrics | Existing ze process-level exposure covers these |
| `nas_cpu_percent`, `nas_memory_rss_bytes`, `nas_memory_virtual_bytes` | Go runtime metrics via Prometheus client default registry | Standard, no work |
| `nas_scrape_duration_seconds{stage}` | Dropped | Exporter self-timing; Prometheus server already measures scrape duration |

### New Metrics (from CQM, not in existing exporter)

| Metric | Type | Notes |
|---|---|---|
| `ze_l2tp_lcp_echo_rtt_seconds` | HistogramVec | Labeled by `username` (login-keyed). 1-2-5 buckets: 0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1.0, 2.0, 5.0 |
| `ze_l2tp_lcp_echo_loss_ratio` | GaugeVec | Per-login current 100s bucket loss ratio |
| `ze_l2tp_bucket_state` | GaugeVec | Per-login current bucket state enum (established/negotiating/down) |

### Out of Scope

| Area | Location |
|------|----------|
| Observer ring buffer mechanism | `spec-l2tp-9-observer` |
| Web UI, CSV/JSON/SSE feeds, uPlot graph | `spec-l2tp-11-web` |
| Per-session CSV export | `spec-l2tp-11-web` |
| Grafana dashboard updates | Operator-side task, documented |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- metrics registry usage
  -> Constraint: `metrics.Registry` interface; Counter/Gauge/Histogram + Vec variants; `Delete()` for cardinality cleanup
- [ ] `plan/spec-l2tp-9-observer.md` -- state that feeds CQM-derived metrics
  -> Constraint: observer owns per-session event rings + per-login CQM sample rings; `wireObserverSubscriptions` is the wiring point
- [ ] `internal/plugins/bfd/metrics.go` -- precedent for subsystem-registered metric families
  -> Constraint: atomic pointer pattern; `bindMetricsRegistry(reg)` called from OnStarted; `metricsHook` interface for event-driven increments
- [ ] `internal/component/vpp/telemetry.go` -- subsystem-adjacent telemetry precedent
  -> Constraint: `statsPoller` pattern for periodic kernel reads; delta computation for counters
- [ ] `internal/component/iface/dispatch.go:180` -- `GetStats(ifaceName)` returns `*InterfaceStats`
  -> Constraint: existing kernel counter reader via netlink; returns RxBytes/RxPackets/TxBytes/TxPackets/errors/drops
- [ ] `internal/component/plugin/registry/registry.go:286` -- `GetMetricsRegistry()` global getter
  -> Constraint: returns `any`; type-assert to `metrics.Registry`

### RFC Summaries
- [ ] None directly; this spec is about exposure shape, not protocol behavior.

**Key insights:**
- `iface.GetStats(pppN)` already reads kernel counters -- no new kernel infrastructure needed for per-session byte/packet metrics
- BFD wires metrics via `registry.GetMetricsRegistry()` in OnStarted, not via ConfigureMetrics -- same late-binding approach works for L2TP subsystem Start
- `metrics.CounterVec.Delete()` / `GaugeVec.Delete()` / `HistogramVec.Delete()` exist for cardinality cleanup on session close
- Observer's `wireObserverSubscriptions` SessionDown handler is the single point for cleanup (event ring release + metric series deletion)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/metrics/metrics.go` -- Registry interface: Counter/Gauge/Histogram + Vec variants with Delete()
- [ ] `internal/core/metrics/prometheus.go`, `server.go` -- Prometheus backend, HTTP endpoint
- [ ] `internal/plugins/bfd/metrics.go` -- atomic pointer, bindMetricsRegistry, metricsHook, refreshSessionsGauge
- [ ] `internal/component/vpp/telemetry.go` -- statsPoller: periodic poll, delta computation, newVPPMetrics
- [ ] `internal/component/iface/dispatch.go:180` -- `GetStats(ifaceName)` reads kernel counters via backend
- [ ] `internal/component/iface/iface.go:73` -- `InterfaceStats{RxBytes,RxPackets,TxBytes,TxPackets,...}`
- [ ] `internal/component/l2tp/observer.go` -- Observer, wireObserverSubscriptions, SessionDown releases rings
- [ ] `internal/component/l2tp/cqm.go` -- CQMBucket{Start,State,EchoCount,MinRTT,MaxRTT,SumRTT}, BucketInterval=100s
- [ ] `internal/component/l2tp/snapshot.go` -- Snapshot/SessionSnapshot with pppInterface, username, assignedAddr
- [ ] `internal/component/l2tp/subsystem.go` -- Start wires observer, reactors; Stop tears down
- [ ] `internal/component/l2tp/session.go:119` -- `pppInterface` field on L2TPSession
- [ ] `internal/component/plugin/registry/registry.go:286` -- `GetMetricsRegistry() any`

**Behavior to preserve:** every existing `nas_*` metric name has a `ze_*` equivalent unless explicitly dropped in this spec with a stated reason.

**Behavior to change:** metric prefix; drop `target` label; drop `service` label where always `l2tp`; drop `scrape_duration_seconds`.

## Data Flow (MANDATORY)

### Entry Points
- Observer ring buffer reads (for CQM-derived metrics)
- Kernel counters via `pppox_linux.go` / `kernel_linux.go` (for per-session byte/packet counters)
- RADIUS plugin send/receive call sites (for RADIUS counters)
- PPPoE component dispatch (for PPPoE counters; future)

### Transformation Path
1. On each Prometheus scrape, the registry serializes current gauge/counter values
2. For session-keyed counters, values are cached from the kernel counter reads triggered by observer events (not per-scrape kernel calls)
3. For CQM histograms, observe calls happen when bucket boundaries land (not per-scrape)

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| Observer to metrics | Direct read of ring-buffer head fields; no channel |
| Kernel to metrics | Existing kernel counter read path in `spec-l2tp-5-kernel` |
| Plugin to core metrics | `core/metrics.Registry` injected at plugin construction |

### Integration Points
- `spec-l2tp-9-observer` ring buffer read API supplies CQM-derived metric values
- `spec-l2tp-8-plugins` RADIUS plugin registers its own `ze_radius_*` family at plugin Start
- `internal/core/metrics/` Registry is the injection point for all three families
- Existing ze process metrics cover build-info / up / uptime / memory / cpu; this spec does not re-implement them

### Architectural Verification
- [ ] Per-session series deleted on session close via observer SessionDown handler (no stale cardinality)
- [ ] No per-scrape kernel reads (statsPoller caches at 30s intervals; Prometheus reads cached counters)
- [ ] Each metric family registered by its owning component: L2TP in subsystem Start, RADIUS in plugin ConfigureMetrics
- [ ] statsPoller uses `iface.GetStats(pppN)` -- reuses existing kernel counter infrastructure, no new netlink code
- [ ] Observer CQM ring reads happen at bucket boundaries (100s), not per-scrape

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Session reaches Established | → | `ze_l2tp_sessions_active` increments | `test/l2tp/metrics-sessions-active.ci` |
| 100s bucket lands with loss=5 | → | `ze_l2tp_lcp_echo_loss_ratio` reflects 0.05 for that login | `test/l2tp/metrics-cqm-loss.ci` |
| Prometheus scrape request | → | All `ze_l2tp_*`, `ze_pppoe_*`, `ze_radius_*` families exposed with expected label set | `test/l2tp/metrics-scrape-shape.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Subsystem Start | All `ze_l2tp_*` metric families registered |
| AC-2 | RADIUS plugin Start | All `ze_radius_*` metric families registered; up/down gauge set per server |
| AC-3 | Session established | `ze_l2tp_sessions_active` increments; per-session counter set with full label set |
| AC-4 | Session closes | `ze_l2tp_sessions_active` decrements; per-session series deleted (no stale cardinality) |
| AC-5 | 100s bucket finalises | `ze_l2tp_lcp_echo_rtt_seconds` histogram observes bucket's min/avg/max values; `ze_l2tp_bucket_state` reflects state enum |
| AC-6 | Prometheus scrape | All metric names follow `ze_<component>_*` convention; no `nas_*` names present; no `target` label present |
| AC-7 | Label cardinality | Per-session labels match the existing `nas_session_*` set (minus `target` and `service`): `sid`, `ifname`, `username`, `ip`, `caller_id` |
| AC-8 | `ze_radius_up{server_id,server_addr}` | 1 when server responsive, 0 when unresponsive; transitions exercised |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestBindMetrics_Registration | `internal/component/l2tp/metrics_test.go` | All `ze_l2tp_*` metrics registered with correct names and label sets | |
| TestBindMetrics_SessionLifecycle | `internal/component/l2tp/metrics_test.go` | sessions_active increments on SessionUp, decrements + series delete on SessionDown | |
| TestBindMetrics_SessionState | `internal/component/l2tp/metrics_test.go` | session_state gauge reflects numeric FSM state values (0-4) | |
| TestBindMetrics_CQMObserve | `internal/component/l2tp/metrics_test.go` | lcp_echo_rtt_seconds histogram observes from CQM bucket; loss_ratio and bucket_state set | |
| TestStatsPoller_CounterDeltas | `internal/component/l2tp/metrics_test.go` | statsPoller reads iface.GetStats, computes deltas, updates rx/tx bytes/packets counters | |
| TestStatsPoller_SessionRemoved | `internal/component/l2tp/metrics_test.go` | Poller skips removed sessions, does not leak stale counter state | |
| TestStatsPoller_UptimeSeconds | `internal/component/l2tp/metrics_test.go` | session_uptime_seconds reflects time since session creation | |
| TestRADIUSMetrics_Registration | `internal/plugins/l2tpauthradius/metrics_test.go` | All `ze_radius_*` metrics registered with correct names and label sets | |
| TestRADIUSMetrics_AuthSent | `internal/plugins/l2tpauthradius/metrics_test.go` | auth_sent_total increments on RADIUS request; labeled by server | |
| TestRADIUSMetrics_AcctSent | `internal/plugins/l2tpauthradius/metrics_test.go` | acct_sent_total and interim_sent_total increment on accounting events | |
| TestRADIUSMetrics_UpDown | `internal/plugins/l2tpauthradius/metrics_test.go` | ze_radius_up gauge transitions between 0 and 1 | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Label cardinality per metric | Implicit | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| metrics-scrape-shape | `test/l2tp/metrics-scrape-shape.ci` | Scrape returns all expected `ze_l2tp_*` metric names; no `nas_*` names; correct label sets | |
| metrics-sessions-active | `test/l2tp/metrics-sessions-active.ci` | Session up increments gauge, session down decrements; per-session series cleaned up | |
| metrics-cqm-rtt | `test/l2tp/metrics-cqm-rtt.ci` | LCP echo RTT observations appear in histogram; bucket state reflects session lifecycle | |

## Files to Modify

- `internal/component/l2tp/subsystem.go` -- call `bindL2TPMetrics()` in Start, start/stop statsPoller
- `internal/component/l2tp/observer.go` -- add metric series Delete calls in SessionDown handler (`wireObserverSubscriptions`)
- `internal/plugins/l2tpauthradius/register.go` -- add `ConfigureMetrics` callback to registration
- `internal/plugins/l2tpauthradius/handler.go` -- increment `ze_radius_auth_sent_total` on RADIUS request, observe accept/reject
- `internal/plugins/l2tpauthradius/acct.go` -- increment `ze_radius_acct_sent_total`, `ze_radius_interim_sent_total`

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A (no new config for metrics) |
| Env vars | [x] | `ze.l2tp.metrics.poll-interval` -- stats poller interval (default 30s) |
| Functional tests | [x] | `test/l2tp/metrics-scrape-shape.ci`, `metrics-sessions-active.ci`, `metrics-cqm-rtt.ci` |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- L2TP Prometheus metrics |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [x] | Metrics reference: `ze_l2tp_*` / `ze_radius_*` name mapping table |
| 5 | Plugin added/changed? | [x] | RADIUS plugin docs: `ze_radius_*` metric names |
| 6 | Has a user guide page? | [x] | `docs/guide/observability.md` -- `nas_*` to `ze_*` migration note |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [x] | `docs/architecture/l2tp.md` -- metrics + statsPoller section |

## Files to Create

- `internal/component/l2tp/metrics.go` -- `l2tpMetrics` struct, `bindL2TPMetrics(reg)`, `statsPoller`, RTT bucket constant, session state enum mapping
- `internal/component/l2tp/metrics_test.go` -- unit tests for registration, lifecycle, CQM observe, poller deltas
- `internal/plugins/l2tpauthradius/metrics.go` -- `radiusMetrics` struct, `bindRADIUSMetrics(reg)`, atomic pointer
- `internal/plugins/l2tpauthradius/metrics_test.go` -- unit tests for RADIUS metric registration and increments
- `test/l2tp/metrics-scrape-shape.ci` -- functional: scrape returns correct metric names and labels
- `test/l2tp/metrics-sessions-active.ci` -- functional: session lifecycle reflected in gauges
- `test/l2tp/metrics-cqm-rtt.ci` -- functional: CQM RTT histogram populated

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This spec + observer + BFD metrics precedent |
| 2. Audit | Files to Modify/Create lists |
| 3. Implement (TDD) | Phases 1-5 below |
| 4. Full verification | `make ze-verify` |
| 5-12 | Per standard /implement flow |

### Implementation Phases

1. **L2TP aggregate + session-keyed metrics** -- `metrics.go`: `l2tpMetrics` struct with all `ze_l2tp_*` families; `bindL2TPMetrics(reg)` using atomic pointer; wire in `Subsystem.Start()` via `registry.GetMetricsRegistry()`. Aggregate gauges: `sessions_active`, `sessions_starting`, `sessions_finishing`. Per-session gauges/counters: `session_state`, `session_uptime_seconds`. Register `ze.l2tp.metrics.poll-interval` env var.

2. **Stats poller for per-session counters** -- `metrics.go`: `statsPoller` struct; periodic loop calls `iface.GetStats(pppN)` for each active session from reactor snapshots; delta computation for `session_rx_bytes_total`, `session_tx_bytes_total`, `session_rx_packets_total`, `session_tx_packets_total`. Start in `Subsystem.Start()`, stop in `Stop()`.

3. **CQM-derived metrics** -- Wire into observer: on bucket finalize, observe `lcp_echo_rtt_seconds` histogram (min/avg/max from CQMBucket); set `lcp_echo_loss_ratio` gauge; set `bucket_state` gauge. Delete per-login series when login is evicted from LRU.

4. **Observer SessionDown cleanup** -- In `wireObserverSubscriptions` SessionDown handler, after `ReleaseSession`: call `Delete(labelValues...)` on all per-session metric vecs. Decrement `sessions_active`.

5. **RADIUS plugin metrics** -- `l2tpauthradius/metrics.go`: `radiusMetrics` struct; `bindRADIUSMetrics(reg)` with atomic pointer; `ConfigureMetrics` in registration. Instrument: `ze_radius_auth_sent_total` in `doRADIUS`, `ze_radius_acct_sent_total` in `sendAcct`, `ze_radius_interim_sent_total` in `sendAcctInterim`, `ze_radius_up` in client health check.

6. **Functional tests** -- `.ci` tests for scrape shape, session lifecycle, CQM RTT.

7. **Documentation** -- Metric reference table with rename mapping; operator migration note for `nas_*` to `ze_*`.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-1..AC-8 demonstrated in tests |
| Naming | All metrics follow `ze_<component>_<name>_<unit>` convention |
| Cardinality | Per-session series deleted on close; no stale time series |
| Data flow | statsPoller reads cached counters, not per-scrape kernel calls |
| Rule: derive-not-hardcode | State enum values derived from FSM constants, not hardcoded integers |
| Precedent | Patterns match BFD metrics (atomic pointer, bind function, nil guard) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| L2TP metrics registered at Start | `TestBindMetrics_Registration` + `metrics-scrape-shape.ci` |
| Per-session counters populated | `TestStatsPoller_CounterDeltas` |
| CQM metrics from observer | `TestBindMetrics_CQMObserve` + `metrics-cqm-rtt.ci` |
| Session close cleans up series | `TestBindMetrics_SessionLifecycle` + `metrics-sessions-active.ci` |
| RADIUS metrics registered | `TestRADIUSMetrics_Registration` |
| No `nas_*` metrics in scrape | `metrics-scrape-shape.ci` |
| Docs updated | Metric reference table in docs |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Label injection | All label values from trusted internal state (session FSM, reactor snapshot), not from wire input |
| Cardinality bomb | Max sessions limit bounds the number of per-session series; statsPoller iterates snapshot, not unbounded map |
| Resource leak | statsPoller stopped in Subsystem.Stop(); context cancellation propagated |
| Sensitive data | No secrets in label values; username is already exposed in existing `nas_*` exporter |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in introducing phase |
| Test fails wrong reason | Fix test setup |
| Metric cardinality unexpectedly high | Back to DESIGN, trim labels |
| Lint failure | Fix inline |
| 3 fix attempts fail | STOP. Ask user. |

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

- `iface.GetStats()` already exists -- the per-session counter feature that looked like "new infrastructure" is just a poller loop over an existing API. Key risk was building something that already existed.
- L2TP is a subsystem, not a plugin, so it lacks `ConfigureMetrics` in its registration. The BFD workaround (`registry.GetMetricsRegistry()` in OnStarted) transfers directly. This is a pattern worth documenting for future subsystem authors.
- Observer's SessionDown handler is the natural cleanup point because it already owns ring release. Adding metric deletion there avoids dual-subscription ordering issues where metrics might try to read a released ring.
- The 1-2-5 bucket sequence for RTT histograms gives consistent visual spacing in Grafana's heatmap panel. Sub-1ms bucket (0.001) captures the tail of fibre subscribers; 5s ceiling catches pathological cases without wasting bucket space on impossible values.

## RFC Documentation

N/A for this spec.

## Implementation Summary

### What Was Implemented
- `internal/component/l2tp/metrics.go`: L2TP metrics struct (13 metrics), `bindL2TPMetrics`, `l2tpStatsPoller` (30s kernel counter reads via `iface.GetStats`), CQM bucket observe with loss ratio, session series cleanup on disappearance
- `internal/component/l2tp/metrics_test.go`: 8 unit tests covering registration, CQM observe, poller deltas, stale cleanup, formatSID, poll interval
- `internal/plugins/l2tpauthradius/metrics.go`: RADIUS metrics struct (4 metrics), `bindRADIUSMetrics`, inc/set helpers with server labels
- `internal/plugins/l2tpauthradius/metrics_test.go`: 7 unit tests covering registration, increments, nil safety
- Observer wiring: `observeCQMBucket` on bucket finalize, `deleteLoginMetrics` on LRU eviction, `expectedEchoesPerBucket` for loss ratio
- Subsystem wiring: `registry.GetMetricsRegistry()` in Start, poller start/stop lifecycle
- RADIUS plugin: `ConfigureMetrics` callback, `serverAddr` threaded through auth/acct for real label values
- Snapshot: added `PppInterface` and `StateNum` fields to `SessionSnapshot`

### Bugs Found/Fixed
- Review found `lcp_echo_loss_ratio` was registered but never set -- added loss computation from expected echoes per bucket
- Review found RADIUS metrics called with empty server labels -- threaded `serverAddr` from config through all call sites

### Documentation Updates
- `docs/features.md`: added L2TP metrics and RADIUS metrics to L2TPv2 Tunnels feature description
- `docs/guide/observability.md` and `docs/architecture/l2tp.md` do not exist yet; deferred to broader L2TP documentation spec

### Deviations from Plan
- `caller_id` label always empty: session struct does not track Calling-Station-Id. Label is present for forward compatibility but unpopulated until L2TP wire parsing surfaces it
- Per-session cleanup happens at next poller tick (up to 30s delay), not synchronously from SessionDown: observer lacks the label set needed for immediate deletion, and the poller already handles stale cleanup

## Implementation Audit (filled during IMPLEMENT)

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

## Review Gate (filled during IMPLEMENT)

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

## Pre-Commit Verification (filled during IMPLEMENT)

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table filled with concrete test names
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] All existing `nas_*` metrics accounted for (renamed or explicitly dropped)
- [ ] Documentation updated with rename table

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Metric names match `ze_<component>_*` convention
- [ ] Per-session series deleted on session close (no stale cardinality)
- [ ] Label sets consistent with `rules/naming.md` and Prometheus conventions

### TDD
- [ ] Tests written first
- [ ] Tests FAIL initially
- [ ] Tests PASS after implementation
- [ ] Scrape-shape test covers label set

### Completion (BLOCKING before commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary at `plan/learned/NNN-l2tp-10-metrics.md`
- [ ] Summary in same commit as code
