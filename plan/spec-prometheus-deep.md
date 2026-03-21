# Spec: prometheus-deep

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/386-prometheus-metrics.md` - decisions from initial Prometheus work
4. `plan/learned/388-cli-metrics.md` - CLI metrics commands
5. `plan/learned/376-backpressure.md` - backpressure mechanism (has metrics gaps)
6. `internal/core/metrics/metrics.go` - metrics interfaces (Counter, Gauge, CounterVec, GaugeVec)
7. `internal/component/bgp/reactor/reactor_metrics.go` - current reactor metrics
8. `internal/component/bgp/reactor/peer_stats.go` - per-peer atomic counters

## Task

Expand Ze's Prometheus instrumentation from the current 11 metrics to comprehensive DevOps-grade observability. The current metrics cover basic peer state and RIB route counts. This spec adds session lifecycle, message type breakdown, forward pool backpressure, RIB churn, plugin health, config reload, wire layer, GR state, and Histogram support to the metrics interface.

### Motivation

rustbgpd (reference implementation at `~/Code/github.com/lance0/rustbgpd/`) exposes 13 metrics covering session lifecycle, message exchange, RIB state, policy enforcement, loop detection, GR, and RPKI. Ze currently exposes 11 metrics with no session flap counting, no backpressure visibility, no error code breakdown, no message type split, and no timing distributions. Operators running Ze in production need these to debug convergence issues, detect slow peers, and monitor RIB health.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - metrics interfaces and reactor design
  → Decision: metrics use setter injection (`SetMetricsRegistry`) pattern
  → Constraint: nil-guard on every metrics call (metrics may be disabled)
- [ ] `docs/architecture/api/architecture.md` - plugin RPC flow for plugin metrics
  → Constraint: plugins receive registry via `SetMetricsRegistry` callback, same pattern as reactor
- [ ] `docs/architecture/pool-architecture.md` - attribute dedup pool internals for pool metrics
  → Constraint: pool counters are `atomic.Int64`, already tracked but not exposed

### Learned Summaries
- [ ] `plan/learned/386-prometheus-metrics.md` - initial Prometheus work decisions
  → Decision: map-based idempotent registry, per-instance (not global), `Delete()` for stale labels
  → Constraint: scalar metrics (Counter/Gauge) zero-wrap; Vec types need thin wrappers
- [ ] `plan/learned/388-cli-metrics.md` - CLI metrics access
  → Constraint: `bgp metrics show`/`list` work via dispatch-command, reads from same registry
- [ ] `plan/learned/376-backpressure.md` - backpressure mechanism
  → Decision: pause gate on session read loop, worker pool high/low water thresholds
  → Constraint: `onCongested`/`onResumed` callbacks already exist in forward pool

**Key insights:**
- Registry is idempotent get-or-create, so any component can register metrics by name
- Histogram type is missing from `metrics.Registry` interface -- needs adding for timing distributions
- Per-peer atomic counters (`peer_stats.go`) already track 6 message types but Prometheus aggregates them into 2 counters
- Forward pool has `onCongested`/`onResumed` callbacks and overflow drop logging -- just needs counter wiring
- Attribute pools track `internTotal`/`internHits` atomically but don't expose them

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/metrics/metrics.go` - Counter, Gauge, CounterVec, GaugeVec interfaces. No Histogram.
- [ ] `internal/core/metrics/prometheus.go` - PrometheusRegistry with map-based get-or-create. Handler() for HTTP.
- [ ] `internal/core/metrics/nop.go` - NopRegistry for disabled/test. Must mirror interface changes.
- [ ] `internal/core/metrics/server.go` - HTTP server with configurable address/port/path
- [ ] `internal/component/bgp/reactor/reactor_metrics.go` - `reactorMetrics` struct, `initReactorMetrics()`, `metricsUpdateLoop()` (10s ticker)
- [ ] `internal/component/bgp/reactor/peer_stats.go` - 6 atomic counters per peer, `Incr*` methods with Prometheus nil-guard
- [ ] `internal/component/bgp/reactor/forward_pool.go` - `onCongested`/`onResumed` callbacks, overflow drop log, `TryDispatch`/`DispatchOverflow`
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `SetMetricsRegistry()` with 4 gauges (routes in/out, global + per-peer)

**Current Prometheus metrics (11 total):**

| # | Metric | Type | Labels | Location |
|---|--------|------|--------|----------|
| 1 | `ze_info` | GaugeVec | version, router_id, local_as | reactor_metrics.go |
| 2 | `ze_peers_configured` | Gauge | - | reactor_metrics.go |
| 3 | `ze_uptime_seconds` | Gauge | - | reactor_metrics.go |
| 4 | `ze_cache_entries` | Gauge | - | reactor_metrics.go |
| 5 | `ze_forward_workers_active` | Gauge | - | reactor_metrics.go |
| 6 | `ze_peer_state` | GaugeVec | peer | reactor_metrics.go |
| 7 | `ze_peer_messages_received_total` | CounterVec | peer | reactor_metrics.go |
| 8 | `ze_peer_messages_sent_total` | CounterVec | peer | reactor_metrics.go |
| 9 | `ze_rib_routes_in_total` | Gauge | - | rib.go |
| 10 | `ze_rib_routes_out_total` | Gauge | - | rib.go |
| 11a | `ze_rib_routes_in` | GaugeVec | peer | rib.go |
| 11b | `ze_rib_routes_out` | GaugeVec | peer | rib.go |

**Behavior to preserve:**
- All 11 existing metrics must continue to work unchanged
- Nil-guard pattern on every metric increment
- `metricsUpdateLoop()` 10s ticker for snapshot gauges
- `Delete()` on Vec types for stale peer label cleanup
- `NopRegistry` for tests/disabled state
- `bgp metrics show`/`list` CLI commands

**Behavior to change:**
- Add `Histogram`/`HistogramVec` to metrics interfaces and all backends
- Split `ze_peer_messages_*_total` by message type (currently aggregates all)
- Add new metrics across all phases below

## rustbgpd Comparison (Reference)

rustbgpd exposes 13 metrics. Mapping to Ze equivalents:

| rustbgpd Metric | Type | Ze Current | Ze Planned |
|-----------------|------|------------|------------|
| `bgp_session_state_transitions_total` | CounterVec(peer,from,to) | `ze_peer_state` (gauge, no transitions) | `ze_peer_state_transitions_total` |
| `bgp_session_flaps_total` | CounterVec(peer) | None | `ze_peer_session_flaps_total` |
| `bgp_session_established_total` | CounterVec(peer) | None | `ze_peer_sessions_established_total` |
| `bgp_messages_sent_total` | CounterVec(peer,type) | `ze_peer_messages_sent_total` (no type) | `ze_peer_messages_sent_total` + type label |
| `bgp_messages_received_total` | CounterVec(peer,type) | `ze_peer_messages_received_total` (no type) | `ze_peer_messages_received_total` + type label |
| `bgp_notifications_sent_total` | CounterVec(peer,code,subcode) | None | `ze_peer_notifications_sent_total` |
| `bgp_notifications_received_total` | CounterVec(peer,code,subcode) | None | `ze_peer_notifications_received_total` |
| `bgp_rib_prefixes` | IntGaugeVec(peer,afi_safi) | `ze_rib_routes_in` (peer only) | `ze_rib_routes_in` + family label |
| `bgp_rib_loc_prefixes` | IntGaugeVec(afi_safi) | `ze_rib_routes_in_total` (no family) | `ze_rib_routes_in_total` + family label |
| `bgp_max_prefix_exceeded_total` | CounterVec(peer) | None | Deferred (no max-prefix feature yet) |
| `bgp_outbound_route_drops_total` | CounterVec(peer) | None | `ze_forward_overflow_drops_total` |
| `bgp_as_path_loop_detected_total` | CounterVec(peer) | None | Deferred (loop detection in RS plugin) |
| `bgp_rr_loop_detected_total` | CounterVec(peer) | None | Deferred (loop detection in RS plugin) |
| `bgp_gr_active_peers` | GaugeVec(peer) | None | `ze_gr_active_peers` |
| `bgp_gr_stale_routes` | GaugeVec(peer) | None | `ze_gr_stale_routes` |
| `bgp_gr_timer_expired_total` | CounterVec(peer) | None | `ze_gr_timer_expired_total` |
| `bgp_rpki_vrp_count` | GaugeVec(af) | None | Deferred (RPKI decoration is new) |
| `bgp_aspa_records_total` | Gauge | None | Deferred (no ASPA feature) |

### Beyond rustbgpd (Ze-specific)

Ze has architectural features rustbgpd doesn't (plugin system, forward pool, attribute dedup pools, config reload, chaos testing) that deserve their own metrics:

| Category | Why rustbgpd doesn't have it | Ze-specific value |
|----------|------------------------------|-------------------|
| Forward pool backpressure | Different forwarding model | Per-peer congestion, overflow drops, batch sizes |
| Attribute pool dedup | Different RIB structure | Cache hit ratio, pool utilization, compaction events |
| Plugin event delivery | No plugin architecture | Event queue depth, delivery errors, plugin restarts |
| Config reload | Static config | Reload count, parse errors, peer reconciliation |
| Histogram timing | No histograms at all | UPDATE parse time, forward batch time, RPC latency |

## Data Flow (MANDATORY)

### Entry Point
- Metrics are registered by each component during startup via `SetMetricsRegistry(reg)`
- Metrics are incremented in hot paths (atomic counters, nil-guarded)
- Metrics are read by HTTP scrape or `bgp metrics show` CLI

### Transformation Path
1. `config/loader.go` creates `PrometheusRegistry` from telemetry config
2. Registry injected into reactor (`SetMetricsRegistry`) and plugins
3. Components call `reg.Counter()`/`reg.Gauge()`/`reg.Histogram()` to register
4. Hot paths call `.Inc()`/`.Add()`/`.Set()`/`.Observe()` with nil guards
5. `metricsUpdateLoop()` refreshes snapshot gauges every 10s
6. HTTP handler or CLI scrapes Prometheus text format

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → Reactor | `SetMetricsRegistry(reg)` setter injection | [ ] |
| Config → RIB Plugin | `SetMetricsRegistry(reg)` via registry callback | [ ] |
| Config → GR Plugin | `SetMetricsRegistry(reg)` (new) | [ ] |
| Reactor → Forward Pool | Pool callbacks wired to reactor metric counters | [ ] |
| HTTP → Registry | `promhttp.HandlerFor(registry)` | [ ] |

### Integration Points
- `reactor_metrics.go` `initReactorMetrics()` - add new fields
- `peer_stats.go` `Incr*` methods - add type label to existing Prometheus calls
- `forward_pool.go` `onCongested`/`DispatchOverflow` - wire to counters
- `rib.go` `SetMetricsRegistry` - add churn counters
- `gr.go` - new `SetMetricsRegistry` entry point for GR plugin

### Architectural Verification
- [ ] No bypassed layers (metrics flow through existing injection pattern)
- [ ] No unintended coupling (each component registers its own metrics independently)
- [ ] No duplicated functionality (extends existing metrics infrastructure)
- [ ] Zero-copy preserved (metrics are counters/gauges, no data copying)

## Proposed Metrics (Full Inventory)

### Phase 1: Histogram Interface + Session Lifecycle

**New interface types:**

| Interface | Methods |
|-----------|---------|
| `Histogram` | `Observe(float64)` |
| `HistogramVec` | `With(labelValues ...string) Histogram`, `Delete(labelValues ...string) bool` |

**Registry additions:** `Histogram(name, help string, buckets []float64) Histogram`, `HistogramVec(name, help string, buckets []float64, labelNames []string) HistogramVec`

**New session metrics (reactor_metrics.go):**

| Metric | Type | Labels | Help |
|--------|------|--------|------|
| `ze_peer_sessions_established_total` | CounterVec | peer | Times session reached Established |
| `ze_peer_session_flaps_total` | CounterVec | peer | Sessions dropped from Established |
| `ze_peer_state_transitions_total` | CounterVec | peer, from, to | FSM state transitions |
| `ze_peer_notifications_sent_total` | CounterVec | peer, code, subcode | NOTIFICATION messages sent |
| `ze_peer_notifications_received_total` | CounterVec | peer, code, subcode | NOTIFICATION messages received |
| `ze_peer_session_duration_seconds` | GaugeVec | peer | Seconds since session established |

**Message type split (modify existing):**

| Metric | Change |
|--------|--------|
| `ze_peer_messages_received_total` | Add `type` label: update, keepalive, eor, open, notification, route_refresh |
| `ze_peer_messages_sent_total` | Add `type` label: same set |

### Phase 2: Forward Pool Backpressure

| Metric | Type | Labels | Help |
|--------|------|--------|------|
| `ze_forward_congestion_events_total` | CounterVec | peer | Channel full events (onset) |
| `ze_forward_congestion_resumed_total` | CounterVec | peer | Channel resumed from congestion |
| `ze_forward_overflow_drops_total` | CounterVec | peer | Items dropped from overflow buffer |
| `ze_forward_dispatch_total` | CounterVec | peer, method | Dispatch calls (dispatch, try_dispatch, overflow) |
| `ze_forward_batch_size` | HistogramVec | peer | Items per worker batch |
| `ze_forward_channel_utilization` | GaugeVec | peer | Channel occupancy ratio (len/cap) |

### Phase 3: RIB Operations + Attribute Pools

| Metric | Type | Labels | Help |
|--------|------|--------|------|
| `ze_rib_route_inserts_total` | CounterVec | peer, family | Routes inserted |
| `ze_rib_route_withdrawals_total` | CounterVec | peer, family | Routes withdrawn |
| `ze_rib_eor_received_total` | CounterVec | peer, family | End-of-RIB markers |
| `ze_attr_pool_intern_total` | CounterVec | pool | Total Intern() calls |
| `ze_attr_pool_dedup_hits_total` | CounterVec | pool | Intern() dedup hits |
| `ze_attr_pool_slots_used` | GaugeVec | pool | Active slots per pool |

### Phase 4: Config Reload + Plugin Health

| Metric | Type | Labels | Help |
|--------|------|--------|------|
| `ze_config_reloads_total` | Counter | - | Successful config reloads |
| `ze_config_reload_errors_total` | CounterVec | error_type | Failed config reloads |
| `ze_peers_added_total` | Counter | - | Peers added via config |
| `ze_peers_removed_total` | Counter | - | Peers removed via config |
| `ze_plugin_status` | GaugeVec | plugin | Plugin state (0=stopped, 1=running, 2=error) |
| `ze_plugin_restarts_total` | CounterVec | plugin | Plugin crash restarts |
| `ze_plugin_events_delivered_total` | CounterVec | plugin | Events dispatched to plugin |

### Phase 5: Wire Layer + GR State

| Metric | Type | Labels | Help |
|--------|------|--------|------|
| `ze_wire_bytes_received_total` | CounterVec | peer | Bytes read from TCP |
| `ze_wire_bytes_sent_total` | CounterVec | peer | Bytes written to TCP |
| `ze_wire_read_errors_total` | CounterVec | peer | Socket read failures |
| `ze_wire_write_errors_total` | CounterVec | peer | Socket write failures |
| `ze_gr_active_peers` | Gauge | - | Peers currently in GR |
| `ze_gr_stale_routes` | GaugeVec | peer | Stale routes pending refresh |
| `ze_gr_timer_expired_total` | CounterVec | peer | GR restart timer expirations |

### Phase 6 (Future): Process + Runtime

| Metric | Type | Labels | Help |
|--------|------|--------|------|
| `ze_go_goroutines` | Gauge | - | Current goroutine count |
| `ze_go_gc_pause_seconds` | Histogram | - | GC pause duration |
| `ze_update_parse_seconds` | HistogramVec | peer | UPDATE parse time |
| `ze_forward_batch_write_seconds` | HistogramVec | peer | Batch TCP write time |

**Note:** Phase 6 is speculative. Go runtime metrics may be better served by the `prometheus/client_golang` default process collector. Timing histograms add overhead to hot paths and need benchmarking first.

### Metric Count Summary

| Phase | New Metrics | Running Total |
|-------|-------------|---------------|
| Current | 11 | 11 |
| Phase 1: Histogram + Session | 8 (+ 2 modified) | 19 |
| Phase 2: Forward Pool | 6 | 25 |
| Phase 3: RIB + Pools | 6 | 31 |
| Phase 4: Config + Plugin | 7 | 38 |
| Phase 5: Wire + GR | 7 | 45 |
| Phase 6: Runtime (future) | 4 | 49 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| telemetry config | -> | Prometheus HTTP /metrics endpoint shows new metrics | `test/plugin/cli-metrics-show-deep.ci` |
| `bgp metrics list` CLI | -> | New metric names appear in list output | `test/plugin/cli-metrics-list-deep.ci` |
| peer session established | -> | `ze_peer_sessions_established_total` increments | `test/plugin/metrics-session-lifecycle.ci` |
| forward pool congestion | -> | `ze_forward_congestion_events_total` increments | `test/plugin/forward-backpressure.ci` (existing, extend) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Histogram interface added to `metrics.Registry` | `Histogram()` and `HistogramVec()` methods exist on Registry, PrometheusRegistry, NopRegistry |
| AC-2 | Peer reaches Established, then drops | `ze_peer_sessions_established_total{peer=X}` increments on up, `ze_peer_session_flaps_total{peer=X}` increments on drop |
| AC-3 | Peer sends UPDATE, KEEPALIVE, NOTIFICATION | `ze_peer_messages_received_total{peer=X,type=Y}` increments with correct type label |
| AC-4 | NOTIFICATION sent to peer | `ze_peer_notifications_sent_total{peer=X,code=Y,subcode=Z}` increments |
| AC-5 | Forward pool channel fills | `ze_forward_congestion_events_total{peer=X}` increments |
| AC-6 | Forward pool overflow drops item | `ze_forward_overflow_drops_total{peer=X}` increments |
| AC-7 | Route inserted into RIB | `ze_rib_route_inserts_total{peer=X,family=Y}` increments |
| AC-8 | Config reload succeeds | `ze_config_reloads_total` increments |
| AC-9 | `bgp metrics list` shows new metrics | Output includes `ze_peer_session_flaps_total`, `ze_forward_congestion_events_total`, etc. |
| AC-10 | GR peer timer expires | `ze_gr_timer_expired_total{peer=X}` increments |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHistogramInterface` | `internal/core/metrics/prometheus_test.go` | Histogram and HistogramVec creation, Observe(), buckets | |
| `TestNopHistogram` | `internal/core/metrics/nop_test.go` | NopRegistry Histogram/HistogramVec are no-ops | |
| `TestHistogramIdempotent` | `internal/core/metrics/prometheus_test.go` | Same-name Histogram returns existing | |
| `TestSessionLifecycleMetrics` | `internal/component/bgp/reactor/reactor_metrics_test.go` | Established/flap counters increment on FSM transitions | |
| `TestMessageTypeLabels` | `internal/component/bgp/reactor/peer_stats_test.go` | Each Incr* sets correct type label | |
| `TestForwardPoolCongestionMetrics` | `internal/component/bgp/reactor/forward_pool_test.go` | Congestion/overflow counters fire on full channel | |
| `TestRIBChurnMetrics` | `internal/component/bgp/plugins/rib/rib_metrics_test.go` | Insert/withdrawal counters increment | |
| `TestConfigReloadMetrics` | `internal/component/bgp/reactor/reactor_api_test.go` | Reload counter increments | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Histogram bucket | 0.0+ | 0.001 (1ms) | N/A (any positive) | N/A |
| Notification code | 1-6 | 6 (Cease) | 0 | 255 (string label, no validation) |
| Notification subcode | 0-255 | 255 | N/A | N/A (string label) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-metrics-show-deep` | `test/plugin/cli-metrics-show-deep.ci` | `bgp metrics show` includes new metric names | |
| `cli-metrics-list-deep` | `test/plugin/cli-metrics-list-deep.ci` | `bgp metrics list` includes new metric names | |
| `metrics-session-lifecycle` | `test/plugin/metrics-session-lifecycle.ci` | Session up/down cycle produces correct counters | |

### Future (if deferring any tests)
- Phase 6 (runtime/timing) histograms need benchmarking before committing to hot path instrumentation
- Loop detection metrics deferred until RS plugin implements loop detection
- Max-prefix metrics deferred until max-prefix feature exists
- RPKI/ASPA metrics deferred until RPKI validation is production-ready

## Files to Modify

- `internal/core/metrics/metrics.go` - Add Histogram, HistogramVec interfaces to Registry
- `internal/core/metrics/prometheus.go` - Implement Histogram/HistogramVec with prometheus.Histogram
- `internal/core/metrics/nop.go` - Add nopHistogram, nopHistogramVec
- `internal/core/metrics/server.go` - No change (HTTP handler serves all registered metrics)
- `internal/component/bgp/reactor/reactor_metrics.go` - Add session lifecycle, forward pool, wire layer metrics
- `internal/component/bgp/reactor/peer_stats.go` - Add type label to message counters, add notification counters
- `internal/component/bgp/reactor/forward_pool.go` - Wire `onCongested`/overflow to metrics counters
- `internal/component/bgp/plugins/rib/rib.go` - Add churn counters (insert/withdrawal/EOR)
- `internal/component/bgp/plugins/gr/gr.go` - Add GR state metrics
- `internal/component/bgp/reactor/reactor_api.go` - Add config reload counter
- `internal/component/bgp/reactor/reactor_peers.go` - Add peers added/removed counters

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (metrics auto-appear via existing registry) |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | Existing `bgp metrics show/list` automatically includes new metrics |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/plugin/cli-metrics-show-deep.ci` |

## Files to Create

- `internal/core/metrics/histogram.go` - Histogram wrapper types (promHistogramVec, etc.)
- `test/plugin/cli-metrics-show-deep.ci` - Functional test for new metrics in show output
- `test/plugin/cli-metrics-list-deep.ci` - Functional test for new metrics in list output
- `test/plugin/metrics-session-lifecycle.ci` - Session up/down cycle metric verification

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases 1-5 below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Histogram Interface** -- Add Histogram/HistogramVec to metrics package
   - Tests: `TestHistogramInterface`, `TestNopHistogram`, `TestHistogramIdempotent`
   - Files: `metrics.go`, `prometheus.go`, `nop.go`, `histogram.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Session Lifecycle** -- Established/flap/transition counters, message type split, notifications
   - Tests: `TestSessionLifecycleMetrics`, `TestMessageTypeLabels`
   - Files: `reactor_metrics.go`, `peer_stats.go`
   - Verify: tests fail -> implement -> tests pass
   - Note: Adding `type` label to existing `ze_peer_messages_*_total` is a **breaking change** for existing dashboards. The old metric name stays but gains a label dimension.

3. **Phase: Forward Pool Backpressure** -- Congestion/overflow/dispatch counters
   - Tests: `TestForwardPoolCongestionMetrics`
   - Files: `forward_pool.go`, `reactor_metrics.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: RIB + Attribute Pools** -- Churn counters, pool stats
   - Tests: `TestRIBChurnMetrics`
   - Files: `rib.go`, `rib_metrics_test.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Config + Plugin + Wire + GR** -- Remaining operational metrics
   - Tests: `TestConfigReloadMetrics`
   - Files: `reactor_api.go`, `reactor_peers.go`, `gr.go`, session files
   - Verify: tests fail -> implement -> tests pass

6. **Functional tests** -- Create after feature works. Cover user-visible behavior.
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every metric in Proposed Metrics table has implementation with file:line |
| Correctness | Metric names follow `ze_` prefix convention, labels match Ze naming (kebab-case JSON, but snake_case Prometheus) |
| Naming | All metric names use snake_case with `ze_` prefix per Prometheus conventions |
| Data flow | Metrics registered via same `SetMetricsRegistry` injection pattern, not imported directly |
| Nil guards | Every `.Inc()`/`.Observe()` call has nil check on metrics struct pointer |
| Label cardinality | No unbounded label values (peer address is bounded by config; error codes are bounded by RFC) |
| Hot path overhead | Counter/Gauge increments are O(1). Histograms only where benchmarked acceptable |
| Breaking changes | Adding `type` label to existing message counters documented and handled |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Histogram interface in metrics.go | `grep 'Histogram(' internal/core/metrics/metrics.go` |
| NopHistogram in nop.go | `grep 'nopHistogram' internal/core/metrics/nop.go` |
| PrometheusHistogram in prometheus.go or histogram.go | `grep 'prometheus.NewHistogram' internal/core/metrics/` |
| Session flap counter | `grep 'ze_peer_session_flaps_total' internal/component/bgp/reactor/` |
| Forward congestion counter | `grep 'ze_forward_congestion_events_total' internal/component/bgp/reactor/` |
| RIB insert counter | `grep 'ze_rib_route_inserts_total' internal/component/bgp/plugins/rib/` |
| GR timer counter | `grep 'ze_gr_timer_expired_total' internal/component/bgp/plugins/gr/` |
| Config reload counter | `grep 'ze_config_reloads_total' internal/component/bgp/reactor/` |
| Functional test exists | `ls test/plugin/cli-metrics-show-deep.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Label cardinality | Peer addresses come from config (bounded). Error codes from RFC (bounded). No user-supplied free-text labels. |
| DoS via /metrics | Scrape endpoint returns only registered metrics. No query parameters that expand output. Same risk as existing Prometheus endpoint. |
| Information leakage | Peer addresses are already visible in `ze_peer_state`. No new sensitive data exposed. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

## RFC Documentation

Not applicable -- metrics are operational, not protocol-level.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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
- [ ] Write learned summary to `plan/learned/NNN-prometheus-deep.md`
- [ ] Summary included in commit
