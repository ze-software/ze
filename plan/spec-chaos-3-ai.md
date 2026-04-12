# Spec: chaos-ai

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-31 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/chaos-web-dashboard.md` - dashboard architecture, Consumer interface
4. `docs/guide/mcp/overview.md` - ze MCP patterns (tool naming, schemas, error format)
5. `internal/component/mcp/handler.go` - shared MCP protocol to factor out
6. `internal/chaos/report/reporter.go` - Consumer interface for Watchdog
7. `internal/chaos/web/state.go` - DashboardState (data source for MCP tools)
8. `internal/chaos/validation/convergence.go` - convergence tracker (add per-family)
9. `cmd/ze-chaos/main.go` - CLI flags (add --mcp, --ze-mcp, --ai-help)

## Task

Make ze-chaos AI-friendly so Claude can run chaos tests, detect problems, and investigate issues programmatically. Three components:

1. **Shared MCP protocol** -- factor `ToolProvider` interface from existing ze MCP handler so both ze and ze-chaos share the JSON-RPC 2.0 protocol layer
2. **Chaos MCP server** -- 6 tools exposing chaos state for AI queries and control
3. **Watchdog consumer** -- new `report.Consumer` that prints structured `PROBLEM:` lines to stderr for real-time anomaly detection
4. **Per-family convergence** -- extend convergence tracker to break down latency by address family

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/chaos-web-dashboard.md` - Consumer interface, event flow, dashboard state
  -> Constraint: ProcessEvent() runs synchronously on main event loop, must be fast
  -> Decision: Watchdog is a report.Consumer, same plug-in point as dashboard/metrics/jsonlog
- [ ] `docs/guide/mcp/overview.md` - ze MCP tool patterns, naming, schemas, error format
  -> Constraint: tool names use prefix + underscore (ze_announce, chaos_status)
  -> Constraint: responses use textResult/errResult with isError flag
  -> Decision: chaos MCP follows same patterns: typed tools, escape hatch, discovery via tools/list

### RFC Summaries (MUST for protocol work)
N/A -- no BGP protocol changes.

**Key insights:**
- Consumer interface is the plug-in point for both Watchdog and MCP state reads
- ze MCP patterns: prefix naming, enums in schema, examples in descriptions, field-specific errors
- ProcessEvent must be non-blocking -- Watchdog must be fast (just update counters, check thresholds)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mcp/handler.go` - MCP JSON-RPC 2.0 handler with 6 ze tools. Protocol and tools are mixed in one file. server struct holds CommandDispatcher callback.
  -> Constraint: handler.go is 417 lines, single file. Split protocol from tools.
- [ ] `internal/chaos/report/reporter.go` - Consumer interface (ProcessEvent + Close), Reporter fan-out multiplexer
  -> Constraint: ProcessEvent runs synchronously on main goroutine, must be fast
- [ ] `internal/chaos/web/state.go` - DashboardState with per-peer and global counters, convergence histogram, throughput EMA, chaos history, route matrix
  -> Constraint: state is RWMutex-protected, can be read from MCP handler goroutines
- [ ] `internal/chaos/validation/convergence.go` - tracks announcement-to-receipt latency via pending map. No per-family breakdown.
  -> Constraint: pending key is (peer, prefix). Family not stored.
- [ ] `internal/chaos/validation/model.go` - expected route state (per-peer announced sets, global refcount)
- [ ] `internal/chaos/validation/property.go` - 5 properties: route-consistency, convergence-deadline, no-duplicate-routes, hold-timer-enforcement, message-ordering
- [ ] `internal/chaos/scenario/config.go` - ConfigParams with SSHPort, WebUIPort, LGPort fields injected into generated ze config
  -> Decision: add MCPPort field following same pattern
- [ ] `cmd/ze-chaos/main.go` - CLI flags: --web, --ssh, --lg, --web-ui, --metrics, --pprof
  -> Decision: add --mcp, --ze-mcp, --ai-help following same pattern
- [ ] `cmd/ze-chaos/orchestrator_run.go` - setupReporting() creates consumers list, wires dashboard/metrics/jsonlog
  -> Decision: add Watchdog and MCP state provider to consumer list

**Behavior to preserve:**
- Existing Consumer interface unchanged (ProcessEvent + Close)
- Existing ze MCP handler.go API unchanged (Handler function signature)
- Existing ze MCP tools work identically after ToolProvider refactor
- All existing CLI flags and their behavior
- DashboardState thread-safety (RWMutex)
- Event pipeline: synchronous fan-out, unbuffered channel

**Behavior to change:**
- handler.go: extract ToolProvider interface, make tools pluggable
- convergence.go: add per-family latency tracking alongside aggregate
- config.go: add MCPPort field for ze MCP injection
- main.go: add --mcp, --ze-mcp, --ai-help flags
- orchestrator_run.go: add Watchdog consumer and chaos MCP server to setup

## Data Flow (MANDATORY)

### Entry Point
- Events enter via `peer.Event` channel from peer simulators
- MCP requests enter via HTTP POST to chaos MCP port

### Transformation Path
1. Peer simulators emit events to shared channel
2. Orchestrator main loop reads events, dispatches to Reporter.Process()
3. Reporter fans out to all Consumers (dashboard, jsonlog, metrics, Watchdog, MCP state)
4. Watchdog: updates internal counters, checks thresholds, prints PROBLEM lines to stderr
5. MCP state provider: updates queryable snapshot (or reads DashboardState directly)
6. MCP HTTP handler: receives JSON-RPC request, calls ToolProvider.CallTool(), returns JSON-RPC response

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Event loop -> Watchdog | ProcessEvent (synchronous, must be fast) | [ ] |
| Event loop -> MCP state | ProcessEvent updates state; HTTP reads state via RWMutex | [ ] |
| HTTP -> ToolProvider | CallTool(name, args) returns result map | [ ] |
| ze handler -> ToolProvider | Existing ze tools adapted to interface | [ ] |

### Integration Points
- `report.Reporter` -- Watchdog added to consumers list
- `DashboardState` -- MCP tools read from existing state (no duplication)
- `validation.Convergence` -- extended with per-family tracking
- `scenario.ConfigParams` -- MCPPort added for ze MCP injection
- `mcp.Handler()` -- refactored to accept ToolProvider instead of CommandDispatcher

### Architectural Verification
- [ ] No bypassed layers -- Watchdog and MCP both use Consumer interface
- [ ] No unintended coupling -- chaos MCP reads state, does not modify event pipeline
- [ ] No duplicated functionality -- MCP tools read DashboardState, do not re-track counters
- [ ] Zero-copy preserved -- event processing unchanged

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `--mcp :8001` CLI flag | -> | chaos MCP HTTP server starts | `test/chaos-web/mcp-status.ci` |
| `chaos_status` MCP tool call | -> | reads DashboardState, returns JSON | `test/chaos-web/mcp-status.ci` |
| `chaos_problems` MCP tool call | -> | checks Watchdog state, returns issues | `test/chaos-web/mcp-problems.ci` |
| Watchdog detects peer stuck down | -> | prints PROBLEM line to stderr | `TestWatchdogPeerStuckDown` |
| `--ze-mcp 9718` CLI flag | -> | `environment { mcp { port 9718; } }` in generated config | `TestConfigGenerateMCPPort` |
| `--ai-help` CLI flag | -> | prints tool definitions to stdout | `TestAIHelpOutput` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ze MCP handler refactored with ToolProvider | Existing ze MCP functional test (`test/plugin/mcp-announce.ci`) passes unchanged |
| AC-2 | `ze-chaos --mcp :8001` | HTTP server starts on port 8001, responds to `initialize` and `tools/list` |
| AC-3 | `chaos_status` called during run | Returns JSON with seed, elapsed, peers-total, peers-up, routes-announced, routes-received, routes-missing, convergence stats (min/avg/max/p50/p90/p99), convergence-histogram (13 buckets), per-family convergence, chaos-events, throughput, dropped-events, properties |
| AC-4 | `chaos_problems` called when healthy | Returns empty problems list |
| AC-5 | `chaos_problems` called when peer stuck disconnected >30s | Returns problem with type "peer-stuck-down", peer index, address, duration |
| AC-6 | `chaos_problems` called when route plateau detected | Returns problem with type "route-plateau", peer index, current/expected counts, stale duration |
| AC-7 | `chaos_problems` called when routes missing | Returns problem with type "missing-routes", peer index, expected/actual counts, sample prefixes |
| AC-8 | `chaos_problems` called when slow convergence | Returns problem with type "slow-convergence", peer index, prefix, age, deadline |
| AC-9 | `chaos_problems` called when property violated | Returns problem with type "property-violation", property name, violation count, first message |
| AC-10 | `chaos_problems` called when events dropped | Returns problem with type "dropped-events", count |
| AC-11 | `chaos_peers` called with no args | Returns all peers with index, address, asn, ibgp, status, routes-sent/received/missing, families with per-family counts, chaos-count, reconnects, throughput, last-event |
| AC-12 | `chaos_peers` called with peer index | Returns single peer with full detail including recent chaos history (last 5 actions) |
| AC-13 | `chaos_scenario` called | Returns seed, peer-count, peer-profiles (index, address, asn, families, route-target), chaos-rate, duration, warmup, properties-enabled |
| AC-14 | `chaos_control` called with action "pause" | Chaos scheduling pauses, returns confirmation |
| AC-15 | `chaos_control` called with action "resume" | Chaos scheduling resumes |
| AC-16 | `chaos_control` called with action "trigger" and chaos-action name | Specific chaos action triggered on specified peer |
| AC-17 | `chaos_control` called with action "rate" and value | Chaos rate updated |
| AC-18 | `chaos_execute` called with arbitrary command | Command dispatched, output returned |
| AC-19 | Watchdog detects peer not reconnecting after 30s | Prints `PROBLEM: peer N (addr ASN) not reconnected after 30s` to stderr |
| AC-20 | Watchdog detects route count plateau (>10s no change, recv < sent) | Prints `PROBLEM: peer N stuck at M/T routes (no change for Ds)` to stderr |
| AC-21 | Watchdog detects route regression without chaos withdrawal | Prints `PROBLEM: peer N lost M routes (was X, now Y) -- no withdrawal` to stderr |
| AC-22 | Watchdog detects convergence stall (no EOR within 2x warmup) | Prints `PROBLEM: peer N initial sync stalled (no EOR after Ds)` to stderr |
| AC-23 | Watchdog detects EventError | Prints `PROBLEM: peer N error: message` to stderr |
| AC-24 | Watchdog detects EventDroppedEvents | Prints `PROBLEM: peer N dropped M events (overloaded)` to stderr |
| AC-25 | Watchdog detects property pass->fail transition | Prints `PROBLEM: property name FAILED: first violation message` to stderr |
| AC-26 | Same problem repeats within 10s | Prints only once (rate-limited) |
| AC-27 | Per-family convergence tracked | convergence.Stats() includes per-family breakdown in addition to aggregate |
| AC-28 | `--ze-mcp 9718` flag | Generated ze config includes `environment { mcp { port 9718; } }` |
| AC-29 | `--ai-help` flag | Prints chaos MCP tool names, descriptions, and parameters to stdout, then exits |
| AC-30 | Error in MCP tool call (bad args) | Returns field-specific error with isError flag, e.g. "peer must be a valid index: \"abc\"" |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestToolProviderInterface` | `internal/component/mcp/handler_test.go` | ToolProvider interface works with existing ze tools | |
| `TestChaosToolStatus` | `internal/chaos/mcp/tools_test.go` | chaos_status returns expected JSON structure | |
| `TestChaosToolProblems` | `internal/chaos/mcp/tools_test.go` | chaos_problems returns empty when healthy, non-empty when issues exist | |
| `TestChaosToolPeers` | `internal/chaos/mcp/tools_test.go` | chaos_peers returns per-peer data with family breakdown | |
| `TestChaosToolScenario` | `internal/chaos/mcp/tools_test.go` | chaos_scenario returns static metadata | |
| `TestChaosToolControl` | `internal/chaos/mcp/tools_test.go` | chaos_control dispatches pause/resume/trigger/rate/stop | |
| `TestChaosToolErrors` | `internal/chaos/mcp/tools_test.go` | Invalid args return field-specific errors with isError | |
| `TestWatchdogPeerStuckDown` | `internal/chaos/watchdog/watchdog_test.go` | Disconnect event, no Established within timeout -> PROBLEM printed | |
| `TestWatchdogRoutePlateau` | `internal/chaos/watchdog/watchdog_test.go` | Route recv stagnant for >10s while < sent -> PROBLEM printed | |
| `TestWatchdogRouteRegression` | `internal/chaos/watchdog/watchdog_test.go` | Route recv decreased without chaos withdrawal -> PROBLEM printed | |
| `TestWatchdogConvergenceStall` | `internal/chaos/watchdog/watchdog_test.go` | No EOR within 2x warmup -> PROBLEM printed | |
| `TestWatchdogInstantErrors` | `internal/chaos/watchdog/watchdog_test.go` | EventError, EventDroppedEvents -> PROBLEM printed | |
| `TestWatchdogPropertyTransition` | `internal/chaos/watchdog/watchdog_test.go` | Property pass->fail -> PROBLEM printed | |
| `TestWatchdogRateLimit` | `internal/chaos/watchdog/watchdog_test.go` | Same problem within 10s prints only once | |
| `TestConvergencePerFamily` | `internal/chaos/validation/convergence_test.go` | Per-family latency stats tracked alongside aggregate | |
| `TestConfigGenerateMCPPort` | `internal/chaos/scenario/config_test.go` | MCPPort > 0 adds environment { mcp { port N; } } to config | |
| `TestAIHelpOutput` | `cmd/ze-chaos/main_test.go` | --ai-help prints tool definitions and exits | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MCP port | 1-65535 | 65535 | 0 (disabled) | N/A (uint16) |
| ze-mcp port | 1-65535 | 65535 | 0 (disabled) | N/A (uint16) |
| Watchdog reconnect timeout | >0 | 1s | N/A (uses default) | N/A |
| Watchdog plateau duration | >0 | 1s | N/A (uses default) | N/A |
| Rate limit interval | >0 | 1s | N/A (uses default) | N/A |
| chaos_peers peer index | 0 to peer-count-1 | peer-count-1 | -1 (error) | peer-count (error) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mcp-status` | `test/chaos-web/mcp-status.ci` | Start ze-chaos with --mcp, call chaos_status, verify JSON structure | |
| `mcp-problems` | `test/chaos-web/mcp-problems.ci` | Start ze-chaos with --mcp, wait for events, call chaos_problems | |

### Future (if deferring any tests)
- Property-based testing of Watchdog thresholds (requires deterministic virtual clock injection into Watchdog)

## Files to Modify
- `internal/component/mcp/handler.go` - extract ToolProvider interface, adapt ze tools
- `internal/chaos/validation/convergence.go` - add per-family tracking
- `internal/chaos/scenario/config.go` - add MCPPort field, generate mcp config block
- `cmd/ze-chaos/main.go` - add --mcp, --ze-mcp, --ai-help flags
- `cmd/ze-chaos/orchestrator_run.go` - add Watchdog consumer, start chaos MCP server

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A -- ze-chaos does not use YANG |
| CLI commands/flags | Yes | `cmd/ze-chaos/main.go` (--mcp, --ze-mcp, --ai-help) |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/chaos-web/mcp-status.ci`, `test/chaos-web/mcp-problems.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add chaos MCP and Watchdog |
| 2 | Config syntax changed? | No | N/A -- ze-chaos has no config file |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- add --mcp, --ze-mcp, --ai-help |
| 4 | API/RPC added/changed? | Yes | Create `docs/guide/mcp/chaos.md` -- chaos MCP tool reference |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/chaos.md` (new) |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/chaos-web-dashboard.md` -- add MCP and Watchdog sections |

## Files to Create
- `internal/chaos/mcp/tools.go` - chaos ToolProvider implementation with 6 tools
- `internal/chaos/mcp/tools_test.go` - unit tests for chaos tools
- `internal/chaos/watchdog/watchdog.go` - Watchdog report.Consumer implementation
- `internal/chaos/watchdog/watchdog_test.go` - unit tests for Watchdog anomaly detection
- `docs/guide/mcp/chaos.md` - chaos MCP tool reference documentation
- `test/chaos-web/mcp-status.ci` - functional test: chaos MCP status query
- `test/chaos-web/mcp-problems.ci` - functional test: chaos MCP problems query

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-verify` |
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

1. **Phase: ToolProvider interface** -- extract shared MCP protocol from handler.go
   - Tests: `TestToolProviderInterface`
   - Files: `internal/component/mcp/handler.go`
   - Verify: existing `test/plugin/mcp-announce.ci` passes unchanged

2. **Phase: Per-family convergence** -- extend convergence tracker
   - Tests: `TestConvergencePerFamily`
   - Files: `internal/chaos/validation/convergence.go`
   - Verify: existing convergence tests pass, new per-family stats available

3. **Phase: Watchdog consumer** -- anomaly detection printing to stderr
   - Tests: `TestWatchdogPeerStuckDown`, `TestWatchdogRoutePlateau`, `TestWatchdogRouteRegression`, `TestWatchdogConvergenceStall`, `TestWatchdogInstantErrors`, `TestWatchdogPropertyTransition`, `TestWatchdogRateLimit`
   - Files: `internal/chaos/watchdog/watchdog.go`, `internal/chaos/watchdog/watchdog_test.go`
   - Verify: unit tests pass, Watchdog satisfies report.Consumer interface

4. **Phase: Chaos MCP tools** -- implement 6 chaos tools reading DashboardState
   - Tests: `TestChaosToolStatus`, `TestChaosToolProblems`, `TestChaosToolPeers`, `TestChaosToolScenario`, `TestChaosToolControl`, `TestChaosToolErrors`
   - Files: `internal/chaos/mcp/tools.go`, `internal/chaos/mcp/tools_test.go`
   - Verify: unit tests pass, tools return expected JSON structures

5. **Phase: CLI integration** -- wire --mcp, --ze-mcp, --ai-help into ze-chaos
   - Tests: `TestConfigGenerateMCPPort`, `TestAIHelpOutput`
   - Files: `cmd/ze-chaos/main.go`, `cmd/ze-chaos/orchestrator_run.go`, `internal/chaos/scenario/config.go`
   - Verify: flags parsed, MCP server starts, config injection works

6. **Functional tests** -- end-to-end .ci tests
   - Tests: `test/chaos-web/mcp-status.ci`, `test/chaos-web/mcp-problems.ci`
   - Verify: tests pass in `make ze-chaos-test`

7. **Documentation** -- chaos MCP guide, update architecture doc, features list
   - Files: `docs/guide/mcp/chaos.md`, `docs/architecture/chaos-web-dashboard.md`, `docs/features.md`, `docs/guide/command-reference.md`

8. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | ToolProvider refactor does not break existing ze MCP tests |
| Naming | chaos_ tool prefix, kebab-case JSON keys, field-specific error messages |
| Data flow | MCP reads DashboardState via RWMutex, never modifies event pipeline |
| Rule: no-layering | Old Handler(CommandDispatcher) signature replaced, not wrapped |
| Rule: goroutine-lifecycle | Watchdog ProcessEvent is synchronous, no goroutines per event |
| Rule: json-format | All JSON output uses kebab-case keys |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| ToolProvider interface in mcp package | grep "ToolProvider" internal/component/mcp/handler.go |
| Ze tools adapted to ToolProvider | test/plugin/mcp-announce.ci passes |
| 6 chaos MCP tools | grep "chaos_" internal/chaos/mcp/tools.go |
| Watchdog consumer | grep "report.Consumer" internal/chaos/watchdog/watchdog.go |
| 4 stateful anomalies | grep "PROBLEM:" internal/chaos/watchdog/watchdog.go -- 4 distinct stateful checks |
| 4 instant anomalies | grep "PROBLEM:" internal/chaos/watchdog/watchdog.go -- 4 distinct instant checks |
| Rate limiting | grep "rateLimit" internal/chaos/watchdog/watchdog.go |
| Per-family convergence | grep "family" internal/chaos/validation/convergence.go |
| --mcp flag | grep "mcp" cmd/ze-chaos/main.go |
| --ze-mcp flag | grep "ze-mcp" cmd/ze-chaos/main.go |
| --ai-help flag | grep "ai-help" cmd/ze-chaos/main.go |
| MCPPort in ConfigParams | grep "MCPPort" internal/chaos/scenario/config.go |
| Functional tests exist | ls test/chaos-web/mcp-status.ci test/chaos-web/mcp-problems.ci |
| Documentation | ls docs/guide/mcp/chaos.md |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| MCP binds to localhost only | Verify 127.0.0.1 hardcoded, same as ze MCP |
| Request body size limited | maxRequestBody = 1 MB (shared protocol) |
| Content-Type validated | CSRF prevention via JSON-only (shared protocol) |
| No command injection via tool args | chaos tools validate peer index as integer, action as enum |
| chaos_execute scope | If included: what commands can it dispatch? Limit to read-only or document risk |
| Rate limiting | Watchdog stderr output is rate-limited (no log flood from fast event loop) |

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
| Existing mcp-announce.ci fails after ToolProvider refactor | Phase 1 regression -- fix before proceeding |

## ToolProvider Interface

| Method | Signature | Purpose |
|--------|-----------|---------|
| ServerName | returns string | MCP server name ("ze-mcp" or "ze-chaos-mcp") |
| Tools | returns tool definitions list | For tools/list response |
| CallTool | accepts name + raw JSON args, returns result map | For tools/call dispatch |

Ze daemon adapter: wraps existing CommandDispatcher + tool handlers.
Chaos adapter: wraps DashboardState + Watchdog + control channel reads.

## Chaos MCP Tools

### chaos_status -- full snapshot

| Field | Type | Source |
|-------|------|--------|
| `seed` | integer | DashboardState.Seed |
| `elapsed` | string (duration) | time.Since(StartTime) |
| `peers-total` | integer | PeerCount |
| `peers-up` | integer | PeersUp |
| `peers-syncing` | integer | PeersSyncing |
| `routes-announced` | integer | TotalAnnounced |
| `routes-received` | integer | TotalReceived |
| `routes-missing` | integer | TotalMissing |
| `routes-withdrawn` | integer | TotalWithdrawn |
| `sync-duration` | string (duration) | SyncDuration |
| `convergence` | object | ConvergenceHistogram + Percentiles |
| `convergence.min` | string (duration) | Histogram.Min |
| `convergence.avg` | string (duration) | Histogram.Avg() |
| `convergence.max` | string (duration) | Histogram.Max |
| `convergence.p50` | string (duration) | Percentiles.P50 |
| `convergence.p90` | string (duration) | Percentiles.P90 |
| `convergence.p99` | string (duration) | Percentiles.P99 |
| `convergence.histogram` | array of objects | 13 buckets with label + count |
| `convergence.per-family` | object | Family name -> min/avg/max/p50/p90/p99 |
| `chaos-events` | integer | TotalChaos |
| `chaos-rate` | number | chaosRate (EMA events/sec) |
| `throughput-in` | string | AggregateThroughput formatted |
| `throughput-out` | string | AggregateThroughput formatted |
| `dropped-events` | integer | TotalDropped |
| `properties` | array of objects | name, status (pass/fail), violation-count |

### chaos_problems -- filtered actionable issues

Returns JSON array. Empty array = healthy.

| Problem Type | Fields |
|-------------|--------|
| `missing-routes` | peer (index), address, expected (int), actual (int), missing-count (int), sample-prefixes (first 5 strings) |
| `extra-routes` | peer, address, count, sample-prefixes |
| `slow-convergence` | peer, address, prefix, source-peer, age (duration), deadline (duration) |
| `peer-stuck-down` | peer, address, asn, down-since (timestamp), duration |
| `route-plateau` | peer, address, current (int), expected (int), stale-duration |
| `dropped-events` | count |
| `property-violation` | property-name, violation-count, first-message |

### chaos_peers -- per-peer detail

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `peer` | integer | No | Peer index for detail view. Omit for all peers summary. |

Per-peer fields: index, address, asn, ibgp (bool), status, routes-sent, routes-received, missing, families (array of objects with name, sent, received, target), chaos-count, reconnects, bytes-sent, bytes-received, throughput-in, throughput-out, last-event, last-event-at.

When single peer requested: adds recent-chaos (last 5 actions with time and action name).

### chaos_scenario -- static metadata

No parameters. Returns seed, peer-count, ibgp-count, peer-profiles (array of index/address/asn/ibgp/families/route-target/hold-time), chaos-rate, chaos-interval, duration, warmup, properties-enabled.

### chaos_control -- scheduling control

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `pause`, `resume`, `trigger`, `rate`, `stop` |
| `peer` | integer | No | Peer index for trigger action |
| `chaos-action` | string | No | Chaos action name for trigger (e.g. "tcp-disconnect") |
| `value` | number | No | New rate for rate action (0.0-1.0) |

Description steers AI: "Control chaos scheduling. Use chaos_problems or chaos_status first to understand the situation before changing anything."

### chaos_execute -- escape hatch

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | Yes | Command to execute |

Description: "Execute a chaos orchestrator command. Prefer the specific tools (chaos_status, chaos_problems, chaos_peers, chaos_control) when possible."

## Watchdog Consumer

### Configuration

| Parameter | Default | Purpose |
|-----------|---------|---------|
| reconnect-timeout | 30s | Time after disconnect before alerting |
| plateau-duration | 10s | Time of no route change before alerting |
| warmup-multiplier | 2x | EOR timeout = warmup * multiplier |
| rate-limit | 10s | Same problem suppressed for this duration |

### Stateful Anomalies (require tracking over time)

| Anomaly | State Tracked | Detection | Output |
|---------|--------------|-----------|--------|
| Peer not reconnecting | map of peer -> disconnect time | No Established within reconnect-timeout after Disconnected | `PROBLEM: peer N (addr ASXXX) not reconnected after 30s` |
| Route count plateau | map of peer -> (last-recv-count, last-change-time) | recv < sent AND no change for plateau-duration | `PROBLEM: peer N stuck at M/T routes (no change for Ds)` |
| Route regression | map of peer -> prev-recv-count | recv decreased AND no chaos withdrawal pending | `PROBLEM: peer N lost M routes (was X, now Y) -- no withdrawal` |
| Convergence stall | map of peer -> established-time | No EOR within warmup-multiplier * warmup after Established | `PROBLEM: peer N initial sync stalled (no EOR after Ds)` |

### Instant Anomalies (single event triggers)

| Anomaly | Trigger Event | Output |
|---------|--------------|--------|
| Error | EventError | `PROBLEM: peer N error: message` |
| Dropped events | EventDroppedEvents | `PROBLEM: peer N dropped M events (overloaded)` |
| Extra routes | RouteConsistency property violation with "unexpected" | `PROBLEM: peer N received unknown route prefix` |
| Property violation | Any property transitions pass -> fail | `PROBLEM: property name FAILED: first-violation-message` |

### Rate Limiting

Each (anomaly-type, peer-index) pair is rate-limited. Last-printed time stored in a map. If current time - last-printed < rate-limit (default 10s), the problem is suppressed. Different peers with the same anomaly type print independently.

## Per-Family Convergence

### Changes to convergence.go

Current pending key: (peer-index, prefix).
Add: family field to pendingEntry.

New method: `StatsByFamily() map[string]ConvergenceStats` -- returns per-family breakdown.
Each family gets independent min/max/avg/p99 computed from its subset of resolved latencies.

Storage: `familyLatencies map[string][]time.Duration` alongside existing aggregate `latencies []time.Duration`.

The aggregate stats remain unchanged. Per-family is additive.

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

N/A -- no BGP protocol changes.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

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
- [ ] AC-1..AC-30 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all checks in `rules/quality.md` -- no failures)

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
- [ ] Write learned summary to `plan/learned/499-chaos-ai.md`
- [ ] **Summary included in commit**
