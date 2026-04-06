# Spec: Plugin-Owned Prometheus Metrics

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/core/metrics/metrics.go` - Registry interface
4. `internal/component/plugin/registry/registry.go` - Registration struct, ConfigureMetrics
5. `internal/component/plugin/inprocess.go` - GetInternalPluginRunner
6. `internal/component/bgp/reactor/reactor_metrics.go` - current centralized metrics

## Task

Enable every plugin to declare and own its own Prometheus metrics instead of centralizing
all metrics in the reactor. Inspired by osvbng's plugin-based collectors
(see `docs/research/comparison/osvbng.md`).

Today the infrastructure is 90% there: `ConfigureMetrics` callback exists in Registration,
`GetInternalPluginRunner` calls it, and 2 of 34 plugins use it (bgp-rib, gr). But 32
plugins have no metrics, the reactor centralizes metrics that belong to plugins, and there
is no naming convention or documentation.

The goal is: every plugin that has observable state registers its own metrics via the
existing `ConfigureMetrics` callback, following a documented naming convention. Metrics
that currently live in `reactor_metrics.go` but belong to a specific plugin move to that
plugin. The reactor keeps only reactor-level metrics.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - config design
  -> Constraint: telemetry config is YANG-modeled (`ze-telemetry-conf.yang`)
- [ ] `docs/research/comparison/osvbng.md` - osvbng plugin-owned metrics pattern

**Key insights:**
- `metrics.Registry` interface has 6 creation methods: Counter, Gauge, CounterVec, GaugeVec, Histogram, HistogramVec
- `NopRegistry` exists for when metrics are disabled (tests, config `enabled: false`)
- `PrometheusRegistry` is idempotent: calling Counter("x", "help") twice returns same counter
- `ConfigureMetrics func(reg any)` already in Registration, already called by GetInternalPluginRunner
- bgp-rib and gr already use ConfigureMetrics pattern: type-assert to metrics.Registry, store in atomic pointer
- Reactor owns ~40 metrics in reactor_metrics.go, some belong to plugins (prefix limits -> rib, GR timers -> gr)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/metrics/metrics.go` - Registry interface (6 methods)
  -> Constraint: Counter, Gauge, CounterVec, GaugeVec, Histogram, HistogramVec
- [ ] `internal/core/metrics/prometheus.go` - PrometheusRegistry, idempotent registration
  -> Constraint: same name returns same metric (thread-safe via sync.RWMutex)
- [ ] `internal/core/metrics/nop.go` - NopRegistry for disabled state
  -> Constraint: all methods return no-op implementations
- [ ] `internal/component/plugin/registry/registry.go` - Registration struct (lines 216-218)
  -> Constraint: ConfigureMetrics is `func(reg any)` -- uses `any` to avoid import cycle
- [ ] `internal/component/plugin/inprocess.go` - GetInternalPluginRunner (lines 80-82)
  -> Constraint: calls ConfigureMetrics if non-nil and registry available
- [ ] `internal/component/bgp/plugins/rib/register.go` - example ConfigureMetrics usage
  -> Constraint: type-asserts `any` to `metrics.Registry`, calls SetMetricsRegistry
- [ ] `internal/component/bgp/plugins/rib/rib.go` - SetMetricsRegistry (line 111)
  -> Constraint: stores in atomic pointer, creates metrics from registry
- [ ] `internal/component/bgp/plugins/gr/register.go` - gr ConfigureMetrics usage
  -> Constraint: same pattern as rib
- [ ] `internal/component/bgp/reactor/reactor_metrics.go` - centralized metrics
  -> Constraint: ~40 metrics all prefixed `ze_` created in initReactorMetrics

**Behavior to preserve:**
- Registry interface unchanged (6 methods)
- PrometheusRegistry idempotent behavior
- NopRegistry for disabled state
- ConfigureMetrics callback signature `func(reg any)`
- GetInternalPluginRunner calling sequence (logger, metrics, bus, server)
- Existing bgp-rib and gr metrics continue to work
- Reactor-level metrics (ze_peers_configured, ze_uptime_seconds, etc.) stay in reactor

**Behavior to change:**
- Plugins that have observable state gain ConfigureMetrics callbacks
- Plugin metrics follow naming convention `ze_plugin_{name}_*`
- Prefix limit metrics move from reactor to bgp-rib (they are RIB state)
- Document the naming convention and pattern

## Data Flow (MANDATORY)

### Entry Point
- Metrics registry created during config loading when telemetry is enabled
- Stored in registry global via `registry.SetMetricsRegistry(reg)`

### Transformation Path
1. Config loader creates PrometheusRegistry (or NopRegistry if disabled)
2. Registry stored globally via `registry.SetMetricsRegistry(reg)`
3. `GetInternalPluginRunner` retrieves registry, calls `ConfigureMetrics(reg)` per plugin
4. Plugin type-asserts to `metrics.Registry`, stores in package-level atomic pointer
5. Plugin creates metrics from registry during init or OnStarted
6. Plugin updates metrics during normal operation
7. Prometheus scrapes `/metrics` endpoint, all metrics appear under one registry

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config loader -> Plugin registry | `registry.SetMetricsRegistry(reg)` | [ ] |
| Plugin registry -> Plugin | `ConfigureMetrics(reg any)` callback | [ ] |
| Plugin -> Prometheus | Metrics registered in shared PrometheusRegistry | [ ] |

### Integration Points
- `GetInternalPluginRunner` - existing, already calls ConfigureMetrics
- `registry.SetMetricsRegistry` - existing, stores registry globally
- `metrics.Registry` interface - existing, used to create metrics
- `Registration.ConfigureMetrics` - existing callback field

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (N/A for metrics)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin startup with metrics enabled | -> | Plugin's ConfigureMetrics called, metrics registered | `TestPluginMetricsRegistered` |
| Prometheus scrape after plugin startup | -> | Plugin-owned metrics appear in output | `TestPluginMetricsScrapable` |
| Plugin startup with metrics disabled | -> | NopRegistry used, no panic | `TestPluginMetricsDisabled` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin registers ConfigureMetrics callback | Callback called during GetInternalPluginRunner with metrics.Registry |
| AC-2 | Plugin creates counter via registry | Counter appears in Prometheus scrape output |
| AC-3 | Metrics disabled in config | Plugin receives NopRegistry, no panic, no metrics exposed |
| AC-4 | Two plugins register same metric name | PrometheusRegistry returns same metric (idempotent, no error) |
| AC-5 | fibkernel plugin has ConfigureMetrics | Route install/delete counters exposed |
| AC-6 | sysrib plugin has ConfigureMetrics | Best-route selection counters exposed |
| AC-7 | Prefix limit metrics | Move from reactor_metrics.go to bgp-rib plugin |
| AC-8 | Plugin naming convention | All new plugin metrics use `ze_{pluginname}_` prefix |
| AC-9 | Documentation | Pattern documented in plugin development guide |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginMetricsRegistered` | `plugin/inprocess_test.go` | ConfigureMetrics called with registry | |
| `TestPluginMetricsScrapable` | `plugin/inprocess_test.go` | Metrics appear in Prometheus text output | |
| `TestPluginMetricsDisabled` | `plugin/inprocess_test.go` | NopRegistry passed when metrics nil | |
| `TestFibkernelMetrics` | `plugins/fibkernel/fibkernel_test.go` | Route counters registered | |
| `TestSysribMetrics` | `plugins/sysrib/sysrib_test.go` | Best-route counters registered | |
| `TestRibPrefixMetricsMoved` | `plugins/rib/rib_metrics_test.go` | Prefix metrics owned by RIB | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-plugin-metrics` | `test/plugin/test-plugin-metrics.ci` | Start ze with telemetry, scrape /metrics, plugin metrics present | |

### Future (if deferring any tests)
- Per-plugin metric completeness audit (verify every plugin with state has metrics)

## Files to Modify

### Plugins gaining ConfigureMetrics
- `internal/plugins/fibkernel/register.go` - add ConfigureMetrics callback
- `internal/plugins/fibkernel/fibkernel.go` - add SetMetricsRegistry, route install/delete counters
- `internal/plugins/sysrib/register.go` - add ConfigureMetrics callback
- `internal/plugins/sysrib/sysrib.go` - add SetMetricsRegistry, best-route counters
- `internal/component/bgp/plugins/watchdog/register.go` - add ConfigureMetrics callback
- `internal/component/bgp/plugins/rpki/register.go` - add ConfigureMetrics callback
- `internal/component/bgp/plugins/persist/register.go` - add ConfigureMetrics callback

### Metrics migration (reactor -> plugin)
- `internal/component/bgp/reactor/reactor_metrics.go` - remove prefix limit metrics
- `internal/component/bgp/plugins/rib/rib.go` - absorb prefix limit metrics from reactor

### Documentation
- `docs/plugin-development/metrics.md` - new: plugin metrics pattern and naming convention

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| CLI commands/flags | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - mention metrics capability |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/metrics.md` - new page |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - plugin-owned metrics |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - metrics ownership |

## Files to Create
- `docs/plugin-development/metrics.md` - plugin metrics pattern documentation
- `test/plugin/test-plugin-metrics.ci` - functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Document naming convention** -- write `docs/plugin-development/metrics.md`
   - Convention: `ze_{pluginname}_{metric}_{unit}_{type}` (e.g., `ze_fibkernel_routes_installed_total`)
   - Pattern: atomic pointer + SetMetricsRegistry + create metrics from registry
   - NopRegistry fallback when metrics disabled
   - Tests: N/A (documentation only)
   - Files: `docs/plugin-development/metrics.md`

2. **Phase: Add ConfigureMetrics to key plugins** -- fibkernel, sysrib, watchdog
   - Tests: `TestFibkernelMetrics`, `TestSysribMetrics`
   - Files: fibkernel/register.go, fibkernel/fibkernel.go, sysrib/register.go, sysrib/sysrib.go, watchdog/register.go
   - Verify: tests fail -> implement -> tests pass
   - Pattern per plugin:
     - Add `ConfigureMetrics` to registration (type-assert to metrics.Registry)
     - Add `SetMetricsRegistry(reg metrics.Registry)` function
     - Add package-level `metricsPtr atomic.Pointer[pluginMetrics]` struct
     - Create metrics from registry in SetMetricsRegistry
     - Update metrics at relevant points in plugin logic

3. **Phase: Migrate prefix metrics from reactor to rib** -- move ownership
   - Tests: `TestRibPrefixMetricsMoved`
   - Files: reactor_metrics.go, rib/rib.go
   - Verify: prefix limit metrics still appear in scrape but are owned by rib
   - Key: reactor stops creating `ze_bgp_prefix_*` metrics, rib creates them instead
   - Reactor still calls rib to update prefix counts (via existing event flow)

4. **Phase: Functional test** -- scrape /metrics, verify plugin metrics present
   - Tests: `test-plugin-metrics.ci`
   - Files: `test/plugin/test-plugin-metrics.ci`

5. **Full verification** -> `make ze-verify`
6. **Complete spec** -> Fill audit tables, write learned summary

### Naming Convention

| Prefix | Owner | Examples |
|--------|-------|---------|
| `ze_` | Reactor (system-level) | `ze_peers_configured`, `ze_uptime_seconds` |
| `ze_peer_` | Reactor (per-peer) | `ze_peer_state`, `ze_peer_messages_received_total` |
| `ze_rib_` | bgp-rib plugin | `ze_rib_routes_total`, `ze_rib_prefix_count` |
| `ze_gr_` | gr plugin | `ze_gr_restart_time_seconds`, `ze_gr_stale_routes` |
| `ze_fibkernel_` | fibkernel plugin | `ze_fibkernel_routes_installed_total`, `ze_fibkernel_routes_deleted_total` |
| `ze_sysrib_` | sysrib plugin | `ze_sysrib_best_routes_total`, `ze_sysrib_events_processed_total` |
| `ze_watchdog_` | watchdog plugin | `ze_watchdog_checks_total`, `ze_watchdog_failures_total` |
| `ze_rpki_` | rpki plugin | `ze_rpki_vrps_total`, `ze_rpki_validation_total` |
| `ze_persist_` | persist plugin | `ze_persist_writes_total`, `ze_persist_reads_total` |

Labels follow reactor conventions: `{peer}`, `{family}`, `{type}`.

### Plugin Metrics Pattern (canonical)

Each plugin follows this pattern (from bgp-rib, the reference implementation):

Registration: add `ConfigureMetrics` callback that type-asserts `any` to `metrics.Registry`.

Storage: package-level `atomic.Pointer` holding a metrics struct.

Creation: `SetMetricsRegistry` creates all metrics from registry, stores in atomic pointer.

Fallback: if `ConfigureMetrics` is not called (metrics disabled), all metric operations
are no-ops because the pointer is nil and callers check before use.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Metrics names follow naming convention, no duplicates across plugins |
| Naming | All plugin metrics use `ze_{pluginname}_` prefix |
| Data flow | Metrics created during plugin startup, updated during operation |
| Rule: no-layering | Migrated prefix metrics fully removed from reactor_metrics.go |
| Rule: existing pattern | New plugins follow exact same pattern as bgp-rib/gr |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| fibkernel has ConfigureMetrics | `grep ConfigureMetrics internal/plugins/fibkernel/register.go` |
| sysrib has ConfigureMetrics | `grep ConfigureMetrics internal/plugins/sysrib/register.go` |
| Prefix metrics moved to rib | `grep -c ze_bgp_prefix internal/component/bgp/reactor/reactor_metrics.go` returns 0 |
| Naming convention doc exists | `ls docs/plugin-development/metrics.md` |
| Functional test exists | `ls test/plugin/test-plugin-metrics.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Metric names are compile-time constants, no user input |
| Resource exhaustion | Bounded number of metrics per plugin (no dynamic label explosion) |
| Information leakage | Metric names/labels do not expose sensitive config (passwords, keys) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

### Use existing ConfigureMetrics callback, not a new mechanism

The infrastructure is already there. No new callback type, no new registry method.
Just add `ConfigureMetrics` to more plugin registrations following the proven bgp-rib pattern.

### Prefix metrics move from reactor to rib

Prefix count, maximum, warning, ratio, exceeded, teardown, stale metrics are all RIB state.
The reactor creates them today because the reactor was the only component with a metrics
registry. Now that plugins have their own, these metrics belong in bgp-rib.

The reactor still receives prefix limit events (via bus) and acts on them (teardown).
But the metrics that track prefix counts are RIB state, not reactor state.

### Naming convention uses plugin name, not component name

`ze_fibkernel_*` not `ze_fib_kernel_*`. Plugin names are already lowercase identifiers
without punctuation. Using them directly in metric names avoids mapping ambiguity.

### Not every plugin needs metrics

Plugins that are pure NLRI codecs (flowspec, evpn, labeled, etc.) have no runtime state
to observe. Only plugins with state machines, caches, or I/O operations need metrics.
This spec targets the obvious candidates. Others can be added incrementally.

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
N/A - internal architecture, not protocol work.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-plugin-metrics.md`
- [ ] Summary included in commit
