# Spec: l2tp-10 -- Prometheus Metrics Exposure

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-l2tp-9-observer, spec-l2tp-7-subsystem, spec-l2tp-8-plugins |
| Phase | - |
| Updated | 2026-04-17 |

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

## Design Decisions (agreed with user, 2026-04-17)

| # | Decision |
|---|----------|
| D2 | Adopt ze convention: `ze_l2tp_*`, `ze_pppoe_*`, `ze_radius_*`. Drop redundant `target` label. Drop `nas_scrape_duration_seconds` (exporter self-timing; not a real signal). Lean on ze process metrics for build-info / up / uptime / memory / cpu. |
| D3 | Metric emitters live in `internal/component/l2tp/metrics.go` (L2TP + observer-fed metrics) and in the RADIUS plugin / future PPPoE component. Precedent: `internal/plugins/bfd/metrics.go`. |

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
| `ze_l2tp_lcp_echo_rtt_seconds` | HistogramVec | Labeled by `username` (login-keyed). Histogram of bucket RTTs |
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

### Architecture Docs (filled during DESIGN phase)
- [ ] `docs/architecture/core-design.md` -- metrics registry usage
- [ ] `plan/spec-l2tp-9-observer.md` -- state that feeds CQM-derived metrics
- [ ] `internal/plugins/bfd/metrics.go` -- precedent for subsystem-registered metric families

### RFC Summaries
- [ ] None directly; this spec is about exposure shape, not protocol behavior.

**Key insights:** (filled during DESIGN phase)

## Current Behavior (MANDATORY)

**Source files to read during DESIGN phase:**
- [ ] `internal/core/metrics/metrics.go`, `prometheus.go`, `server.go`
- [ ] `internal/plugins/bfd/metrics.go`
- [ ] `internal/component/vpp/telemetry.go`
- [ ] `internal/component/l2tp/kernel_linux.go`, `pppox_linux.go` -- kernel counters source
- [ ] Existing `nas_*` exporter code (external to ze; reference only)

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

### Architectural Verification (filled during DESIGN phase)
- [ ] Per-session series deleted on session close (no stale cardinality)
- [ ] No per-scrape kernel reads (counters cached on observer events)
- [ ] Each metric family registered by its owning component, not in a central place

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

### Unit Tests (filled during DESIGN phase)

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TBD | `internal/component/l2tp/metrics_test.go` | Registration at Start, correct label sets | |
| TBD | `internal/component/l2tp/metrics_test.go` | Delete on session close (no stale series) | |
| TBD | RADIUS plugin | RADIUS metric registration | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Label cardinality per metric | Implicit | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| TBD | `test/l2tp/metrics-scrape-shape.ci` | Scrape returns expected metric names and label sets | |
| TBD | `test/l2tp/metrics-sessions-active.ci` | Session lifecycle reflected in active gauge | |
| TBD | `test/l2tp/metrics-cqm-loss.ci` | Loss ratio from bucket reaches the histogram | |

## Files to Modify

- `internal/component/l2tp/subsystem.go` -- register metrics at Start
- RADIUS plugin entry point (per `spec-l2tp-8-plugins`) -- register RADIUS metrics

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A (no new config for metrics) |
| Env vars | [ ] | N/A |
| Functional tests | [ ] | `test/l2tp/metrics-*.ci` |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` (operator section) |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | Metrics reference in `docs/architecture/api/metrics.md` |
| 5 | Plugin added/changed? | [ ] | RADIUS plugin docs note new metric names |
| 6 | Has a user guide page? | [ ] | `docs/guide/observability.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/l2tp.md` (metrics section) |

## Files to Create

- `internal/component/l2tp/metrics.go` -- L2TP metric families and registration
- `internal/component/l2tp/metrics_test.go`
- `test/l2tp/metrics-*.ci` (multiple; filled during DESIGN phase)
- New files in RADIUS plugin per `spec-l2tp-8-plugins` scope

## Implementation Steps

### /implement Stage Mapping (filled during DESIGN phase)

### Implementation Phases (filled during DESIGN phase)

Outline (rough):
1. L2TP metric registration in subsystem Start, covering aggregate and session-keyed metrics
2. CQM-derived metrics fed from observer ring reads
3. RADIUS plugin metric registration
4. PPPoE metric names reserved (implementation lands with PPPoE work)
5. Functional tests
6. Docs: metric reference table, operator migration note

### Critical Review Checklist (filled during DESIGN phase)

### Deliverables Checklist (filled during DESIGN phase)

### Security Review Checklist (filled during DESIGN phase)

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

(LIVE during DESIGN and IMPLEMENT phases)

## RFC Documentation

N/A for this spec.

## Implementation Summary (filled during IMPLEMENT)

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
