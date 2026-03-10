# Spec: cli-metrics

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/commands.md` - command handler patterns
4. `internal/component/bgp/plugins/cmd/cache/cache.go` - reference command plugin
5. `internal/core/metrics/prometheus.go` - PrometheusRegistry and Handler()

## Task

Add `ze bgp metrics` CLI commands to view Prometheus metrics from the shell. Two subcommands: `bgp metrics show` (dump Prometheus text format) and `bgp metrics list` (list metric names only). The handler captures Prometheus text output by invoking the registry's HTTP handler with a synthetic request/recorder, then returns the result to the CLI caller.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - command handler patterns, RPC registration
  → Decision: all commands follow RPCRegistration pattern with YANG schema
  → Constraint: handler signature is `func(ctx *CommandContext, args []string) (*plugin.Response, error)`
- [ ] `docs/architecture/core-design.md` - metrics subsystem overview
  → Constraint: PrometheusRegistry is per-instance (not global default)

### RFC Summaries (MUST for protocol work)
N/A - no protocol work.

**Key insights:**
- CommandContext provides access to Server, which provides Reactor. The metrics registry is stored in `plugin/registry.GetMetricsRegistry()` (package-level, type `any`). The handler needs to type-assert to `*metrics.PrometheusRegistry` to call `Handler()`.
- PrometheusRegistry.Handler() returns `http.Handler` that produces Prometheus text format.
- Capturing output: create `httptest.NewRecorder()`, build a synthetic `http.Request` for GET /metrics, call `handler.ServeHTTP(recorder, request)`, read `recorder.Body`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/metrics/prometheus.go` - PrometheusRegistry with Handler() returning http.Handler
  → Constraint: Handler() returns promhttp handler for a private per-instance registry
- [ ] `internal/core/metrics/server.go` - HTTP server that serves /metrics endpoint
  → Constraint: metrics are only accessible via HTTP when telemetry is enabled in config
- [ ] `internal/component/plugin/registry/registry.go` - SetMetricsRegistry/GetMetricsRegistry stores registry as `any`
  → Constraint: stored as `any` to avoid importing metrics package from leaf registry package
- [ ] `internal/component/bgp/plugins/cmd/cache/cache.go` - reference command plugin pattern
  → Constraint: handlers use pluginserver.RegisterRPCs in init(), schema in schema/ subdir
- [ ] `internal/component/bgp/plugins/cmd/cache/doc.go` - reference doc.go pattern
- [ ] `internal/component/bgp/plugins/cmd/cache/schema/embed.go` - reference embed pattern
- [ ] `internal/component/bgp/plugins/cmd/cache/schema/register.go` - reference YANG registration
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - handler registration examples

**Behavior to preserve:**
- PrometheusRegistry is per-instance, not global default
- Metrics registry storage in plugin/registry as `any` (avoids import cycles)
- Existing command plugin file structure: doc.go, schema/, handler .go file

**Behavior to change:**
- None - this is a new command, no existing behavior affected

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- User runs `ze show bgp metrics show` or `ze run bgp metrics show` (read-only, so `ze show` is the natural path)
- CLI dispatches to unix socket, which reaches Server.Dispatch

### Transformation Path
1. CLI text command arrives at Dispatcher via socket or text session
2. Dispatcher matches `bgp metrics show` or `bgp metrics list` to registered handler
3. Handler retrieves metrics registry via `registry.GetMetricsRegistry()`
4. Handler type-asserts to `*metrics.PrometheusRegistry`
5. For `show`: calls `Handler().ServeHTTP(recorder, request)`, returns body as string in Response.Data
6. For `list`: same capture, then parse output to extract metric names only
7. Response returns to CLI caller as JSON with status and data

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ Server | Unix socket IPC, JSON-RPC or text command | [ ] |
| Handler ↔ Registry | `registry.GetMetricsRegistry()` returns `any`, type-assert to `*metrics.PrometheusRegistry` | [ ] |
| Registry ↔ Prometheus | `Handler().ServeHTTP()` captures Prometheus text format | [ ] |

### Integration Points
- `internal/component/plugin/registry/registry.go` - GetMetricsRegistry() provides access to the registry
- `internal/core/metrics/prometheus.go` - PrometheusRegistry.Handler() produces text output
- `internal/component/plugin/server/command.go` - CommandContext, Dispatcher, RPCRegistration
- `internal/component/plugin/server/rpc_register.go` - RegisterRPCs() for init-time registration

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (handler imports registry and metrics packages, both already public within internal)
- [ ] No duplicated functionality (no existing CLI metrics command exists)
- [ ] Zero-copy preserved where applicable (N/A - text output, not wire path)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| `ze show bgp metrics show` via CLI | -> | `handleMetricsShow` handler | `test/plugin/cli-metrics-show.ci` |
| `ze show bgp metrics list` via CLI | -> | `handleMetricsList` handler | `test/plugin/cli-metrics-list.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bgp metrics show` with metrics enabled | Returns Prometheus text format output containing metric names and values |
| AC-2 | `bgp metrics list` with metrics enabled | Returns JSON list of metric name strings only (no values, no help text) |
| AC-3 | `bgp metrics show` with no metrics registry (telemetry disabled) | Returns error response: "metrics not available" |
| AC-4 | `bgp metrics list` with no metrics registry (telemetry disabled) | Returns error response: "metrics not available" |
| AC-5 | Both commands registered as ReadOnly | Commands accessible via `ze show` path |
| AC-6 | YANG module `ze-bgp-cmd-metrics-api` registered | CLI autocomplete and command tree include metrics commands |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMetricsShowWithRegistry` | `internal/component/bgp/plugins/cmd/metrics/metrics_test.go` | Handler returns Prometheus text when registry available | |
| `TestMetricsShowNoRegistry` | `internal/component/bgp/plugins/cmd/metrics/metrics_test.go` | Handler returns error when no registry | |
| `TestMetricsListWithRegistry` | `internal/component/bgp/plugins/cmd/metrics/metrics_test.go` | Handler returns metric names only, no values | |
| `TestMetricsListNoRegistry` | `internal/component/bgp/plugins/cmd/metrics/metrics_test.go` | Handler returns error when no registry | |
| `TestMetricsDispatch` | `internal/component/bgp/plugins/cmd/metrics/dispatch_test.go` | Verifies RPCs are registered and dispatchable | |

### Boundary Tests (MANDATORY for numeric inputs)
No numeric inputs for these commands.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-metrics-show` | `test/plugin/cli-metrics-show.ci` | User runs `ze show bgp metrics show` with telemetry enabled, gets Prometheus text output | |
| `cli-metrics-list` | `test/plugin/cli-metrics-list.ci` | User runs `ze show bgp metrics list` with telemetry enabled, gets metric name list | |

### Future (if deferring any tests)
- None deferred

## Files to Modify
- `docs/architecture/api/commands.md` - add metrics command documentation

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/bgp/plugins/cmd/metrics/schema/ze-bgp-cmd-metrics-api.yang` |
| RPC count in architecture docs | Yes | `docs/architecture/api/architecture.md` |
| CLI commands/flags | No | N/A - auto-registered via RegisterRPCs |
| CLI usage/help text | Yes | Help strings in RPCRegistration |
| API commands doc | Yes | `docs/architecture/api/commands.md` |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/plugin/cli-metrics-show.ci`, `test/plugin/cli-metrics-list.ci` |

## Files to Create
- `internal/component/bgp/plugins/cmd/metrics/doc.go` - package doc with `// Design:` annotation + blank import of schema
- `internal/component/bgp/plugins/cmd/metrics/schema/embed.go` - `//go:embed` of YANG file
- `internal/component/bgp/plugins/cmd/metrics/schema/register.go` - `init()` calling `yang.RegisterModule()`
- `internal/component/bgp/plugins/cmd/metrics/schema/ze-bgp-cmd-metrics-api.yang` - YANG RPC definitions
- `internal/component/bgp/plugins/cmd/metrics/metrics.go` - handlers + `init()` calling `pluginserver.RegisterRPCs()`
- `internal/component/bgp/plugins/cmd/metrics/metrics_test.go` - unit tests for handlers
- `internal/component/bgp/plugins/cmd/metrics/dispatch_test.go` - dispatch registration tests
- `test/plugin/cli-metrics-show.ci` - functional test for `bgp metrics show`
- `test/plugin/cli-metrics-list.ci` - functional test for `bgp metrics list`

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests** -> Review: edge cases? Boundary tests?
2. **Run tests** -> Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement** -> Create files following cache/peer patterns. Minimal code to pass. Handler retrieves registry via `registry.GetMetricsRegistry()`, type-asserts to `*metrics.PrometheusRegistry`, calls `Handler().ServeHTTP()`.
4. **Run tests** -> Verify PASS (paste output). All pass? Any flaky?
5. **RFC refs** -> N/A (not protocol code)
6. **RFC constraints** -> N/A
7. **Functional tests** -> Create `.ci` tests for both commands
8. **Verify all** -> `make ze-test` (lint + all ze tests including fuzz + exabgp)
9. **Critical Review** -> All 6 checks from `rules/quality.md` must pass
10. **Complete spec** -> Fill audit tables, write learned summary

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 (fix syntax/types) |
| Test fails wrong reason | Step 1 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

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

N/A - not protocol code.

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
