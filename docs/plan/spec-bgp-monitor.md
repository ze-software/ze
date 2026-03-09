# Spec: bgp-monitor

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/architecture.md` — API architecture, connection types, event formats
4. `internal/component/plugin/server/client.go` — current Client type + clientLoop
5. `internal/component/plugin/server/subscribe.go` — SubscriptionManager + ParseSubscription
6. `internal/component/plugin/events.go` — event type constants
7. `internal/component/bgp/server/events.go` — event dispatch to plugins

## Task

Add a `bgp monitor` CLI command that streams live BGP events over the Unix socket, with keyword-based filtering for event type, peer, and direction. Inspired by VyOS `monitor protocol bgp`. The connection stays open and events stream continuously until the client disconnects (Ctrl-C).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API architecture, connection types, event delivery
  → Constraint: CLI clients use NUL-framed JSON-RPC over Unix socket
  → Constraint: Event formats already defined (JSON + text, parsed/raw/full)
  → Decision: `Request.More` and `RPCResult.Continues` already defined in wire protocol
- [ ] `docs/architecture/api/commands.md` - existing command patterns
  → Constraint: keyword-based syntax, no `--` flags

### RFC Summaries (MUST for protocol work)
N/A — this is an operational/CLI feature, not a protocol extension.

**Key insights:**
- `Request.More` (rpc/message.go:16) and `RPCResult.Continues` (rpc/message.go:23) are defined but unused — designed for exactly this use case
- SubscriptionManager is keyed on `*process.Process` — CLI clients have no Process. MonitorManager is a parallel type for CLI monitor subscriptions.
- Event dispatch in `events.go` has 6 event functions that all need monitor delivery: `onMessageReceived`, `onMessageBatchReceived`, `onMessageSent`, `onPeerStateChange`, `onPeerNegotiated`, `onEORReceived`
- Formatting cache in event functions is per (format+encoding) key. Monitor format+encoding combinations must be added to this cache to avoid duplicate formatting.
- `clientLoop` dispatch chain (`rpcDispatcher.Dispatch` → `wrapHandler`) returns a single response. Monitor must be intercepted before dispatch — streaming handler called directly with the writer.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/client.go` (105L) — Client struct, clientLoop: reads one request, dispatches, writes one response, loops. No streaming support.
  → Constraint: clientLoop is synchronous request-response; streaming requires restructuring the loop or a separate goroutine
- [ ] `internal/component/plugin/server/server.go` (409L) — Server struct, acceptLoop, wrapHandler. Server has `subscriptions *SubscriptionManager`.
  → Constraint: `wrapHandler` returns `(any, error)` — single response. Streaming needs a different dispatch path.
- [ ] `internal/component/plugin/server/subscribe.go` (285L) — Subscription, SubscriptionManager, ParseSubscription. Subscription matching logic is reusable.
  → Decision: Reuse `Subscription` type and `ParseSubscription()` for monitor filter parsing
  → Constraint: `SubscriptionManager.GetMatching()` returns `[]*process.Process` — monitor clients need a parallel path
- [ ] `internal/component/bgp/plugins/cmd/subscribe/subscribe.go` (100L) — subscribe/unsubscribe handlers. Require `ctx.Process != nil`.
  → Constraint: Cannot reuse subscribe handler directly for CLI clients
- [ ] `internal/component/plugin/events.go` (53L) — Event namespace/type/direction constants. All reusable.
- [ ] `internal/component/bgp/server/events.go` (507L) — Event processing: `onMessageReceived`, `onPeerStateChange`, etc. Pre-formats once per (format+encoding), delivers to matched processes.
  → Decision: Monitor delivery must hook into the same formatting path to avoid duplicate formatting
- [ ] `internal/component/plugin/process/delivery.go` (248L) — `deliveryLoop`, `EventDelivery`, batch drain from eventChan. Process-specific.
  → Constraint: Cannot reuse deliveryLoop for monitor clients without creating a Process
- [ ] `pkg/plugin/rpc/message.go` (32L) — `Request.More`, `RPCResult.Continues` defined but unused.
  → Decision: Use these fields for monitor streaming protocol
- [ ] `cmd/ze/cli/main.go` (753L) — CLI client, longest-prefix matching, pipe operators.
  → Constraint: CLI currently sends one request, reads one response. Must handle streaming reads for monitor.

**Behavior to preserve:**
- Existing subscribe/unsubscribe commands for plugin processes unchanged
- SubscriptionManager API unchanged
- Event dispatch pipeline for plugins unchanged
- clientLoop for non-monitor requests unchanged
- JSON and text event formats unchanged

**Behavior to change:**
- clientLoop intercepts monitor wire method before rpcDispatcher, calls streaming handler
- CLI client gains streaming receive loop for `continues:true` responses
- All 6 event functions in `bgp/server/events.go` gain monitor delivery after plugin delivery
- Server struct gains MonitorManager field (parallel to existing SubscriptionManager)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- CLI user types: `bgp monitor [peer <addr>] [event <types>] [direction <dir>] [encoding <enc>]`
- CLI client sends JSON-RPC: `{"method":"ze-bgp:monitor","params":{"args":["peer","10.0.0.1","event","update"]},"id":1,"more":true}`

### Transformation Path
1. **CLI dispatch** — `cmd/ze/cli/main.go`: longest-prefix matches `"bgp monitor"` → wire method `"ze-bgp:monitor"`. Sends request with `more: true`. Enters streaming receive loop.
2. **Server intercept** — `client.go clientLoop`: receives request, checks method name. Monitor method detected → calls streaming handler directly (bypasses `rpcDispatcher.Dispatch`).
3. **Monitor registration** — Streaming handler parses args, creates MonitorClient, registers with `MonitorManager.Add()`. Sends initial `RPCResult{Continues: true}` confirmation frame. Enters select loop draining `eventChan` → writer.
4. **Event occurs (received)** — Reactor fires event → `EventDispatcher` → `events.go` functions (`onMessageReceived`, `onMessageBatchReceived`, `onPeerStateChange`, `onPeerNegotiated`, `onEORReceived`).
5. **Event occurs (sent)** — Reactor sends message → `EventDispatcher` → `events.go` `onMessageSent()`.
6. **Monitor delivery** — Each event function, after delivering to plugin processes (existing path), also delivers to matching monitors. Format cache is extended to include monitor format+encoding combinations. MonitorManager enqueues formatted string to each matching monitor's `eventChan`.
7. **Client receives** — CLI client reads each frame, applies pipe operators, displays to user.
8. **Disconnect (client)** — Client closes connection (Ctrl-C). Next `writer.Write()` in streaming handler returns broken-pipe error. Handler returns → deferred `MonitorManager.Remove()` cleans up. `clientLoop` deferred cleanup closes connection and removes client.
9. **Disconnect (server)** — Server shuts down. `client.ctx` cancelled → streaming handler's `ctx.Done()` fires → handler returns → same cleanup path as step 8.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ Engine | NUL-framed JSON-RPC over Unix socket, `more:true` / `continues:true` | [ ] |
| Reactor ↔ EventDispatcher | `OnMessageReceived()` / `OnPeerStateChange()` (unchanged) | [ ] |
| EventDispatcher ↔ MonitorManager | New: after formatting events, deliver to monitor clients | [ ] |

### Integration Points
- `bgp/server/events.go` — all 6 event functions gain monitor delivery after plugin delivery
- `plugin/server/server.go` — add MonitorManager field + `Monitors()` accessor
- `plugin/server/client.go` — clientLoop intercepts monitor method before rpcDispatcher
- `cmd/ze/cli/main.go` — streaming receive loop for `continues:true` responses
- `rpc/message.go` — use existing `More`/`Continues` fields (no changes needed)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Design

### Dispatch Transition: clientLoop Intercept

The existing dispatch chain (`clientLoop` → `rpcDispatcher.Dispatch` → `wrapHandler` → single response) cannot support streaming — `wrapHandler` returns `(any, error)` and `Dispatch` returns a single `RPCResult`/`RPCError`. The handler has no access to the `FrameWriter`.

**Solution:** `clientLoop` intercepts the monitor method *before* calling `rpcDispatcher.Dispatch`. After unmarshaling the request, check the method name. If it matches the monitor wire method, call a dedicated streaming handler on the Server that receives the client, the writer, and the request. This handler does not return until the monitor session ends (client disconnect or server shutdown). Normal clientLoop cleanup (deferred `removeClient`, `conn.Close`) runs when the handler returns.

| clientLoop Step | Normal Path | Monitor Path |
|----------------|-------------|--------------|
| Read + unmarshal request | same | same |
| Check method | not monitor → dispatch | is monitor → streaming handler |
| Dispatch | `rpcDispatcher.Dispatch(req)` | skipped |
| Streaming handler | N/A | parses args, registers monitor, writes events until disconnect |
| Write response | `writeRPCResponse(writer, result)` | N/A (handler writes directly) |
| Loop | next request | returns (clientLoop exits, deferred cleanup runs) |

The monitor wire method constant is defined alongside the RPC registration in the monitor plugin package.

### MonitorManager

New type in `internal/component/plugin/server/monitor.go`. Manages active monitor clients parallel to SubscriptionManager (which manages plugin process subscriptions).

| Field | Type | Purpose |
|-------|------|---------|
| `mu` | `sync.RWMutex` | Thread-safe access |
| `monitors` | `map[string]*MonitorClient` | Client ID → monitor state |

MonitorManager is a field on `Server` (alongside `subscriptions`), exposed via `Server.Monitors()` accessor — same pattern as `Server.Subscriptions()`. This allows `bgp/server/events.go` (different package) to access it through the `pluginserver.Server` parameter it already receives.

### MonitorClient

| Field | Type | Purpose |
|-------|------|---------|
| `id` | `string` | Client ID (from Client.id) |
| `subscriptions` | `[]*Subscription` | Event filters (one per event type — comma-separated types expand to multiple subscriptions) |
| `eventChan` | `chan string` | Buffered channel for formatted events (size: 256) |
| `encoding` | `string` | "json" or "text" |
| `format` | `string` | "parsed", "raw", "full" |
| `ctx` | `context.Context` | Client-scoped context for cancellation |
| `cancel` | `context.CancelFunc` | Cancel function for cleanup |
| `dropped` | `atomic.Uint64` | Count of events dropped due to full channel |

### Multiple Event Types → Multiple Subscriptions

When the user specifies `event update,state`, the arg parser splits on comma and creates one `Subscription` per event type. The MonitorClient holds all of them. `MonitorManager.GetMatching()` checks all subscriptions for each monitor, same logic as `SubscriptionManager.GetMatching()` — match any subscription, add monitor once.

If no `event` keyword is specified, one subscription per valid BGP event type is created (all 8 types from `events.go`).

### Monitor Protocol

| Step | Direction | Message |
|------|-----------|---------|
| 1. Request | CLI → Engine | `{"method":"ze-bgp:monitor","params":{"args":[...]},"id":1,"more":true}` |
| 2. Confirm | Engine → CLI | `{"result":{"status":"streaming","subscriptions":[...]},"id":1,"continues":true}` |
| 3. Events | Engine → CLI | `{"result":{"event":<formatted-event>},"id":1,"continues":true}` (repeated) |
| 4. End | Engine → CLI | Connection closed (server shutdown) or client disconnects |

### CLI Keyword Grammar

```
bgp monitor [peer <addr>] [event <type>[,<type>]] [direction received|sent] [encoding json|text] [format parsed|raw|full]
```

All keywords are optional. Defaults: all peers, all BGP events, both directions, json encoding, parsed format.

| Keyword | Values | Default | Example |
|---------|--------|---------|---------|
| `peer` | IP address, `*` | all peers | `peer 10.0.0.1` |
| `event` | comma-separated event types | all BGP events | `event update,state` |
| `direction` | `received`, `sent` | both | `direction received` |
| `encoding` | `json`, `text` | `json` | `encoding text` |
| `format` | `parsed`, `raw`, `full` | `parsed` | `format full` |

### Event Delivery to Monitors

**All 6 event functions** in `bgp/server/events.go` must deliver to monitors:

| Function | Event Type | Direction |
|----------|-----------|-----------|
| `onMessageReceived` | update, open, notification, keepalive, refresh | received |
| `onMessageBatchReceived` | update (batch) | received |
| `onMessageSent` | update, open, notification, keepalive, refresh | sent |
| `onPeerStateChange` | state | N/A |
| `onPeerNegotiated` | negotiated | N/A |
| `onEORReceived` | eor | received |

Each function already pre-formats events per (format+encoding) key for plugin processes. Monitor delivery extends this:

1. After building the `formatOutputs` map for plugin processes, also check active monitors (via `s.Monitors().GetFormatKeys(namespace, eventType, direction, peer)`) for additional format+encoding combinations not already in the map.
2. Format any missing combinations using the existing `formatMessageForSubscription()` / `format.FormatStateChange()` / etc.
3. Call `s.Monitors().Deliver(namespace, eventType, direction, peer, formatOutputs)` — MonitorManager matches each monitor's subscriptions, looks up the formatted output by the monitor's format+encoding key, and enqueues to the monitor's `eventChan`.

This avoids duplicate formatting: if a plugin and a monitor share the same format+encoding, the event is formatted once.

### Streaming Handler and Disconnect Detection

The streaming handler (called from clientLoop intercept) follows this sequence:

1. Parse monitor args → on error, write `RPCError` to writer, return immediately.
2. Create `MonitorClient` with buffered `eventChan` and client-scoped context.
3. Register with `MonitorManager.Add()`.
4. Write confirmation frame (`RPCResult{Continues: true}`) to writer.
5. Enter select loop: read from `eventChan` → write frame to writer, or `ctx.Done()` → return.
6. Deferred: `MonitorManager.Remove(id)` cleans up on any exit path.

**Disconnect detection:** When the client disconnects (Ctrl-C closes the Unix socket), the next `writer.Write()` returns a broken-pipe error. The select loop detects this write error and returns, triggering cleanup. No separate reader goroutine is needed — write errors are sufficient for Unix socket disconnect detection.

For server shutdown: `client.ctx` is derived from `server.ctx`. When the server shuts down, `client.ctx` is cancelled, the select loop's `ctx.Done()` case fires, and the handler returns cleanly.

### Backpressure

Monitor `eventChan` buffer size: 256. When enqueuing, use non-blocking send. If the channel is full (slow client), drop the event and increment the `dropped` counter.

The dropped-event warning is piggybacked on the next successfully delivered event: before writing the event frame, check `dropped`. If non-zero, swap the counter to zero and prepend a warning frame. This avoids the problem of trying to send a warning to an already-full channel.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CLI `bgp monitor` command | → | `handleMonitor()` in monitor plugin | `test/plugin/monitor-basic.ci` |
| Event dispatch with active monitor | → | `MonitorManager.Deliver()` in events.go | `test/plugin/monitor-events.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bgp monitor` with no args, peer sends UPDATE | Client receives formatted UPDATE event as streaming JSON-RPC frame |
| AC-2 | `bgp monitor event state`, peer goes up/down | Client receives state events only (no updates, keepalives, etc.) |
| AC-3 | `bgp monitor peer 10.0.0.1`, events from two peers | Client receives events only from 10.0.0.1 |
| AC-4 | `bgp monitor event update,state` | Client receives both update and state events, nothing else |
| AC-5 | Client disconnects (Ctrl-C) during monitoring | Server cleans up monitor entry, no goroutine leak |
| AC-6 | `bgp monitor encoding text` | Events delivered in text format instead of JSON |
| AC-7 | `bgp monitor direction received` | Only received events (not sent) |
| AC-8 | Invalid keyword/value in monitor args | Error response with clear message, connection not held open |
| AC-9 | `bgp monitor \| match 10.0.0.0/24` | Pipe operators apply to streamed output (client-side filtering) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseMonitorArgs` | `internal/component/bgp/plugins/cmd/monitor/monitor_test.go` | Keyword parsing: peer, event, direction, encoding, format | |
| `TestParseMonitorArgsMultipleEvents` | same | Comma-separated event types expand correctly | |
| `TestParseMonitorArgsInvalid` | same | Invalid keywords/values return errors | |
| `TestParseMonitorArgsDefaults` | same | No args → all events, all peers, both directions, json, parsed | |
| `TestMonitorManagerAddRemove` | `internal/component/plugin/server/monitor_test.go` | Add/remove monitor clients | |
| `TestMonitorManagerGetMatching` | same | Subscription matching against events | |
| `TestMonitorManagerCleanup` | same | Client disconnect cleans up state | |
| `TestMonitorDelivery` | same | Events delivered to matching monitors, not non-matching | |
| `TestMonitorBackpressure` | same | Full channel drops events, counter increments | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| eventChan buffer | 256 | N/A (internal) | N/A | N/A |

No user-facing numeric inputs in this feature.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-monitor-basic` | `test/plugin/monitor-basic.ci` | Start monitor, peer sends UPDATE, verify event received | |
| `test-monitor-event-filter` | `test/plugin/monitor-events.ci` | Monitor with event filter, verify only matching events | |
| `test-monitor-peer-filter` | `test/plugin/monitor-peer.ci` | Monitor with peer filter, verify only matching peer | |

### Future (if deferring any tests)
- Property testing (fuzz with random event streams) — deferrable, not user-facing
- Benchmarks for monitor delivery overhead — deferrable, performance optimization

## Files to Modify
- `internal/component/plugin/server/server.go` — add MonitorManager field to Server struct
- `internal/component/plugin/server/client.go` — add monitor streaming path in clientLoop
- `internal/component/bgp/server/events.go` — add monitor delivery after plugin delivery
- `cmd/ze/cli/main.go` — add streaming receive mode for monitor responses

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/bgp/schema/ze-bgp-api.yang` |
| RPC count in architecture docs | [x] | `docs/architecture/api/architecture.md` |
| CLI commands/flags | [x] | `cmd/ze/cli/main.go` (auto-dispatched via command map) |
| CLI usage/help text | [x] | Help text in RPC registration |
| API commands doc | [x] | `docs/architecture/api/commands.md` |
| Plugin SDK docs | [ ] | N/A — not an SDK feature |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/monitor-*.ci` |

## Files to Create
- `internal/component/bgp/plugins/cmd/monitor/monitor.go` — monitor command handler + arg parsing
- `internal/component/bgp/plugins/cmd/monitor/monitor_test.go` — unit tests for arg parsing
- `internal/component/bgp/plugins/cmd/monitor/doc.go` — package doc + plugin registration
- `internal/component/plugin/server/monitor.go` — MonitorManager type
- `internal/component/plugin/server/monitor_test.go` — MonitorManager unit tests
- `test/plugin/monitor-basic.ci` — functional test: basic monitoring
- `test/plugin/monitor-events.ci` — functional test: event filtering
- `test/plugin/monitor-peer.ci` — functional test: peer filtering

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for arg parsing** → Review: edge cases? All keyword combinations?
2. **Run tests** → Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement `parseMonitorArgs()`** → Minimal code to pass. Reuse `ParseSubscription` logic.
4. **Run tests** → Verify PASS (paste output). All pass? Any flaky?
5. **Write unit tests for MonitorManager** → Add/remove, matching, cleanup, delivery, backpressure.
6. **Run tests** → Verify FAIL.
7. **Implement MonitorManager** → Channel-based delivery with goroutine per monitor.
8. **Run tests** → Verify PASS.
9. **Wire monitor into Server** — Add MonitorManager field, wire into event dispatch.
10. **Wire monitor into clientLoop** — Add streaming path when monitor request received.
11. **Wire CLI streaming receive** — Read loop for `continues:true` frames.
12. **Register RPC** — YANG schema + handler registration via init().
13. **Functional tests** → Create `.ci` tests.
14. **Verify all** → `make ze-verify`
15. **Critical Review** → All 6 checks from `rules/quality.md` must pass.
16. **Complete spec** → Fill audit tables, write learned summary.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 7 (fix syntax/types) |
| Test fails wrong reason | Step 1 or 5 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation

N/A — operational feature, no RFC requirements.

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
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

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
