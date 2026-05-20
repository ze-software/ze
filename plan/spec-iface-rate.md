# Spec: iface-rate

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/7 |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/iface/iface.go` - InterfaceStats, InterfaceInfo types
4. `internal/component/iface/counters.go` - baseline store pattern
5. `internal/component/iface/backend.go` - Backend interface (ListInterfaces, GetStats)
6. `internal/component/iface/dispatch.go` - package-level dispatch functions
7. `internal/component/iface/register.go` - plugin registration and lifecycle
8. `internal/component/host/metrics.go` - template: RegisterMetrics + StartRefresh pattern
9. `internal/plugins/bfd/metrics.go` - template: ConfigureMetrics + atomic.Pointer pattern
10. `internal/component/cmd/show/show.go` - handleShowInterface dispatch
11. `internal/component/cmd/show/register_netlink_monitor.go` - streaming handler registration
12. `internal/component/plugin/server/handler.go` - StreamingHandler type

## Task

Add per-interface rate/traffic counters to ze. A daemon-side sampler polls
kernel interface stats every 1s, computes byte and packet rates, and exposes
the data through three surfaces:

1. **Prometheus metrics** (`ze_interface_*` gauges on the shared registry)
2. **CLI** (`show interface rate [<name>]` point-in-time, `monitor interface rate [<name>]` streaming)
3. **Web** (rate columns on the interfaces page)

Single sampler, multiple consumers. Follows the `host/metrics.go` pattern:
component owns the ticker, registers gauges directly, no collector framework.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component/plugin registration, lifecycle
  -> Constraint: components register via registry.Registration in init()
  -> Constraint: metrics registry passed via ConfigureMetrics callback
- [ ] `ai/rules/goroutine-lifecycle.md` - ticker goroutine pattern
  -> Constraint: long-lived worker only, ticker + stop channel
- [ ] `ai/rules/json-format.md` - kebab-case JSON keys
  -> Constraint: all JSON keys lowercase kebab-case
- [ ] `ai/rules/design-principles.md` - YAGNI, no premature abstraction
  -> Decision: hardcode 1s interval, no config surface

### Existing Code (read as architecture context)
- [ ] `internal/component/host/metrics.go` - RegisterMetrics + StartRefresh + CollectOnce pattern
  -> Decision: follow this exact pattern for iface rate tracker
- [ ] `internal/plugins/bfd/metrics.go` - ConfigureMetrics + atomic.Pointer + bindMetricsRegistry
  -> Decision: follow this exact pattern for metrics registry injection
- [ ] `internal/component/telemetry/collector/netdev_linux.go` - existing netdev rate computation
  -> Decision: keep as-is for Netdata compatibility, separate concern
- [ ] `internal/component/telemetry/collector/delta_linux.go` - safeDelta helper
  -> Constraint: handle counter wrap (cur < prev -> return 0)
- [ ] `internal/component/cmd/show/register_netlink_monitor.go` - streaming handler pattern
  -> Decision: follow this for monitor interface rate

**Key insights:**
- Single PrometheusRegistry shared by all components, served from one /metrics endpoint
- Two naming families coexist: ze_* (native) and netdata_* (Netdata-compatible)
- iface component currently registers zero Prometheus metrics
- Plugins get registry via Registration.ConfigureMetrics callback, store via atomic.Pointer
- Streaming commands use RegisterStreamingHandler + StreamingHandler func signature
- InterfaceStats has 8 fields: rx/tx bytes, packets, errors, dropped
- Backend.ListInterfaces() returns all interfaces with stats in one syscall
- Counter wrap detection: if cur < prev, treat as kernel reset (return 0 delta)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/iface.go` - InterfaceStats (8 counters), InterfaceInfo, bus topics
- [ ] `internal/component/iface/backend.go` - Backend interface with GetStats, ListInterfaces
- [ ] `internal/component/iface/dispatch.go` - package-level GetStats/ListInterfaces with baseline
- [ ] `internal/component/iface/counters.go` - baseline store for clear counters (software delta)
- [ ] `internal/component/iface/register.go` - Registration{} with ConfigureEventBus, no ConfigureMetrics
- [ ] `internal/component/cmd/show/show.go:627` - handleShowInterface dispatches: brief, type, errors, <name>, <name> counters
- [ ] `internal/plugins/iface/netlink/show_linux.go` - ListInterfaces/GetInterface read link.Attrs().Statistics
- [ ] `internal/plugins/iface/netlink/manage_linux.go:419` - GetStats reads LinkStatistics via netlink

**Behavior to preserve:**
- `show interface` / `show interface <name>` / `show interface brief` / `show interface errors` unchanged
- `show interface <name> counters` continues to return cumulative counters
- `clear interface counters` baseline mechanism in counters.go unchanged
- Existing InterfaceStats type unchanged (new type InterfaceRate added alongside)
- Netdata netdev collector continues to serve `netdata_net_*` metrics independently

**Behavior to change:**
- Add `ConfigureMetrics` callback to iface Registration
- Add `show interface rate [<name>]` as new CLI subcommand
- Add `monitor interface rate [<name>]` as new streaming command

## Data Flow (MANDATORY)

### Entry Point
- Kernel interface statistics via Backend.ListInterfaces() (netlink on Linux)
- Polled every 1s by rate tracker goroutine

### Transformation Path
1. Rate tracker calls Backend.ListInterfaces() -> []InterfaceInfo with Stats
2. Rate tracker computes deltas: (current - previous) / elapsed seconds
3. Rate tracker stores computed InterfaceRate map (RWMutex-protected)
4. Rate tracker updates Prometheus GaugeVec values
5. CLI reads from in-memory rate map via iface.ListRates() / iface.GetRate()
6. Monitor streaming handler reads same data on 1s tick, writes JSON lines

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel -> iface | Backend.ListInterfaces() (netlink syscall) | [ ] |
| iface -> Prometheus | GaugeVec.With(name).Set(value) on shared registry | [ ] |
| iface -> CLI | iface.ListRates() returns map[string]InterfaceRate | [ ] |
| iface -> Monitor | streaming handler reads iface.ListRates() per tick | [ ] |

### Integration Points
- `registry.Registration.ConfigureMetrics` - receives shared metrics.Registry
- `pluginserver.RegisterStreamingHandler` - registers streaming command prefix
- `pluginserver.RegisterRPCs` - registers show RPC wire method
- `iface.Backend.ListInterfaces()` - data source (already exists)

### Architectural Verification
- [ ] No bypassed layers (rate tracker uses existing Backend abstraction)
- [ ] No unintended coupling (iface owns its metrics, no dependency on telemetry collector)
- [ ] No duplicated functionality (extends iface, netdev collector is separate naming/interval)
- [ ] Zero-copy preserved where applicable (stats are value types, copied per tick)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ConfigureMetrics callback | -> | bindMetricsRegistry creates gauges | TestIfaceMetrics_BindRegistry |
| Backend.ListInterfaces() on tick | -> | rateTracker.collect() computes deltas | TestRateTracker_ComputesDelta |
| iface.ListRates() | -> | returns current rate map | TestListRates_ReturnsData |
| iface.GetRate(name) | -> | returns single interface rate | TestGetRate_SingleInterface |
| `show interface rate` RPC | -> | handleShowInterface dispatches to rate | TestShowInterfaceRate_AllInterfaces |
| `show interface rate <name>` RPC | -> | handleShowInterface returns one rate | TestShowInterfaceRate_SingleInterface |
| `monitor interface rate` streaming | -> | streamInterfaceRate writes JSON lines | TestMonitorInterfaceRate_Streaming |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Rate tracker started with metrics registry | 12 ze_interface_* GaugeVec metrics registered |
| AC-2 | Two consecutive ListInterfaces calls 1s apart | InterfaceRate computed with correct bytes/packets per sec |
| AC-3 | Counter wrap (cur < prev) | Rate returns 0 for that interval, no negative values |
| AC-4 | Interface appears between samples | Rate for new interface is 0 (no previous sample) |
| AC-5 | Interface disappears between samples | Stale rate entry and Prometheus labels cleaned up |
| AC-6 | `show interface rate` with no args | JSON array of all interfaces with rate + counter data |
| AC-7 | `show interface rate eth0` | JSON object for eth0 with rate + counter data |
| AC-8 | `show interface rate nonexistent` | Error response, not crash |
| AC-9 | `monitor interface rate` | Streaming JSON lines every 1s until cancelled |
| AC-10 | `monitor interface rate eth0` | Streaming JSON lines for eth0 only |
| AC-11 | Metrics registry not available (telemetry disabled) | Rate tracker still works for CLI, no panic |
| AC-12 | Rate tracker stopped | Goroutine exits cleanly, no leak |
| AC-13 | Web interfaces table page | Rate columns (rx/tx bps, pps) visible in interface table |
| AC-14 | Web interface detail page | Rate data shown in counters tab alongside cumulative stats |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestRateTracker_ComputesDelta | `internal/component/iface/rate_test.go` | Delta computation from two snapshots | |
| TestRateTracker_WrapReturnsZero | `internal/component/iface/rate_test.go` | Counter wrap produces 0, not negative | |
| TestRateTracker_NewInterfaceZeroRate | `internal/component/iface/rate_test.go` | First-seen interface has 0 rate | |
| TestRateTracker_StaleCleanup | `internal/component/iface/rate_test.go` | Disappeared interface removed from map | |
| TestRateTracker_NilMetrics | `internal/component/iface/rate_test.go` | Works without metrics registry (AC-11) | |
| TestIfaceMetrics_BindRegistry | `internal/component/iface/rate_test.go` | 12 gauges registered on registry | |
| TestListRates_ReturnsData | `internal/component/iface/rate_test.go` | Package-level function returns rate map | |
| TestGetRate_SingleInterface | `internal/component/iface/rate_test.go` | Single interface lookup works | |
| TestGetRate_NotFound | `internal/component/iface/rate_test.go` | Missing interface returns error | |
| TestShowInterfaceRate_AllInterfaces | `internal/component/cmd/show/show_test.go` | RPC returns all rates | |
| TestShowInterfaceRate_SingleInterface | `internal/component/cmd/show/show_test.go` | RPC returns one rate | |
| TestMonitorInterfaceRate_Streaming | `internal/component/cmd/show/show_test.go` | Streaming handler registered | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Rate values | 0 - MaxFloat64 | MaxFloat64 | N/A (0 is valid) | N/A |
| Counter values | 0 - MaxUint64 | MaxUint64 | N/A | Wrap handled |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-show-interface-rate | `test/plugin/show-interface-rate.ci` | `show interface rate` returns JSON with rate fields | |

### Future (if deferring any tests)
- None

## Files to Modify
- `internal/component/iface/iface.go` - add InterfaceRate type
- `internal/component/iface/register.go` - add ConfigureMetrics callback, start/stop rate tracker
- `internal/component/iface/dispatch.go` - add ListRates(), GetRate() package-level functions
- `internal/component/cmd/show/show.go` - add `rate` dispatch in handleShowInterface
- `internal/component/web/page_interfaces.go` - add rate columns to table, rate data to detail page

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/iface/schema/ze-iface-cmd.yang` (show interface rate, monitor interface rate) |
| CLI commands/flags | Yes | show.go (RPC), rate streaming handler |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/plugin/show-interface-rate.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - interface rate monitoring |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - show/monitor interface rate |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - ze-show:interface (rate arg), ze-monitor:interface-rate |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - interface rate monitoring capability |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `internal/component/iface/rate.go` - rate tracker: goroutine, snapshot, delta, Prometheus gauges
- `internal/component/iface/rate_test.go` - unit tests for rate tracker
- `internal/component/cmd/show/interface_rate.go` - show interface rate handler + monitor streaming handler
- `test/plugin/show-interface-rate.ci` - functional test

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
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** - register entry points, write failing wiring tests
   - Add ConfigureMetrics to iface Registration in register.go
   - Add InterfaceRate type to iface.go
   - Create rate.go skeleton: rateTracker struct, Start/Stop, empty ListRates/GetRate
   - Create dispatch.go: ListRates(), GetRate() package-level functions
   - Add `rate` dispatch case in handleShowInterface
   - Register streaming handler for `monitor interface rate`
   - Tests: TestIfaceMetrics_BindRegistry, TestListRates_ReturnsData (both fail)
   - Files: iface.go, register.go, rate.go, dispatch.go, show.go, interface_rate.go
   - Verify: entry points exist, wiring tests fail (stub returns empty)

2. **Phase: Rate computation** - delta math, wrap handling, stale cleanup
   - Implement rateTracker.collect(): snapshot, delta, wrap detection
   - Implement Start()/Stop() ticker lifecycle
   - Tests: TestRateTracker_ComputesDelta, TestRateTracker_WrapReturnsZero,
            TestRateTracker_NewInterfaceZeroRate, TestRateTracker_StaleCleanup
   - Files: rate.go, rate_test.go
   - Verify: delta tests pass, wiring tests progress

3. **Phase: Prometheus gauges** - register and update gauges
   - Implement bindMetricsRegistry: create 12 GaugeVec handles
   - Update collect() to set gauge values after delta computation
   - Handle nil metrics (AC-11)
   - Tests: TestIfaceMetrics_BindRegistry, TestRateTracker_NilMetrics
   - Files: rate.go, rate_test.go
   - Verify: metrics registered, nil-safe

4. **Phase: CLI commands** - show + monitor handlers
   - Implement handleShowInterfaceRate for `show interface rate [<name>]`
   - Implement streamInterfaceRate for `monitor interface rate [<name>]`
   - Tests: TestShowInterfaceRate_AllInterfaces, TestShowInterfaceRate_SingleInterface,
            TestMonitorInterfaceRate_Streaming
   - Files: interface_rate.go, show.go
   - Verify: CLI returns JSON with rate data

5. **Phase: YANG schema** - add RPC definitions
   - Add show interface rate and monitor interface rate to YANG
   - Files: ze-iface-cmd.yang (or appropriate YANG module)
   - Verify: YANG compiles, commands visible in help

6. **Functional tests** - create .ci test
   - Files: test/plugin/show-interface-rate.ci
   - Verify: functional test passes

7. **Full verification** - `make ze-verify`

8. **Complete spec** - fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Delta computation handles wrap, elapsed=0, disappearing interfaces |
| Naming | JSON keys use kebab-case, Prometheus metrics use ze_interface_ prefix |
| Data flow | Rate tracker reads raw backend stats (not baseline-adjusted) |
| Goroutine | Ticker goroutine exits on Stop(), no leak |
| Thread safety | RWMutex protects rate map, atomic.Pointer for metrics |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| rate.go exists | `ls internal/component/iface/rate.go` |
| rate_test.go exists | `ls internal/component/iface/rate_test.go` |
| interface_rate.go exists | `ls internal/component/cmd/show/interface_rate.go` |
| 12 Prometheus gauges registered | `grep -c "GaugeVec\|Gauge(" internal/component/iface/rate.go` |
| show interface rate wired | `grep "rate" internal/component/cmd/show/show.go` |
| monitor interface rate wired | `grep "RegisterStreamingHandler.*interface rate" internal/component/cmd/show/interface_rate.go` |
| functional test exists | `ls test/plugin/show-interface-rate.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Interface name validated via iface.ValidateIfaceName before lookup |
| Resource exhaustion | Rate map bounded by number of kernel interfaces (not user-controlled) |
| Goroutine leak | Stop() closes stop channel, ticker.Stop() called |
| Prometheus cardinality | Labels are interface names only (bounded by OS), no user-supplied labels |

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

## Design Decisions

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | Rate tracker in iface component | iface | Owns interface stats already (counters.go, dispatch.go) |
| 2 | Metrics registry injection | ConfigureMetrics + atomic.Pointer | BFD pattern (bfd/register.go:27, bfd/metrics.go:61) |
| 3 | Metric naming | ze_interface_* with {name} label | ze_<subsystem>_<name>_<unit> convention (bfd/metrics.go:66) |
| 4 | Metrics scope | 4 rates + 8 raw counters (12 gauges) | Netdata covers fifo/compressed/etc; YAGNI |
| 5 | Rate data type | InterfaceRate with embedded *InterfaceStats | Reuse existing type, kebab-case JSON |
| 6 | Sample interval | 1s hardcoded | SONiC precedent (1s), EOS (2s); YAGNI for config |
| 7 | CLI grammar | `show interface rate [<name>]` | Grammar: show interface <action> [<interface>] |
| 8 | Netdev collector | Keep as-is | Different interval (10s), different naming (netdata_*) |
| 9 | Goroutine lifecycle | Ticker + stop channel | goroutine-lifecycle.md, host/metrics.go pattern |
| 10 | Platform split | None (rate.go platform-independent) | Backend abstraction handles OS boundary |
| 11 | Monitor command | `monitor interface rate [<name>]` | RegisterStreamingHandler pattern |
| 12 | CLI grammar rule | `show interface <action> [<interface>]` | Prevents keyword/name collision; audit existing commands |

## CLI Grammar Rule (follow-up)

**Rule:** `show interface <action> [<interface>]`. The first token after `interface` is
always a keyword (action), never an interface name. This eliminates ambiguity when an
interface happens to be named the same as a keyword.

**Existing violations to audit after this spec:**
| Current | Correct grammar | Status |
|---------|-----------------|--------|
| `show interface <name>` | `show interface detail <name>` | Audit |
| `show interface <name> counters` | `show interface counters <name>` | Audit |
| `show interface brief` | Already correct (action first) | OK |
| `show interface errors` | Already correct (action first) | OK |
| `show interface type <type>` | Already correct (action first, type is not iface name position) | OK |

This audit is out of scope for this spec. Document as a rule in `.claude/rules/` and
create a separate spec for the migration.

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

N/A - no RFC protocol work.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

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
- [ ] AC-1..AC-14 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
