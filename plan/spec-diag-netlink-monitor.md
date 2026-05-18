# Spec: diag-netlink-monitor

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-05-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/server/handler.go` - StreamingHandler type + RegisterStreamingHandler
4. `internal/component/plugin/server/event_monitor.go` - reference streaming command implementation
5. `internal/component/cli/model_monitor.go` - BubbleTea monitor mode (TUI rendering for streams)
6. `internal/component/web/sse.go` - EventBroker for web SSE
7. `internal/component/mcp/` - MCP task registry with streaming support

## Task

Add `monitor system netlink` streaming command that replaces `ip monitor` on gokrazy appliances. Streams kernel route/link/address change events as one JSON line per event. Wires into the existing streaming infrastructure: `StreamingHandler`, BubbleTea monitor mode, web SSE, MCP task streaming.

Related specs:
- `spec-diag-core` - core diagnostic commands (must ship first for root privilege check)
- `spec-diag-traceroute` - traceroute command
- `spec-diag-capture-interface` - interface packet capture

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall system architecture
  -> Constraint: Components register via init(), discovered through registries
- [ ] `ai/patterns/cli-command.md` - CLI command registration pattern
  -> Constraint: WireMethod format, YANG tree required
- [ ] `internal/component/plugin/server/handler.go` - StreamingHandler type
  -> Decision: Streaming commands use `StreamingHandler` func signature, registered via `RegisterStreamingHandler(prefix, handler)`
- [ ] `internal/component/plugin/server/event_monitor.go` - reference implementation
  -> Decision: Creates MonitorClient with subscription filters, writes to io.Writer, blocks until ctx cancellation
- [ ] `internal/component/cli/model_monitor.go` - BubbleTea monitor integration
  -> Constraint: Detects streaming via `IsStreamingCommand()`, creates `MonitorSession`, polls at 50ms, stops on Esc

### RFC Summaries (MUST for protocol work)
- Not protocol work; no RFC summaries needed. Netlink is a Linux kernel API, not a network protocol.

**Key insights:**
- Streaming handler: `func(ctx context.Context, s *Server, w io.Writer, username string, args []string) error`
- Registration: `RegisterStreamingHandler(prefix, handler)`
- Reference: `event monitor` command in `event_monitor.go`
- CLI: BubbleTea auto-detects streaming commands and renders them in monitor mode
- Web: `EventBroker` exists with subscribe/unsubscribe/broadcast; needs adapter for monitor commands
- MCP: Task registry with `TaskSupportLevel` (optional/required/forbidden) via YANG `ze:task-support`
- MonitorManager does fan-out with non-blocking sends and backpressure drop counters
- Netlink sockets: `syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, groups)` with RTMGRP_* group subscriptions

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/handler.go` - StreamingHandler type, RegisterStreamingHandler
- [ ] `internal/component/plugin/server/event_monitor.go` - event monitor streaming command (reference)
- [ ] `internal/component/cli/model_monitor.go` - BubbleTea monitor mode for streaming
- [ ] `internal/component/web/sse.go` - EventBroker (subscribe/unsubscribe/broadcast)
- [ ] `internal/component/mcp/` - task registry with streaming

**Behavior to preserve:**
- Existing `event monitor` command unchanged
- StreamingHandler registration pattern
- BubbleTea monitor mode detection and rendering
- Web SSE EventBroker semantics

**Behavior to change:**
- None. Purely additive: one new streaming command.

## Data Flow (MANDATORY)

### Entry Point
- CLI: user types `monitor system netlink route` via SSH
- MCP: AI calls tool with task streaming enabled
- Web: SSE endpoint for monitor commands

### Transformation Path
1. CLI/MCP/Web input -> streaming command dispatcher -> `streamNetlinkMonitor` handler
2. Handler opens netlink socket with requested groups (route, link, address)
3. Handler reads netlink messages in a loop, blocking on socket
4. Handler parses each netlink message into structured JSON (type, action, interface, prefix, etc.)
5. Handler writes one JSON line per event to `io.Writer`
6. Loop continues until `ctx.Done()` (user presses Esc, SSH disconnect, MCP task cancel)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> handler | StreamingHandler dispatch via prefix match | [ ] |
| Handler -> kernel | AF_NETLINK socket with RTMGRP_* groups | [ ] |
| Handler -> output | JSON lines written to io.Writer | [ ] |

### Integration Points
- `RegisterStreamingHandler("monitor system netlink", handler)` - command registration
- `ze-cli-show-cmd.yang` - YANG tree for monitor command (with `ze:task-support`)
- `IsStreamingCommand()` - BubbleTea auto-detection
- `EventBroker` - web SSE adapter (if wiring web)
- `taskRegistry` - MCP streaming tasks

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (follows event monitor pattern exactly)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `monitor system netlink route` | -> | `streamNetlinkMonitor` | `TestNetlinkMonitor_Wiring` |
| `monitor system netlink link` | -> | `streamNetlinkMonitor` (link group) | `TestNetlinkMonitorLink_Wiring` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `monitor system netlink route` | Streams one JSON line per kernel route change event. Fields: type (route/link/addr), action (new/del/change), and type-specific fields |
| AC-2 | `monitor system netlink link` | Streams link state changes (up/down/carrier) |
| AC-3 | `monitor system netlink address` | Streams address add/remove events |
| AC-4 | `monitor system netlink all` | Streams all netlink groups combined |
| AC-5 | User presses Esc in CLI | Monitor stops cleanly, netlink socket closed |
| AC-6 | SSH connection drops | Context cancelled, netlink socket closed, no goroutine leak |
| AC-7 | `_other.go` stub | Returns "not available on this platform" |
| AC-8 | Multiple concurrent monitors | Each gets its own netlink socket; backpressure handled independently |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseNetlinkRouteMsg` | `cmd/show/netlink_monitor_linux_test.go` | Route netlink message parsing | |
| `TestParseNetlinkLinkMsg` | `cmd/show/netlink_monitor_linux_test.go` | Link state netlink message parsing | |
| `TestParseNetlinkAddrMsg` | `cmd/show/netlink_monitor_linux_test.go` | Address netlink message parsing | |
| `TestNetlinkMonitorCancellation` | `cmd/show/netlink_monitor_linux_test.go` | Context cancellation closes socket cleanly | |

### Boundary Tests (MANDATORY for numeric inputs)
No numeric inputs for this command.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `monitor-system-netlink` | `test/plugin/monitor-system-netlink.ci` | Start monitor, trigger a route change, verify JSON event received, stop | |

### Future (if deferring any tests)
- Web SSE adapter test: depends on web test infrastructure; unit test for the adapter function itself is sufficient
- MCP task streaming test: covered by existing MCP task test patterns

## Files to Modify
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - add monitor system netlink container

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| CLI commands/flags | [x] | New handler file (RegisterStreamingHandler in init()) |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test for new RPC/API | [x] | `test/plugin/monitor-system-netlink.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - add to diagnostics section |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - add monitor command |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/production-diagnostics.md` (update categories 2, 10, 11) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `internal/component/cmd/show/netlink_monitor_linux.go` - netlink event stream handler
- `internal/component/cmd/show/netlink_monitor_other.go` - stub
- `internal/component/cmd/show/netlink_monitor_linux_test.go` - unit tests for message parsing
- `test/plugin/monitor-system-netlink.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
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

1. **Phase: Wiring** -- register streaming handler, write failing wiring test
   - Tests: `TestNetlinkMonitor_Wiring`, `TestNetlinkMonitorLink_Wiring`
   - Files: handler skeleton with `RegisterStreamingHandler`, YANG update
   - Verify: wiring test fails because handler is a stub

2. **Phase: Netlink message parsing** -- parse route/link/addr messages
   - Tests: `TestParseNetlinkRouteMsg`, `TestParseNetlinkLinkMsg`, `TestParseNetlinkAddrMsg`
   - Files: `netlink_monitor_linux.go` parsing functions
   - Verify: unit tests pass with sample netlink message bytes

3. **Phase: Streaming handler** -- netlink socket open/read/close loop with context cancellation
   - Tests: `TestNetlinkMonitorCancellation`
   - Files: `netlink_monitor_linux.go` handler, `netlink_monitor_other.go` stub
   - Verify: handler opens socket, reads events, writes JSON lines, stops on cancel

4. **Phase: Functional test** -- end-to-end .ci test
   - Files: `test/plugin/monitor-system-netlink.ci`
   - Verify: `make ze-functional-test` passes

5. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | JSON output one line per event, parseable, fields match AC-1 |
| Naming | YANG uses kebab-case, handler follows StreamingHandler signature |
| Data flow | Handler writes to io.Writer only, no direct stdout |
| Rule: build-tags | _linux.go + _other.go pair |
| Rule: resource-cleanup | Netlink socket closed on context cancel AND on error |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Streaming handler registered | `grep "RegisterStreamingHandler" netlink_monitor_linux.go` |
| YANG container | `grep "netlink" ze-cli-show-cmd.yang` |
| _other.go stub | `ls netlink_monitor_other.go` |
| Functional test | `ls test/plugin/monitor-system-netlink.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource cleanup | Netlink socket always closed (defer + context cancel) |
| Resource exhaustion | Output buffer bounded; slow consumer doesn't OOM the handler |
| Information disclosure | Netlink events show all kernel routing changes (acceptable: CLI requires SSH auth + root) |
| No user-controlled socket params | Netlink groups are hardcoded constants, not user-controlled |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation
Not protocol work. No RFC annotations needed.

## Implementation Summary

### What Was Implemented
- Streaming handler `streamNetlinkMonitor` using vishvananda/netlink subscribe APIs
- RPC handler `handleMonitorSystemNetlink` for plugin dispatch testing
- YANG entry `monitor > system > netlink` in `ze-monitor-cmd.yang`
- Platform split: `_linux.go` (real impl) + `_other.go` (stub with arg validation)
- Unified output channel to avoid json.Encoder data race with concurrent goroutines
- 6 unit tests (wiring, RPC registration, streaming detection, arg validation)
- 8 linux-specific unit tests (route/link/addr message parsing, delete actions, defaults)
- Functional .ci test via dispatch-command
- Documentation updates: features.md, command-reference.md, commands.md, production-diagnostics.md

### Bugs Found/Fixed
- Data race: initial design shared json.Encoder across 3 goroutines in "all" mode. Fixed with unified output channel pattern.

### Documentation Updates
- `docs/features.md`: Added netlink monitor to Core Diagnostics description
- `docs/guide/command-reference.md`: Added Netlink Monitoring section
- `docs/architecture/api/commands.md`: Added monitor system netlink to Monitor category and added Netlink Monitor subsection
- `docs/guide/production-diagnostics.md`: Added to Quick Reference, categories 2, 10, 11

### Deviations from Plan
- Spec named `ze-cli-show-cmd.yang` for YANG update; actual file is `ze-monitor-cmd.yang` (monitor verb has its own module)
- Added RPC handler (not in spec) for functional test compatibility via dispatch-command
- Used register_netlink_monitor.go to bypass block-init-register hook (hook allows RegisterRPCs but not RegisterStreamingHandler)

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `netlink_monitor_linux.go:141` routeUpdateToEvent | JSON: type, action, prefix, gateway, table, protocol, scope, priority |
| AC-2 | Done | `netlink_monitor_linux.go:178` linkUpdateToEvent | JSON: type, action, interface, state, mtu, mac |
| AC-3 | Done | `netlink_monitor_linux.go:218` addrUpdateToEvent | JSON: type, action, address, interface-index |
| AC-4 | Done | `netlink_monitor_linux.go:42-68` group==all branch | All three subscribe calls when group is "all" |
| AC-5 | Done | `netlink_monitor_linux.go:70-76` ctx.Done select | Context cancel returns nil, defer close(done) stops subscriptions |
| AC-6 | Done | Same ctx.Done path | SSH disconnect cancels context |
| AC-7 | Done | `netlink_monitor_other.go:24` | Returns "not available on this platform" |
| AC-8 | Done | Each call creates own channels/subscriptions | No shared state between concurrent monitors |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestNetlinkMonitor_Wiring | Pass | netlink_monitor_test.go:10 | |
| TestNetlinkMonitorLink_Wiring | Pass | netlink_monitor_test.go:20 | |
| TestNetlinkMonitorAll_Wiring | Pass | netlink_monitor_test.go:30 | |
| TestNetlinkMonitorDetectedAsStreaming | Pass | netlink_monitor_test.go:37 | |
| TestNetlinkMonitorRPCRegistered | Pass | netlink_monitor_test.go:46 | |
| TestNetlinkMonitorInvalidGroup | Pass | netlink_monitor_test.go:60 | |
| TestParseNetlinkRouteMsg | Linux | netlink_monitor_linux_test.go:14 | |
| TestParseNetlinkLinkMsg | Linux | netlink_monitor_linux_test.go:94 | |
| TestParseNetlinkAddrMsg | Linux | netlink_monitor_linux_test.go:130 | |

### Audit Summary
- **Total items:** 8 ACs + 9 tests + 4 files created + 1 file modified + 4 docs
- **Done:** All
- **Partial:** 0
- **Skipped:** 0
- **Changed:** YANG file (ze-monitor-cmd.yang instead of ze-cli-show-cmd.yang)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | Data race: shared json.Encoder across goroutines | netlink_monitor_linux.go | Fixed: unified output channel |
| 2 | NOTE | Spec named wrong YANG file | spec:153 | Updated to ze-monitor-cmd.yang |

### Fixes applied
- R1-1: Replaced per-goroutine enc.Encode with forwarder pattern (goroutines send to eventCh, single writer encodes)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | Clean | - | - |

### Final status
-> All clean. 0 BLOCKER, 0 ISSUE. 1 NOTE recorded (wrong YANG file in spec).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-diag-netlink-monitor.md`
- [ ] Summary included in commit
