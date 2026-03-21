# Spec: monitor-1-event-stream

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-20 |

**Spec set:** `spec-monitor-1-event-stream.md` (this), `spec-monitor-2-bgp-dashboard.md` (sibling).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/commands.md` - command architecture
4. `internal/component/plugin/server/monitor.go` - MonitorManager
5. `internal/component/bgp/plugins/cmd/monitor/monitor.go` - current handler

## Task

Rename the current `bgp monitor` event stream to `event monitor`. Move the command handler from the BGP plugin to engine level. Add include/exclude filtering for event types (mutually exclusive). Preserve existing peer and direction filters. Generalize the streaming handler singleton into a prefix-keyed registry to support multiple streaming commands. This frees the `bgp monitor` name for the peer dashboard (spec-monitor-2).

**Ordering note:** spec-monitor-1 and spec-monitor-2 should be committed together. During the gap where spec-1 has removed the old `bgp monitor` event stream but spec-2 hasn't added the dashboard, `bgp monitor` should return an error: `"use 'event monitor' for event streaming, dashboard coming soon"`. This avoids silent breakage.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - command dispatch and streaming handlers
  → Constraint: streaming handlers register via `pluginserver.RegisterStreamingHandler`
  → Decision: SSH exec is the streaming transport, not JSON-RPC
- [ ] `docs/architecture/api/process-protocol.md` - plugin process management
  → Constraint: MonitorManager is parallel to SubscriptionManager (no process.Process for CLI monitors)

### Learned Summaries
- [ ] `plan/learned/396-bgp-monitor.md` - original monitor implementation
  → Decision: SSH-based streaming, not Unix socket JSON-RPC
  → Constraint: StreamingHandler registry pattern (loader must not import plugin implementations)

**Key insights:**
- MonitorManager and Subscription types are already engine-level (`plugin/server/`)
- The command handler (argument parsing, session lifecycle) moves to engine level
- `format.go` (FormatMonitorLine) is deeply BGP-specific (parses `ev.BGP.Peer`, `ev.BGP.Update.NLRI`, etc.) and MUST stay in the BGP subsystem -- it does NOT move. MonitorManager.Deliver() receives pre-formatted `output string` and is already format-agnostic
- Subscription already supports namespace filtering (bgp, rib)
- PeerFilter already supports exclusion (`!10.0.0.1`)
- The existing `subscribe` command (`subscribe.go`) is for plugin processes (SubscriptionManager). `event monitor` is for CLI users (MonitorManager). Different consumers, shared subscription types. Argument parsing can share validation helpers but not the command handler itself

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/monitor/monitor.go` - parses `bgp monitor` arguments (peer, event, direction), registers RPC + streaming handler, StreamMonitor blocks in for/select loop
- [ ] `internal/component/bgp/plugins/cmd/monitor/format.go` - FormatMonitorLine converts JSON events to visual one-liners
- [ ] `internal/component/plugin/server/monitor.go` - MonitorClient, MonitorManager (already engine-level)
- [ ] `internal/component/plugin/server/subscribe.go` - Subscription, PeerFilter, SubscriptionManager (already engine-level)
- [ ] `internal/component/cli/model_monitor.go` - TUI integration: MonitorSession, 50ms polling, Esc to stop
- [ ] `internal/component/bgp/server/events.go` - Deliver() calls on all 6 event hooks

**Behavior to preserve:**
- Streaming over SSH exec (line-by-line formatted events)
- Non-blocking backpressure with atomic drop counter and warning piggybacking
- Peer filtering by IP address or `*` (all peers)
- Direction filtering: `received`, `sent`, or both
- Context-based cleanup (deferred cancel + Remove)
- Authorization check on streaming path
- 256-event buffered channel per client
- FormatMonitorLine visual one-liner output
- TUI integration: 50ms poll, Esc to stop, auto-scroll

**Behavior to change:**
- Command name: `bgp monitor` becomes `event monitor`
- Event type filtering: current `event <type>[,<type>]` (include-only) becomes `include <type>[,<type>]` OR `exclude <type>[,<type>]` (mutually exclusive)
- Handler location: command handler (argument parsing, StreamMonitor loop) moves from BGP plugin to engine-level. FormatMonitorLine stays in BGP subsystem (it is BGP-specific)
- Wire method: changes from `ze-bgp:monitor` to `ze-event:monitor`
- YANG module: new `ze-event-cmd.yang` under a new `event` command tree, registered in `internal/component/plugin/server/schema/` (engine-level YANG, not in a plugin package)
- CLI prefix detection: `model_monitor.go` updates from `"bgp monitor"` to `"event monitor"`
- Streaming handler: the singleton `registeredStreamingHandler` becomes a prefix-keyed registry (`map[string]StreamingHandler`) so multiple streaming commands can coexist. `isStreamingCommand()` checks all registered prefixes. `RegisterStreamingHandler(prefix, handler)` replaces the current no-prefix variant

## Data Flow (MANDATORY)

### Entry Point
- User types `event monitor [include|exclude <types>] [peer <addr>] [direction received|sent]` in CLI
- CLI detects `event monitor` prefix, routes to streaming handler

### Transformation Path
1. CLI sends `event monitor <args>` via SSH exec to daemon
2. SSH streaming executor detects streaming command, calls registered StreamingHandler
3. StreamingHandler parses arguments (include/exclude, peer, direction)
4. StreamMonitor creates MonitorClient with subscriptions, registers with MonitorManager
5. Engine event hooks call MonitorManager.Deliver() for each event
6. MonitorClient receives events on buffered channel
7. StreamMonitor writes formatted lines to SSH writer
8. CLI model polls channel at 50ms, appends to viewport

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI to daemon | SSH exec streaming | [ ] |
| Engine events to monitor | MonitorManager.Deliver() | [ ] |
| Monitor to CLI | Buffered channel + formatted strings | [ ] |

### Integration Points
- `pluginserver.RegisterStreamingHandler(prefix, handler)` - registers `"event monitor"` as a streaming command prefix (new registry-based API, replaces singleton)
- `pluginserver.RegisterRPCs()` - registers the non-streaming RPC with wire method `ze-event:monitor`
- `MonitorManager.Add/Remove/Deliver` - client lifecycle (unchanged)
- `model_monitor.go` - TUI prefix detection and session factory
- `subscribe.go` - shares `validateEventType()` and `validatePeerSelector()` helpers (reuse, not duplicate)

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (event monitor decoupled from BGP plugin)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | via | Feature Code | Test |
|-------------|---|--------------|------|
| `event monitor` CLI command | SSH exec streaming | StreamMonitor handler | `test/plugin/event-monitor-basic.ci` |
| `event monitor include update` | SSH exec streaming | include filtering | `test/plugin/event-monitor-include.ci` |
| `event monitor exclude keepalive` | SSH exec streaming | exclude filtering | `test/plugin/event-monitor-exclude.ci` |
| `event monitor include update peer 10.0.0.1` | SSH exec streaming | combined filters | `test/plugin/event-monitor-peer.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `event monitor` with no filters | Streams all events from all namespaces, all peers, both directions |
| AC-2 | `event monitor include update,state` | Only streams events of type update or state |
| AC-3 | `event monitor exclude keepalive` | Streams all event types except keepalive |
| AC-4 | `event monitor include update exclude state` | Error: include and exclude are mutually exclusive |
| AC-5 | `event monitor include update peer 10.0.0.1` | Streams only update events from peer 10.0.0.1 |
| AC-6 | `event monitor direction received` | Streams only received events |
| AC-7 | `event monitor include invalid-type` | Error with valid event types listed |
| AC-8 | `bgp monitor` no longer starts event stream | `bgp monitor` is freed for dashboard (spec-monitor-2) |
| AC-9 | Old `test/plugin/monitor-*.ci` tests removed or updated | No stale references to old `bgp monitor` event stream |

## Command Syntax

| Command | Behavior |
|---------|----------|
| `event monitor` | All events, all namespaces, all peers, both directions |
| `event monitor include <type>[,<type>]` | Only listed event types |
| `event monitor exclude <type>[,<type>]` | All event types except listed |
| `event monitor peer <addr>` | Filter to specific peer |
| `event monitor peer *` | All peers (explicit) |
| `event monitor direction received` | Received events only |
| `event monitor direction sent` | Sent events only |
| `event monitor include update peer 10.0.0.1 direction received` | All filters combined |

Keywords may appear in any order. Each keyword at most once. `include` and `exclude` are mutually exclusive (error if both present).

### Valid Event Types

| Namespace | Event Types |
|-----------|-------------|
| bgp | update, open, notification, keepalive, refresh, state, negotiated, eor, congested, resumed |
| rib | cache, route |

Event types are validated against the union of all namespaces. An event type is valid if it exists in ANY namespace (bgp OR rib). At delivery time, the subscription filters by both namespace and type -- so `event monitor include cache` will only match rib events (since `cache` is only emitted by rib namespace). This avoids requiring a namespace qualifier in the command syntax while keeping validation strict.

The existing `validateEventType()` in `subscribe.go` takes a namespace parameter. For event monitor, a new `validateEventTypeAnyNamespace()` helper accepts a type valid in any namespace. Reuse the underlying valid-type maps (`plugin.ValidBgpEvents`, `plugin.ValidRibEvents`).

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStreamingHandlerRegistry` | `plugin/server/handler_test.go` | Register multiple prefixes, lookup by command input | |
| `TestStreamingHandlerPrefixMatch` | `plugin/server/handler_test.go` | Longest prefix wins, extracts correct args | |
| `TestParseEventMonitorArgs` | `plugin/server/event_monitor_test.go` | All argument combinations | |
| `TestParseEventMonitorIncludeExcludeMutuallyExclusive` | `plugin/server/event_monitor_test.go` | Error when both include and exclude present | |
| `TestParseEventMonitorInvalidType` | `plugin/server/event_monitor_test.go` | Error on unknown event types | |
| `TestParseEventMonitorDuplicateKeyword` | `plugin/server/event_monitor_test.go` | Error on repeated keywords | |
| `TestValidateEventTypeAnyNamespace` | `plugin/server/event_monitor_test.go` | Accepts types valid in any namespace, rejects unknown | |
| `TestStreamEventMonitorInclude` | `plugin/server/event_monitor_test.go` | Only subscribed types delivered | |
| `TestStreamEventMonitorExclude` | `plugin/server/event_monitor_test.go` | All types except excluded delivered | |

### Boundary Tests (MANDATORY for numeric inputs)
No numeric inputs in this spec.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `event-monitor-basic` | `test/plugin/event-monitor-basic.ci` | `event monitor` streams events | |
| `event-monitor-include` | `test/plugin/event-monitor-include.ci` | `event monitor include update,state` filters correctly | |
| `event-monitor-exclude` | `test/plugin/event-monitor-exclude.ci` | `event monitor exclude keepalive` filters correctly | |
| `event-monitor-peer` | `test/plugin/event-monitor-peer.ci` | `event monitor peer 10.0.0.1` filters by peer | |

## Files to Modify

- `internal/component/plugin/server/event_monitor.go` - NEW: event monitor command handler + argument parser (moved from bgp plugin, without format.go)
- `internal/component/plugin/server/handler.go` - UPDATE: replace singleton `registeredStreamingHandler` with prefix-keyed registry (`map[string]StreamingHandler`), update `RegisterStreamingHandler(prefix, handler)`, update `isStreamingCommand()` to check all registered prefixes, update `extractStreamingArgs()` to strip matched prefix
- `internal/component/bgp/plugins/cmd/monitor/monitor.go` - UPDATE: remove old event stream handler, keep package for `bgp monitor` placeholder (returns error pointing to `event monitor` until spec-2 implements dashboard). Update `RegisterStreamingHandler` call to use prefix form
- `internal/component/bgp/plugins/cmd/monitor/format.go` - KEEP: FormatMonitorLine stays here (BGP-specific). No changes needed -- formatting happens at event emission time in `events.go`, MonitorManager.Deliver() receives pre-formatted strings
- `internal/component/cli/model_monitor.go` - UPDATE: change prefix from `"bgp monitor"` to `"event monitor"`
- `internal/component/ssh/ssh.go` - UPDATE: replace hardcoded `monitorPrefix = "bgp monitor"` with call to streaming handler registry. Replace `isStreamingCommand()` and `extractMonitorArgs()` with registry-based lookup
- `internal/component/plugin/server/schema/ze-event-cmd.yang` - NEW: YANG schema for `event monitor` command tree
- `internal/component/plugin/server/schema/register.go` - NEW or UPDATE: register event YANG module
- `test/plugin/monitor-basic.ci` - UPDATE: change `bgp monitor` to `event monitor`
- `test/plugin/monitor-events.ci` - UPDATE: change `bgp monitor event` to `event monitor include`
- `test/plugin/monitor-peer.ci` - UPDATE: change `bgp monitor peer` to `event monitor peer`
- `docs/architecture/api/commands.md` - UPDATE: document event monitor, streaming handler registry

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/plugin/server/schema/ze-event-cmd.yang` |
| RPC count in architecture docs | [x] | `docs/architecture/api/architecture.md` |
| CLI commands/flags | [x] | streaming handler registry in `handler.go` |
| CLI usage/help text | [x] | help text in RPC registration |
| API commands doc | [x] | `docs/architecture/api/commands.md` |
| Functional test for new RPC/API | [x] | `test/plugin/event-monitor-*.ci` |
| Streaming handler registry | [x] | `handler.go` (singleton to map) + `ssh.go` (prefix lookup) |

## Files to Create

- `internal/component/plugin/server/event_monitor.go` - command handler + argument parser
- `internal/component/plugin/server/event_monitor_test.go` - unit tests
- `internal/component/plugin/server/schema/ze-event-cmd.yang` - YANG schema
- `test/plugin/event-monitor-basic.ci` - functional test
- `test/plugin/event-monitor-include.ci` - functional test
- `test/plugin/event-monitor-exclude.ci` - functional test
- `test/plugin/event-monitor-peer.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
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

1. **Phase: Streaming handler registry** -- replace singleton `registeredStreamingHandler` in `handler.go` with prefix-keyed `map[string]StreamingHandler`. Update `RegisterStreamingHandler(prefix, handler)`, add `GetStreamingHandlerForCommand(input)` that returns the matched handler and extracted args. Update `ssh.go` to use registry-based lookup instead of hardcoded `monitorPrefix`
   - Tests: `TestStreamingHandlerRegistry`, `TestStreamingHandlerPrefixMatch`
   - Files: `handler.go`, `ssh.go`
   - Verify: tests fail then pass. Existing `bgp monitor` still works through registry

2. **Phase: Move handler to engine level** -- move event stream command handler (argument parsing, StreamMonitor loop) from `bgp/plugins/cmd/monitor/monitor.go` to `plugin/server/event_monitor.go`. Register with new prefix `"event monitor"`. FormatMonitorLine stays in BGP subsystem (no move). Wire method: `ze-event:monitor`. YANG: `ze-event-cmd.yang`
   - Tests: `TestParseEventMonitorArgs`
   - Files: `event_monitor.go` (new), `monitor.go` (update registration)
   - Verify: tests fail then pass

3. **Phase: Add include/exclude filtering** -- replace `event <type>` with `include <type>` / `exclude <type>`, enforce mutual exclusivity. Validate event types against union of all namespaces using new `validateEventTypeAnyNamespace()` helper (reuses `plugin.ValidBgpEvents`, `plugin.ValidRibEvents` maps)
   - Tests: `TestParseEventMonitorIncludeExcludeMutuallyExclusive`, `TestParseEventMonitorInvalidType`, `TestStreamEventMonitorInclude`, `TestStreamEventMonitorExclude`
   - Files: `event_monitor.go`
   - Verify: tests fail then pass

4. **Phase: Update CLI prefix and old tests** -- change `model_monitor.go` prefix detection from `"bgp monitor"` to `"event monitor"`. Update existing `test/plugin/monitor-*.ci` tests to use `event monitor` syntax. Update old `bgp monitor` handler to return error pointing to `event monitor`
   - Tests: updated `monitor-*.ci` + new `event-monitor-*.ci`
   - Files: `model_monitor.go`, `monitor-basic.ci`, `monitor-events.ci`, `monitor-peer.ci`, `bgp/plugins/cmd/monitor/monitor.go`
   - Verify: `make ze-verify`

5. **Functional tests** -- create `.ci` tests for event monitor
6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- audit, learned summary, delete spec

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Include/exclude mutual exclusivity enforced, error messages list valid types |
| Naming | Wire method is `ze-event:monitor`, YANG module is `ze-event-cmd`, JSON keys use kebab-case |
| Data flow | Events still flow through MonitorManager.Deliver unchanged. FormatMonitorLine stays in BGP subsystem |
| Rule: no-layering | Old `bgp monitor` event stream handler fully replaced (not kept alongside), old prefix removed from streaming registry |
| Rule: integration-completeness | `event monitor` reachable from CLI via SSH streaming |
| Streaming registry | Singleton replaced with prefix map. No hardcoded prefix strings in `ssh.go` |
| Subscribe alignment | `event monitor` and `subscribe` share validation helpers, not command handlers |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `event monitor` command works | `test/plugin/event-monitor-basic.ci` passes |
| Include filtering works | `test/plugin/event-monitor-include.ci` passes |
| Exclude filtering works | `test/plugin/event-monitor-exclude.ci` passes |
| Old `bgp monitor` event stream removed | grep for `ze-bgp:monitor` returns no handler registrations |
| CLI prefix updated | grep `model_monitor.go` for `"event monitor"` |
| Streaming handler registry works | grep `handler.go` for `map[string]StreamingHandler` |
| YANG module exists | `ls internal/component/plugin/server/schema/ze-event-cmd.yang` |
| format.go stays in BGP | `ls internal/component/bgp/plugins/cmd/monitor/format.go` |
| Old .ci tests updated | grep `test/plugin/monitor-*.ci` for `event monitor` (not `bgp monitor`) |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Event type names validated against known set, peer addresses validated as valid IP |
| Authorization | Streaming path checks `Dispatcher.IsAuthorized()` |
| Resource exhaustion | Channel buffer bounded (256), non-blocking send with drop counter |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
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

N/A -- no RFC requirements for this refactor.

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
- [ ] AC-1..AC-9 all demonstrated
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/399-event-stream.md`
- [ ] Summary included in commit
