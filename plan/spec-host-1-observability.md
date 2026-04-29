# Spec: host-1-observability

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-host-0-inventory (done) |
| Phase | - |
| Updated | 2026-04-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/host/inventory.go` - Inventory struct, Detector, DetectSection
4. `internal/core/metrics/metrics.go` - Registry interface (Counter, Gauge, GaugeVec)
5. `internal/core/report/` - report bus (for hardware-change events)

## Task

Expose host inventory data through the observability stack: Prometheus metrics for
scraping, hardware-change events on the report bus for alerting, and a cached
detector with configurable refresh TTL to avoid re-reading sysfs on every scrape.

**Origin:** deferred from spec-host-0-inventory (items 1-3 in Deferrals table, dated 2026-04-18).

### Sub-tasks

1. **Prometheus `/metrics` endpoint exposing inventory as gauges/counters** -
   `ze_host_memory_total_bytes`, `ze_host_cpu_logical_count`, per-NIC link speed
   and carrier state, per-storage size, thermal readings, ECC error counters.

2. **Hardware-change events on report bus** - detect NIC carrier flip, CPU throttle
   spike, ECC error increment between successive inventory snapshots.

3. **Cached inventory with configurable refresh TTL** - `CachedDetector` wrapping
   `Detector`. Lazy refresh on access, no background goroutines.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - metric collection patterns
- [ ] `internal/core/metrics/metrics.go` - Registry interface
- [ ] `internal/core/metrics/prometheus.go` - PrometheusRegistry implementation
- [ ] `internal/core/report/` - report bus event emission
- [ ] `internal/component/host/inventory.go` - Inventory struct, Detector
- [ ] `internal/component/host/doc.go` - package overview and wiring points

**Key insights:**
- Skeleton: to be filled during design phase

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/host/inventory.go` - Detector.Detect() assembles full Inventory
- [ ] `internal/core/metrics/server.go` - HTTP server serving /metrics
- [ ] `internal/core/report/` - report bus API

**Behavior to preserve:**
- Inventory detection is stateless and safe for concurrent use
- Section-level failures populate Errors but do NOT return an error from Detect
- ErrUnsupported on non-Linux platforms

**Behavior to change:**
- Add metrics registration and collection
- Add cached detection layer
- Add hardware-change event emission

## Data Flow (MANDATORY)

### Entry Point
- Prometheus scrape hits `/metrics` endpoint
- Periodic inventory refresh (driven by scrape interval / TTL)

### Transformation Path
1. Scrape request arrives at metrics HTTP handler
2. Prometheus collects registered gauges
3. Gauges read from CachedDetector (lazy refresh if TTL expired)
4. CachedDetector calls Detector.Detect() on cache miss
5. Diff engine compares previous and current inventory for change events
6. Changed values emitted to report bus

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Host detection ↔ Metrics | RegisterMetrics + CollectOnce | [ ] |
| Host detection ↔ Report bus | Event emission on diff | [ ] |
| HTTP ↔ Prometheus registry | promhttp.Handler | [ ] |

### Integration Points
- `metrics.Registry` - registers gauges/counters
- `report.Event()` - emits change events

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Prometheus scrape | → | HostMetrics.CollectOnce | `TestHostMetricsCollectOnce` |
| CachedDetector.Detect() | → | Detector.Detect() | `TestCachedDetectorTTL` |
| Inventory diff | → | report.Event() | `TestDiffEmitsCarrierChange` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Prometheus scrape on Linux host | `ze_host_memory_total_bytes` gauge present with non-zero value |
| AC-2 | Prometheus scrape on Linux host | `ze_host_cpu_logical_count` gauge matches runtime.NumCPU() |
| AC-3 | Prometheus scrape with NICs | `ze_host_nic_link_speed_mbps{name="eth0"}` gauge present |
| AC-4 | Prometheus scrape with NICs | `ze_host_nic_carrier{name="eth0"}` gauge is 0 or 1 |
| AC-5 | Prometheus scrape on Linux host | `ze_host_uptime_seconds` gauge present |
| AC-6 | Prometheus scrape with ECC memory | `ze_host_ecc_correctable_errors` counter present |
| AC-7 | Prometheus scrape with storage | `ze_host_storage_size_bytes{name="sda"}` gauge present |
| AC-8 | Prometheus scrape with thermal | `ze_host_thermal_temp_mc{name="...",device="..."}` gauge present |
| AC-9 | CachedDetector within TTL | Returns cached result without calling Detect() |
| AC-10 | CachedDetector after TTL | Calls Detect() and refreshes cache |
| AC-11 | CachedDetector.Invalidate() | Next call to Detect() refreshes regardless of TTL |
| AC-12 | NIC carrier changes between snapshots | Report bus receives carrier-change event |
| AC-13 | ECC error count increases | Report bus receives ecc-error event |
| AC-14 | CPU throttle count increases | Report bus receives throttle event |
| AC-15 | Non-Linux platform | No metrics registered, no errors, graceful degradation |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHostMetricsCollectOnce` | `internal/component/host/metrics_test.go` | AC-1..AC-8 | skeleton |
| `TestHostMetricsNilInventory` | `internal/component/host/metrics_test.go` | AC-15 | skeleton |
| `TestCachedDetectorTTL` | `internal/component/host/cached_test.go` | AC-9, AC-10 | skeleton |
| `TestCachedDetectorInvalidate` | `internal/component/host/cached_test.go` | AC-11 | skeleton |
| `TestDiffEmitsCarrierChange` | `internal/component/host/diff_test.go` | AC-12 | skeleton |
| `TestDiffEmitsECCError` | `internal/component/host/diff_test.go` | AC-13 | skeleton |
| `TestDiffEmitsThrottle` | `internal/component/host/diff_test.go` | AC-14 | skeleton |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| TTL | 0-∞ | any Duration | 0 (disables cache) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `host-metrics-scrape` | `test/host/host-metrics-scrape.ci` | Prometheus scrape returns ze_host_* gauges | skeleton |
| `host-cached-refresh` | `test/host/host-cached-refresh.ci` | Two rapid show-host calls return same cached data | skeleton |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/core/metrics/server.go` - wire host metrics registration

## Files to Create
- `internal/component/host/metrics.go` - RegisterMetrics, CollectOnce
- `internal/component/host/metrics_test.go` - unit tests
- `internal/component/host/cached.go` - CachedDetector
- `internal/component/host/cached_test.go` - unit tests
- `internal/component/host/diff.go` - inventory diff engine for change events
- `internal/component/host/diff_test.go` - unit tests

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/monitoring.md` |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist |
| 7-13. | Standard flow |

### Implementation Phases

1. **Phase: Cached Detector** -- CachedDetector with TTL, Invalidate
   - Tests: `TestCachedDetectorTTL`, `TestCachedDetectorInvalidate`
   - Files: `cached.go`, `cached_test.go`
   - Verify: tests fail → implement → tests pass

2. **Phase: Prometheus Metrics** -- RegisterMetrics, CollectOnce
   - Tests: `TestHostMetricsCollectOnce`, `TestHostMetricsNilInventory`
   - Files: `metrics.go`, `metrics_test.go`, `server.go`
   - Verify: tests fail → implement → tests pass

3. **Phase: Inventory Diff** -- diff engine, report bus events
   - Tests: `TestDiffEmitsCarrierChange`, `TestDiffEmitsECCError`, `TestDiffEmitsThrottle`
   - Files: `diff.go`, `diff_test.go`
   - Verify: tests fail → implement → tests pass

4. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Gauge values match inventory values exactly |
| Naming | Prometheus metric names follow `ze_host_` prefix convention |
| Data flow | Metrics read from CachedDetector, not Detector directly |
| Rule: no-layering | No duplicate detection paths |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| metrics.go exists | `ls internal/component/host/metrics.go` |
| cached.go exists | `ls internal/component/host/cached.go` |
| diff.go exists | `ls internal/component/host/diff.go` |
| All tests pass | `go test ./internal/component/host/...` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | No external input; metrics derived from sysfs reads |
| Resource exhaustion | CachedDetector bounds memory to one Inventory |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
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

N/A -- no protocol work.

## Implementation Summary

### What Was Implemented
- Skeleton

### Bugs Found/Fixed
- None

### Documentation Updates
- None

### Deviations from Plan
- None

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
- **Total items:** -
- **Done:** -
- **Partial:** -
- **Skipped:** -
- **Changed:** -

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- Skeleton

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
