# Spec: vpp-6-telemetry — VPP Telemetry and Counters

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-vpp-1-lifecycle |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/spec-vpp-0-umbrella.md` — parent spec
4. `internal/component/vpp/` — VPP component from vpp-1
5. `internal/component/telemetry/` — existing telemetry component

## Task

Read VPP's stats segment for per-interface counters, per-node CPU cycles, and system metrics.
Expose via ze's telemetry component as Prometheus gauges/counters. The stats segment is a
shared-memory region (separate from the binary API), accessed via GoVPP's stats client.

This provides visibility into VPP's forwarding performance that is not available through the
kernel intermediary approach.

### Reference

- GoVPP stats client: statsclient.NewStatsClient, ConnectStats, GetInterfaceStats/GetNodeStats/GetSystemStats
- VPP stats segment documentation: shared memory layout, counter types
- ze telemetry component: existing Prometheus metrics infrastructure

## Required Reading

### Architecture Docs
- [ ] `internal/component/vpp/` — VPP component from vpp-1 (provides stats socket path)
  → Constraint: stats client uses separate socket from binary API
- [ ] `internal/component/telemetry/` — existing telemetry component
  → Constraint: VPP metrics register via same Prometheus registry
- [ ] `internal/plugins/fibvpp/` — fib-vpp plugin from vpp-2
  → Constraint: FIB route count metric sourced from fibvpp installed map

### RFC Summaries (MUST for protocol work)

Not protocol work. No RFCs apply.

**Key insights:**
- VPP stats segment is shared memory, separate from binary API
- GoVPP stats client provides typed access: InterfaceStats, NodeStats, SystemStats
- Polling interval configurable via YANG config
- Metrics registered via ze's existing Prometheus registry
- fibvpp installed map provides fib_routes_installed gauge

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/telemetry/` — existing telemetry infrastructure
  → Constraint: VPP metrics register via same registry pattern
- [ ] `internal/component/vpp/` — VPP component provides stats socket path
  → Constraint: stats client connection separate from binary API connection
- [ ] `internal/plugins/fibvpp/fibvpp.go` — installed map for route count
  → Constraint: route count metric sourced from existing installed map

**Behavior to preserve:**
- Existing telemetry component and metrics unchanged
- VPP component lifecycle unchanged
- fib-vpp plugin unchanged (route count is a read-only metric)

**Behavior to change:**
- VPP stats polling goroutine added to vpp component or as separate plugin
- New Prometheus metrics registered for VPP counters
- Stats client connects to stats socket (separate from API socket)

## Data Flow (MANDATORY)

### Entry Point
- VPP stats segment (shared memory at /run/vpp/stats.sock)
- Polling timer fires at configured interval

### Transformation Path
1. Stats polling goroutine starts in OnStarted (or when VPP connection is ready)
2. ConnectStats to VPP stats socket via GoVPP statsclient
3. On each poll interval:
   a. GetInterfaceStats → per-interface rx/tx packets, bytes, drops, errors
   b. GetNodeStats → per-graph-node clocks/packet, vectors/call
   c. GetSystemStats → vector rate, input rate
4. Convert to Prometheus metrics (gauge/counter as appropriate)
5. Update registered metrics
6. fibvpp installed map length → fib_routes_installed gauge (read from fibvpp)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VPP stats segment → GoVPP stats client | Shared memory read via stats socket | [ ] |
| GoVPP stats client → telemetry | Stats structs converted to Prometheus metrics | [ ] |
| fibvpp installed map → telemetry | Route count read from fibvpp exported function | [ ] |

### Integration Points
- `internal/component/vpp/` — stats socket path from config, connection lifecycle
- `internal/component/telemetry/` — Prometheus metric registration
- `internal/plugins/fibvpp/` — installed route count
- GoVPP statsclient — stats segment access

### Architectural Verification
- [ ] No bypassed layers (stats read from VPP shared memory, not API calls)
- [ ] No unintended coupling (telemetry reads stats, does not affect VPP operation)
- [ ] No duplicated functionality (new metrics, not replacing existing)
- [ ] Zero-copy preserved where applicable (stats are read-only counters)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| VPP stats poll interval | → | Stats client GetInterfaceStats → Prometheus metric | `test/vpp/012-telemetry.ci` |
| Prometheus /metrics endpoint | → | VPP metrics present in scrape output | `test/vpp/012-telemetry.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | VPP running, stats polling enabled | Stats client connected to stats socket |
| AC-2 | Poll timer fires | InterfaceStats, NodeStats, SystemStats read from VPP |
| AC-3 | Prometheus /metrics scraped | `ze_vpp_interface_rx_packets` counter present per interface |
| AC-4 | Prometheus /metrics scraped | `ze_vpp_interface_tx_bytes` counter present per interface |
| AC-5 | Prometheus /metrics scraped | `ze_vpp_interface_drops` counter present per interface |
| AC-6 | Prometheus /metrics scraped | `ze_vpp_node_clocks_per_packet` gauge present per graph node |
| AC-7 | Prometheus /metrics scraped | `ze_vpp_node_vectors_per_call` gauge present per graph node |
| AC-8 | Prometheus /metrics scraped | `ze_vpp_system_vector_rate` gauge present |
| AC-9 | Prometheus /metrics scraped | `ze_vpp_fib_routes_installed` gauge present |
| AC-10 | Prometheus /metrics scraped | `ze_vpp_fib_route_installs_total` counter present |
| AC-11 | VPP restarts | Stats client reconnects, metrics resume after recovery |
| AC-12 | Stats poll interval configured via YANG | Poll frequency matches config |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStatsPollerRun` | `internal/component/vpp/telemetry_test.go` | Poller calls GetInterfaceStats/GetNodeStats/GetSystemStats | |
| `TestInterfaceStatsToMetrics` | `internal/component/vpp/telemetry_test.go` | InterfaceStats converted to correct Prometheus metrics | |
| `TestNodeStatsToMetrics` | `internal/component/vpp/telemetry_test.go` | NodeStats converted to correct Prometheus metrics | |
| `TestSystemStatsToMetrics` | `internal/component/vpp/telemetry_test.go` | SystemStats converted to correct Prometheus metrics | |
| `TestFibRouteCount` | `internal/plugins/fibvpp/stats_test.go` | Installed map length exposed as gauge | |
| `TestStatsReconnect` | `internal/component/vpp/telemetry_test.go` | Stats client reconnects after VPP restart | |
| `TestStatsPollInterval` | `internal/component/vpp/telemetry_test.go` | Config poll interval respected | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| stats poll interval | 1s-3600s | 3600 | 0 | 3601 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-telemetry` | `test/vpp/012-telemetry.ci` | VPP running, Prometheus scrape returns VPP metrics | |

### Future (if deferring any tests)
- Per-prefix counters (VPP counter segment, more complex) deferred
- SNMP export deferred

## Files to Modify

- `internal/component/vpp/vpp.go` — add stats poller lifecycle (start/stop)
- `internal/component/vpp/schema/ze-vpp-conf.yang` — add stats poll interval leaf

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/component/vpp/schema/ze-vpp-conf.yang` (stats section) |
| CLI commands/flags | No | Metrics via Prometheus endpoint |
| Editor autocomplete | Yes | YANG-driven (automatic) |
| Functional test | Yes | `test/vpp/012-telemetry.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — VPP telemetry |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` — stats section |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | Extends VPP component |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` — telemetry section |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — VPP counters |
| 12 | Internal architecture changed? | No | - |

## Files to Create

- `internal/component/vpp/telemetry.go` — Stats poller, metric registration, conversion
- `internal/component/vpp/telemetry_test.go` — Telemetry tests
- `internal/plugins/fibvpp/stats.go` — FIB route count metric export
- `internal/plugins/fibvpp/stats_test.go` — FIB stats tests
- `test/vpp/012-telemetry.ci` — Telemetry functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Stats client connection** — connect to stats socket, lifecycle management
   - Tests: `TestStatsReconnect`, `TestStatsPollInterval`
   - Files: telemetry.go, telemetry_test.go
   - Verify: tests fail → implement → tests pass

2. **Phase: Interface metrics** — per-interface rx/tx/drops counters
   - Tests: `TestStatsPollerRun`, `TestInterfaceStatsToMetrics`
   - Files: telemetry.go, telemetry_test.go
   - Verify: tests fail → implement → tests pass

3. **Phase: Node and system metrics** — per-node and system-wide gauges
   - Tests: `TestNodeStatsToMetrics`, `TestSystemStatsToMetrics`
   - Files: telemetry.go, telemetry_test.go
   - Verify: tests fail → implement → tests pass

4. **Phase: FIB route metrics** — route count from fibvpp installed map
   - Tests: `TestFibRouteCount`
   - Files: fibvpp/stats.go, fibvpp/stats_test.go
   - Verify: tests fail → implement → tests pass

5. **Functional tests** → `test/vpp/012-telemetry.ci`
6. **Full verification** → `make ze-verify`
7. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N metric present in /metrics output |
| Correctness | Counter vs gauge type correct per metric |
| Naming | Metric names follow Prometheus conventions (ze_vpp_ prefix) |
| Data flow | Stats segment → GoVPP stats client → Prometheus metrics |
| Rule: no-layering | Direct stats read, no intermediate storage |
| Reconnect | Stats client recovers after VPP restart |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Telemetry file | `ls internal/component/vpp/telemetry.go` |
| FIB stats file | `ls internal/plugins/fibvpp/stats.go` |
| Metric registration | `grep "ze_vpp_" internal/component/vpp/telemetry.go` |
| Tests | `go test -run TestStats internal/component/vpp/ internal/plugins/fibvpp/` |
| Functional test | `ls test/vpp/012-telemetry.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Stats socket | Validate path from config, no injection |
| Resource usage | Polling interval bounded to prevent tight loop |
| Memory | Stats structs allocated per poll, garbage collected. No leak. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Metrics

| Metric | Source | Type | Labels |
|--------|--------|------|--------|
| `ze_vpp_interface_rx_packets` | InterfaceStats | counter | interface |
| `ze_vpp_interface_tx_packets` | InterfaceStats | counter | interface |
| `ze_vpp_interface_rx_bytes` | InterfaceStats | counter | interface |
| `ze_vpp_interface_tx_bytes` | InterfaceStats | counter | interface |
| `ze_vpp_interface_drops` | InterfaceStats | counter | interface |
| `ze_vpp_interface_rx_errors` | InterfaceStats | counter | interface |
| `ze_vpp_interface_tx_errors` | InterfaceStats | counter | interface |
| `ze_vpp_node_clocks_per_packet` | NodeStats | gauge | node |
| `ze_vpp_node_vectors_per_call` | NodeStats | gauge | node |
| `ze_vpp_system_vector_rate` | SystemStats | gauge | - |
| `ze_vpp_system_input_rate` | SystemStats | gauge | - |
| `ze_vpp_fib_routes_installed` | fibvpp installed map | gauge | - |
| `ze_vpp_fib_route_installs_total` | fibvpp counter | counter | - |

## Stats Client Connection

GoVPP stats client connects to VPP stats segment via shared memory:

| Step | Action |
|------|--------|
| 1 | Create stats client: NewStatsClient(stats-socket-path) |
| 2 | Connect: ConnectStats(statsClient) |
| 3 | Poll loop: sleep(interval), GetInterfaceStats, GetNodeStats, GetSystemStats |
| 4 | On disconnect: retry with backoff (same as binary API connection) |
| 5 | On shutdown: close stats connection |

Stats client is separate from the binary API connection. Both connect to different VPP sockets.

## YANG Config Extension

Added to existing vpp stats container:

| Container | Leaf | Type | Default | Description |
|-----------|------|------|---------|-------------|
| vpp/stats | poll-interval | uint16 | 30 | Stats poll interval in seconds |

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

## Implementation Summary

### What Was Implemented
- (To be filled after implementation)

### Bugs Found/Fixed
- (To be filled)

### Documentation Updates
- (To be filled)

### Deviations from Plan
- (To be filled)

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
- [ ] AC-1..AC-12 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-vpp-6-telemetry.md`
- [ ] Summary included in commit
