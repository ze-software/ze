# Spec: bgp-monitor

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-03-17 |

## Revision History

| Date | Change |
|------|--------|
| 2026-03-09 | Initial spec (assumed Unix socket + JSON-RPC transport) |
| 2026-03-17 | Corrected transport to SSH-based streaming. Added interactive BubbleTea mode. Added visual text output format. Removed references to nonexistent client.go/clientLoop/NUL-framed JSON-RPC. |
| 2026-03-17b | Output is always JSON (NDJSON). Visual text format is the default client-side rendering. All pipe operators (table, json, yaml, match) work because underlying data is JSON. Removed encoding/format keywords -- engine always sends JSON parsed format. |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/ssh/ssh.go` — SSH server, execMiddleware (dispatch path for CLI commands)
4. `internal/component/plugin/server/server.go` — Server struct, MonitorManager field
5. `internal/component/plugin/server/subscribe.go` — SubscriptionManager + ParseSubscription
6. `internal/component/plugin/events.go` — event type constants
7. `internal/component/bgp/server/events.go` — event dispatch to plugins
8. `internal/component/bgp/config/loader.go:493-514` — executor factory wiring
9. `cmd/ze/cli/main.go` — CLI client (SSH exec path)

## Task

Add a `bgp monitor` CLI command that streams live BGP events via SSH, with keyword-based filtering for event type, peer, and direction. Inspired by VyOS `monitor protocol bgp`. The connection stays open and events stream continuously until the client disconnects (Ctrl-C). Supports both SSH exec mode (`ze cli --run "bgp monitor"`) and interactive BubbleTea mode (type `bgp monitor` in the TUI). Output is designed for human readability with timestamps, direction indicators, and event-type-specific summaries.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API architecture, connection types, event delivery
  → Constraint: Event formats already defined (JSON + text, parsed/raw/full)
  → Decision: CLI path goes through SSH, not Unix socket JSON-RPC
- [ ] `docs/architecture/api/commands.md` - existing command patterns
  → Constraint: keyword-based syntax, no `--` flags

### RFC Summaries (MUST for protocol work)
N/A — this is an operational/CLI feature, not a protocol extension.

**Key insights:**
- CLI dispatch path: SSH exec → `execMiddleware()` → `CommandExecutorFactory(username)` → `Dispatcher.Dispatch(ctx, input)` → `Handler` → returns `(string, error)`. This is request-response; monitor needs streaming.
- SubscriptionManager is keyed on `*process.Process` — CLI clients have no Process. MonitorManager is a parallel type for CLI monitor subscriptions.
- Event dispatch in `events.go` has 6 event functions that all need monitor delivery: `onMessageReceived`, `onMessageBatchReceived`, `onMessageSent`, `onPeerStateChange`, `onPeerNegotiated`, `onEORReceived`
- Formatting cache in event functions is per (format+encoding) key. Monitor format+encoding combinations must be added to this cache to avoid duplicate formatting.
- `Dispatcher.Dispatch()` returns a single `(*plugin.Response, error)`. Monitor handler must write streaming output directly to the SSH session writer, bypassing the single-response path.
- `execMiddleware` in `ssh.go` has access to the `ssh.Session` writer. For monitor, it keeps the session open and writes events line-by-line.
- Pipe operators (`| match`, `| count`, etc.) are in `internal/component/command/pipe.go` — client-side filters applied to output. For monitor in exec mode, the client applies pipes per event line. In BubbleTea interactive mode, the model applies pipes as events arrive.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ssh/ssh.go` (408L) — SSH server, execMiddleware: receives command string via SSH exec, dispatches through executor factory, writes single response, session exits.
  → Constraint: execMiddleware is request-response; streaming requires keeping the session open and writing continuously
- [ ] `internal/component/ssh/session.go` (55L) — createSessionModel: builds BubbleTea model for interactive SSH sessions with executor wired in.
  → Decision: Interactive monitor uses the BubbleTea model's streaming capability
- [ ] `internal/component/plugin/server/server.go` (345L) — Server struct, wrapHandler. Server has `subscriptions *SubscriptionManager`.
  → Constraint: `wrapHandler` returns `(any, error)` — single response. Streaming needs a different dispatch path.
- [ ] `internal/component/plugin/server/subscribe.go` (285L) — Subscription, SubscriptionManager, ParseSubscription. Subscription matching logic is reusable.
  → Decision: Reuse `Subscription` type and `ParseSubscription()` for monitor filter parsing
  → Constraint: `SubscriptionManager.GetMatching()` returns `[]*process.Process` — monitor clients need a parallel path
- [ ] `internal/component/cmd/subscribe/subscribe.go` (100L) — subscribe/unsubscribe handlers. Require `ctx.Process != nil`.
  → Constraint: Cannot reuse subscribe handler directly for CLI clients
- [ ] `internal/component/plugin/events.go` (53L) — Event namespace/type/direction constants. All reusable.
- [ ] `internal/component/bgp/server/events.go` (507L) — Event processing: `onMessageReceived`, `onPeerStateChange`, etc. Pre-formats once per (format+encoding), delivers to matched processes.
  → Decision: Monitor delivery must hook into the same formatting path to avoid duplicate formatting
- [ ] `internal/component/bgp/config/loader.go:493-514` — Executor factory wiring: `SetExecutorFactory` creates per-session executors that call `Dispatcher.Dispatch`. Monitor needs a parallel streaming executor.
  → Decision: Add `StreamingExecutorFactory` for monitor commands
- [ ] `cmd/ze/cli/main.go` (341L) — CLI client, SSH exec via `sshclient.ExecCommand()`. Single request-response.
  → Constraint: `ExecCommand` uses `session.CombinedOutput()` — waits for session to end. Monitor needs streaming SSH session.
- [ ] `cmd/ze/internal/sshclient/sshclient.go` (166L) — SSH client helper: `ExecCommand` opens session, runs `CombinedOutput`, returns string.
  → Decision: Add `StreamCommand` that opens an SSH session, reads stdout line-by-line, and calls a callback per line until disconnect.
- [ ] `internal/component/command/pipe.go` (80L+) — Pipe operators: ParsePipe, ApplyPipes. Client-side filtering.
  → Decision: For monitor, pipe operators apply per-event-line (not to a single blob)

**Behavior to preserve:**
- Existing subscribe/unsubscribe commands for plugin processes unchanged
- SubscriptionManager API unchanged
- Event dispatch pipeline for plugins unchanged
- execMiddleware for non-monitor commands unchanged
- JSON and text event formats unchanged
- Pipe operators unchanged (reused for monitor filtering)

**Behavior to change:**
- SSH execMiddleware gains monitor detection: keeps session open for streaming
- SSH Server gains `StreamingExecutorFactory` field for monitor commands
- CLI sshclient gains `StreamCommand` for line-by-line streaming SSH sessions
- BubbleTea model gains monitor mode for interactive streaming
- All 6 event functions in `bgp/server/events.go` gain monitor delivery after plugin delivery
- Plugin Server struct gains MonitorManager field (parallel to existing SubscriptionManager)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point (SSH exec mode)
- CLI user runs: `ze cli --run "bgp monitor [peer <addr>] [event <types>] [direction <dir>] [encoding <enc>]"`
- CLI calls `sshclient.StreamCommand(creds, "bgp monitor ...")` which opens an SSH session and reads stdout line-by-line

### Entry Point (Interactive BubbleTea mode)
- User types `bgp monitor [args]` in the interactive CLI
- Model detects monitor command prefix, enters monitor mode, wires streaming executor

### Transformation Path
1. **SSH exec** — `sshclient.StreamCommand()`: opens SSH session, sends command string as exec command, reads stdout line-by-line into a callback (print to terminal). Does NOT use `CombinedOutput()`.
2. **SSH server intercept** — `ssh.go execMiddleware()`: detects command starts with monitor prefix. Instead of calling `executor(input)` (single response), calls `streamingExecutor(sess, input)` which receives the `ssh.Session` writer for continuous output.
3. **Streaming executor dispatch** — The streaming executor calls `Dispatcher.Dispatch()` with a special CommandContext that carries a `MonitorWriter` (the SSH session writer). The monitor handler receives this writer.
4. **Monitor handler** — `handleMonitor()` in the monitor plugin: parses args, creates MonitorClient with buffered eventChan, registers with `MonitorManager.Add()`. Writes a header line to the writer. Enters select loop: read from eventChan, format the event line, write to writer. Returns only on disconnect or server shutdown.
5. **Event occurs** — Reactor fires event → `EventDispatcher` → `events.go` functions (`onMessageReceived`, `onMessageBatchReceived`, `onPeerStateChange`, `onPeerNegotiated`, `onEORReceived`, `onMessageSent`).
6. **Monitor delivery** — Each event function, after delivering to plugin processes (existing path), also delivers to matching monitors. Format cache is extended to include monitor format+encoding combinations. MonitorManager enqueues formatted string to each matching monitor's eventChan.
7. **Client displays** — SSH exec mode: `StreamCommand` reads each line, applies pipe operators (if any), prints to terminal. Interactive mode: BubbleTea model appends each event line to viewport, auto-scrolls.
8. **Disconnect (client)** — Client closes SSH session (Ctrl-C). Next `sess.Write()` in streaming handler returns error. Handler returns, deferred `MonitorManager.Remove()` cleans up.
9. **Disconnect (server)** — Server shuts down. SSH server context cancelled → streaming handler's select loop exits → same cleanup path.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ Engine | SSH exec session, streaming text lines (one event per line) | [ ] |
| Reactor ↔ EventDispatcher | `OnMessageReceived()` / `OnPeerStateChange()` (unchanged) | [ ] |
| EventDispatcher ↔ MonitorManager | New: after formatting events, deliver to monitor clients | [ ] |

### Integration Points
- `bgp/server/events.go` — all 6 event functions gain monitor delivery after plugin delivery
- `plugin/server/server.go` — add MonitorManager field + `Monitors()` accessor
- `ssh/ssh.go` — execMiddleware detects monitor command, calls streaming executor
- `ssh/ssh.go` — Server gains StreamingExecutorFactory field
- `bgp/config/loader.go` — wire streaming executor factory alongside regular executor factory
- `cmd/ze/cli/main.go` — `--run "bgp monitor"` calls `StreamCommand` instead of `ExecCommand`
- `cmd/ze/internal/sshclient/sshclient.go` — add `StreamCommand` function
- `internal/component/cli/model_mode.go` — BubbleTea model gains monitor mode for interactive streaming

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Design

### SSH Streaming: execMiddleware Extension

The existing exec path in `execMiddleware` is request-response: `executor(input) → result → sess.Write(result) → session exits`. Monitor needs the session to stay open for continuous event delivery.

**Solution:** The SSH Server gains a `StreamingExecutorFactory` field (parallel to `ExecutorFactory`). When `execMiddleware` receives a command, it first checks if the streaming executor factory is set and if the command matches the monitor prefix. If so, it calls the streaming executor with the SSH session writer. The streaming executor blocks until the monitor session ends.

| execMiddleware Step | Normal Path | Monitor Path |
|---------------------|-------------|--------------|
| Receive command | same | same |
| Check command prefix | not monitor → executor(input) | monitor prefix → streaming executor |
| Execute | `result, err := executor(input)` | `streamingExecutor(sess, input)` (blocks) |
| Write response | `sess.Write(result)` | N/A (handler writes directly to sess) |
| Session end | exits after single response | exits when handler returns (disconnect or shutdown) |

The streaming executor factory is wired in `loader.go` alongside the regular executor factory. It receives the `pluginserver.Server` (which has the MonitorManager) and the Dispatcher (for authorization checks).

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
| `id` | `string` | Unique client ID (generated on registration) |
| `subscriptions` | `[]*Subscription` | Event filters (one per event type — comma-separated types expand to multiple subscriptions) |
| `eventChan` | `chan string` | Buffered channel for formatted events (size: 256) |
| `ctx` | `context.Context` | Client-scoped context for cancellation |
| `cancel` | `context.CancelFunc` | Cancel function for cleanup |
| `dropped` | `atomic.Uint64` | Count of events dropped due to full channel |

All monitors receive `json+parsed` format (the existing ze-bgp JSON format). Display rendering (visual text, table, yaml) is client-side.

### Multiple Event Types → Multiple Subscriptions

When the user specifies `event update,state`, the arg parser splits on comma and creates one `Subscription` per event type. The MonitorClient holds all of them. `MonitorManager.GetMatching()` checks all subscriptions for each monitor, same logic as `SubscriptionManager.GetMatching()` — match any subscription, add monitor once.

If no `event` keyword is specified, one subscription per valid BGP event type is created (all 8 types from `events.go`).

### Monitor Output Protocol

The engine sends JSON events (one per line) over the SSH session. The **default display is text** -- the client renders each JSON event as a visual one-liner. Pipe operators override the display: `| json` pretty-prints the JSON, `| table` renders as a table, `| yaml` renders as YAML, `| match` filters on the rendered output.

| Step | Direction | Content |
|------|-----------|---------|
| 1. Command | CLI → Engine | SSH exec: `"bgp monitor [args]"` (text command string) |
| 2. Header | Engine → CLI | Text line: filter summary (e.g., "monitoring: all events, all peers") |
| 3. Events | Engine → CLI | One JSON object per line (wire format for pipe compatibility) |
| 4. End | Either side | SSH session closes (Ctrl-C or server shutdown) |

**Wire format vs display format:**

| Pipe | Display |
|------|---------|
| (none) | Visual text one-liner per event (default, human-friendly) |
| `\| json` | Pretty-printed JSON per event |
| `\| json compact` | Raw JSON passthrough (NDJSON) |
| `\| table` | Table rendering per event |
| `\| yaml` | YAML rendering per event |
| `\| match <pattern>` | Filter: only show events where rendered text matches pattern |

The JSON wire format reuses the existing ze-bgp JSON event format (same as plugin delivery). Each line is a complete JSON object parseable by jq.

### CLI Keyword Grammar

| Keyword | Values | Default | Example |
|---------|--------|---------|---------|
| `peer` | IP address, `*` | all peers | `peer 10.0.0.1` |
| `event` | comma-separated event types | all BGP events | `event update,state` |
| `direction` | `received`, `sent` | both | `direction received` |

All keywords are optional. Defaults: all peers, all events, both directions.

Standard pipe operators apply to the output: `| table`, `| json`, `| json compact`, `| yaml`, `| match <pattern>`, `| count`. The default rendering (no pipe) is a visual one-liner per event.

### Visual Text Output Format (Default Rendering)

**Design goal:** each event line is self-contained, scannable, and grep-friendly. This is the **client-side default rendering** applied to each JSON event when no pipe operator is specified.

**Line format:**

| Event Type | Format |
|------------|--------|
| UPDATE (received) | `HH:MM:SS recv UPDATE  peer-addr AS{asn}  +prefix [+prefix...] nhop=addr` |
| UPDATE (sent) | `HH:MM:SS sent UPDATE  peer-addr AS{asn}  +prefix [+prefix...] nhop=addr` |
| UPDATE (withdraw) | `HH:MM:SS recv UPDATE  peer-addr AS{asn}  -prefix [-prefix...]` |
| STATE (up) | `HH:MM:SS ---- STATE   peer-addr AS{asn}  established` |
| STATE (down) | `HH:MM:SS ---- STATE   peer-addr AS{asn}  down (reason)` |
| OPEN | `HH:MM:SS recv OPEN    peer-addr AS{asn}  hold=N id=addr` |
| NOTIFICATION | `HH:MM:SS recv NOTIF   peer-addr AS{asn}  code/subcode: description` |
| KEEPALIVE | `HH:MM:SS recv KALIVE  peer-addr AS{asn}` |
| REFRESH | `HH:MM:SS recv RFRSH   peer-addr AS{asn}  family` |
| EOR | `HH:MM:SS recv EOR     peer-addr AS{asn}  family` |
| NEGOTIATED | `HH:MM:SS ---- NEGOT   peer-addr AS{asn}  caps: mp,asn4,...` |

Direction column: `recv` for received, `sent` for sent, `----` for directionless events (state, negotiated).

Event type column: fixed 6-char width for alignment (`UPDATE`, `STATE `, `OPEN  `, `NOTIF `, `KALIVE`, `RFRSH `, `EOR   `, `NEGOT `).

Prefix notation: `+` for announce, `-` for withdraw. Multiple prefixes space-separated. If more than 5 prefixes, show count: `+10.0.0.0/24 +10.0.1.0/24 ... (+8 more)`.

**Dropped event warning** (piggybacked, rendered as text line):

`HH:MM:SS ---- WARN    --- dropped N events (slow reader)`

**Rendering location:** The visual text formatter lives in the monitor plugin package (`format.go`). It is called client-side (in `StreamCommand` for exec mode, in the BubbleTea model for interactive mode) to render each JSON event line into the visual format. When a pipe operator is used, the pipe operates on the raw JSON instead.

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

1. All monitors use `json+parsed` format. After building the `formatOutputs` map for plugin processes, check if `json+parsed` is already in the map (likely yes, since most plugins use JSON). If not, format it.
2. Call `s.Monitors().Deliver(namespace, eventType, direction, peer, jsonOutput)` — MonitorManager matches each monitor's subscriptions and enqueues the JSON string to each matching monitor's `eventChan`.

This is efficient: monitors share the `json+parsed` format entry with any plugins that also use JSON, so formatting happens at most once. No per-monitor format variations on the engine side.

### Streaming Handler and Disconnect Detection

The streaming handler (called from the streaming executor in execMiddleware) follows this sequence:

1. Parse monitor args → on error, write error line to writer, return immediately.
2. Create `MonitorClient` with buffered `eventChan` and client-scoped context.
3. Register with `MonitorManager.Add()`.
4. Write header line to writer (active filters, encoding).
5. Enter select loop: read from `eventChan` → write line to writer, or `ctx.Done()` → return.
6. Deferred: `MonitorManager.Remove(id)` cleans up on any exit path.

**Disconnect detection:** When the client disconnects (Ctrl-C closes the SSH session), the next `sess.Write()` returns an error. The select loop detects this write error and returns, triggering cleanup. No separate reader goroutine is needed — write errors are sufficient for SSH session disconnect detection.

For server shutdown: the SSH server context is cancelled, which triggers `ctx.Done()` in the select loop.

### Interactive BubbleTea Mode

When the user types `bgp monitor [args]` in the interactive TUI, the model enters **monitor mode**:

1. The model detects the `bgp monitor` prefix in the command input.
2. Instead of calling the regular executor (which returns a single string), it starts a background streaming goroutine that connects to the monitor handler.
3. Events arrive as `tea.Msg` values via `tea.Cmd` — each event line is a message that the model's `Update()` appends to the viewport.
4. The viewport auto-scrolls as new events arrive.
5. Pipe operators apply per-event-line: `bgp monitor | match 10.0.0.1` filters each event line against the pattern before displaying. `bgp monitor | match update` shows only UPDATE events.
6. Pressing Escape or Ctrl-C exits monitor mode and returns to the command prompt.
7. On exit, the streaming goroutine is cancelled, which triggers MonitorManager cleanup.

**Implementation:** The model uses the same streaming executor path. The BubbleTea model's `executeOperationalCommand` detects the monitor prefix, starts a goroutine that calls the streaming executor, and sends each event line as a `monitorEventMsg` through a channel. The `Update()` method receives these messages and appends to viewport content.

### Backpressure

Monitor `eventChan` buffer size: 256. When enqueuing, use non-blocking send. If the channel is full (slow client), drop the event and increment the `dropped` counter.

The dropped-event warning is piggybacked on the next successfully delivered event: before writing the event frame, check `dropped`. If non-zero, swap the counter to zero and prepend a warning line. This avoids the problem of trying to send a warning to an already-full channel.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SSH exec `bgp monitor` command | → | `handleMonitor()` streaming via SSH session | `test/plugin/monitor-basic.ci` |
| Event dispatch with active monitor | → | `MonitorManager.Deliver()` in events.go | `test/plugin/monitor-events.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bgp monitor` with no args, peer sends UPDATE | Client receives visual text UPDATE line on stdout (default rendering) |
| AC-2 | `bgp monitor event state`, peer goes up/down | Client receives state events only (no updates, keepalives, etc.) |
| AC-3 | `bgp monitor peer 10.0.0.1`, events from two peers | Client receives events only from 10.0.0.1 |
| AC-4 | `bgp monitor event update,state` | Client receives both update and state events, nothing else |
| AC-5 | Client disconnects (Ctrl-C) during monitoring | Server cleans up monitor entry, no goroutine leak |
| AC-6 | `bgp monitor \| json` | Events displayed as pretty-printed JSON instead of visual text |
| AC-7 | `bgp monitor direction received` | Only received events (not sent) |
| AC-8 | Invalid keyword/value in monitor args | Error response with clear message, SSH session exits immediately |
| AC-9 | `bgp monitor \| match 10.0.0.0/24` | Pipe operators filter per-event-line (client-side) |
| AC-10 | `bgp monitor` in interactive BubbleTea CLI | Events stream into viewport, auto-scroll, Escape exits monitor mode |
| AC-11 | `bgp monitor \| table` | Events rendered as table rows |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseMonitorArgs` | `internal/component/bgp/plugins/cmd/monitor/monitor_test.go` | Keyword parsing: peer, event, direction | |
| `TestParseMonitorArgsMultipleEvents` | same | Comma-separated event types expand correctly | |
| `TestParseMonitorArgsInvalid` | same | Invalid keywords/values return errors | |
| `TestParseMonitorArgsDefaults` | same | No args → all events, all peers, both directions | |
| `TestMonitorManagerAddRemove` | `internal/component/plugin/server/monitor_test.go` | Add/remove monitor clients | |
| `TestMonitorManagerGetMatching` | same | Subscription matching against events | |
| `TestMonitorManagerCleanup` | same | Client disconnect cleans up state | |
| `TestMonitorDelivery` | same | Events delivered to matching monitors, not non-matching | |
| `TestMonitorBackpressure` | same | Full channel drops events, counter increments | |
| `TestFormatMonitorLine` | `internal/component/bgp/plugins/cmd/monitor/format_test.go` | Text line formatting: timestamp, direction, event type, peer, summary | |
| `TestFormatMonitorLineWithdrawn` | same | Withdraw prefixes show `-` prefix notation | |
| `TestFormatMonitorLineDropWarning` | same | Dropped event warning line format | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| eventChan buffer | 256 | N/A (internal) | N/A | N/A |

No user-facing numeric inputs in this feature.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-monitor-basic` | `test/plugin/monitor-basic.ci` | Start monitor via SSH exec, peer sends UPDATE, verify event line received on stdout | |
| `test-monitor-event-filter` | `test/plugin/monitor-events.ci` | Monitor with event filter, verify only matching events appear in output | |
| `test-monitor-peer-filter` | `test/plugin/monitor-peer.ci` | Monitor with peer filter, verify only matching peer events in output | |

### Future (if deferring any tests)
- Property testing (fuzz with random event streams) — deferrable, not user-facing
- Benchmarks for monitor delivery overhead — deferrable, performance optimization
- Interactive BubbleTea mode test — deferrable (requires TUI test harness)

## Files to Modify
- `internal/component/plugin/server/server.go` — add MonitorManager field to Server struct, `Monitors()` accessor
- `internal/component/bgp/server/events.go` — add monitor delivery after plugin delivery in all 6 event functions
- `internal/component/ssh/ssh.go` — add StreamingExecutorFactory field, monitor detection in execMiddleware
- `internal/component/bgp/config/loader.go` — wire streaming executor factory alongside regular executor factory
- `cmd/ze/cli/main.go` — detect `--run "bgp monitor"`, call `StreamCommand` instead of `ExecCommand`
- `cmd/ze/internal/sshclient/sshclient.go` — add `StreamCommand` for line-by-line streaming SSH sessions
- `internal/component/cli/model_mode.go` — add monitor mode for BubbleTea interactive streaming

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/bgp/plugins/cmd/monitor/schema/ze-bgp-cmd-monitor-api.yang` + `ze-monitor-cmd.yang` |
| RPC count in architecture docs | [x] | `docs/architecture/api/architecture.md` |
| CLI commands/flags | [x] | `cmd/ze/cli/main.go` (detect monitor prefix for streaming) |
| CLI usage/help text | [x] | Help text in RPC registration |
| API commands doc | [x] | `docs/architecture/api/commands.md` |
| Plugin SDK docs | [ ] | N/A — not an SDK feature |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/monitor-*.ci` |

## Files to Create
- `internal/component/bgp/plugins/cmd/monitor/monitor.go` — monitor command handler + arg parsing + streaming loop
- `internal/component/bgp/plugins/cmd/monitor/monitor_test.go` — unit tests for arg parsing
- `internal/component/bgp/plugins/cmd/monitor/format.go` — visual text line formatting (timestamp, direction, type, peer, summary)
- `internal/component/bgp/plugins/cmd/monitor/format_test.go` — unit tests for visual line formatting
- `internal/component/bgp/plugins/cmd/monitor/doc.go` — package doc + plugin registration + YANG import
- `internal/component/bgp/plugins/cmd/monitor/schema/embed.go` — embedded YANG content
- `internal/component/bgp/plugins/cmd/monitor/schema/register.go` — YANG module registration
- `internal/component/bgp/plugins/cmd/monitor/schema/ze-bgp-cmd-monitor-api.yang` — monitor RPC definitions
- `internal/component/bgp/plugins/cmd/monitor/schema/ze-monitor-cmd.yang` — monitor CLI tree
- `internal/component/plugin/server/monitor.go` — MonitorManager type
- `internal/component/plugin/server/monitor_test.go` — MonitorManager unit tests
- `test/plugin/monitor-basic.ci` — functional test: basic monitoring
- `test/plugin/monitor-events.ci` — functional test: event filtering
- `test/plugin/monitor-peer.ci` — functional test: peer filtering

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for arg parsing** → Review: edge cases? All keyword combinations?
2. **Run tests** → Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement `parseMonitorArgs()`** → Minimal code to pass. Reuse `ParseSubscription` logic for validation.
4. **Run tests** → Verify PASS (paste output). All pass? Any flaky?
5. **Write unit tests for MonitorManager** → Add/remove, matching, cleanup, delivery, backpressure.
6. **Run tests** → Verify FAIL.
7. **Implement MonitorManager** → Channel-based delivery, non-blocking send, dropped counter.
8. **Run tests** → Verify PASS.
9. **Write unit tests for visual text formatting** → Line format for each event type.
10. **Run tests** → Verify FAIL.
11. **Implement visual text formatter** → Timestamp + direction + type + peer + summary.
12. **Run tests** → Verify PASS.
13. **Wire MonitorManager into Server** — Add field, accessor, initialize in `NewServer()`.
14. **Wire monitor delivery into events.go** — All 6 event functions: get format keys, format, deliver.
15. **Wire monitor handler** — RPC registration, YANG schema, streaming loop.
16. **Wire SSH streaming** — `StreamingExecutorFactory` in ssh.go, detection in execMiddleware.
17. **Wire executor factory** — `loader.go`: set streaming executor factory alongside regular factory.
18. **Wire CLI streaming** — `StreamCommand` in sshclient, detect in `cli/main.go`.
19. **Wire BubbleTea interactive mode** — Monitor mode in model, event messages, viewport streaming.
20. **Functional tests** → Create `.ci` tests.
21. **Verify all** → `make ze-verify`
22. **Critical Review** → All 6 checks from `rules/quality.md` must pass.
23. **Complete spec** → Fill audit tables, write learned summary.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3, 7, 11, or 15 (fix syntax/types) |
| Test fails wrong reason | Step 1, 5, or 9 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| CLI uses Unix socket + JSON-RPC | CLI uses SSH exec → CommandExecutor → Dispatcher.Dispatch | Reading actual source files | Major — entire transport layer redesigned |
| client.go with clientLoop exists | No client.go; SSH execMiddleware handles CLI dispatch | File not found | Major — streaming intercept point is execMiddleware, not clientLoop |
| Request.More/RPCResult.Continues for CLI streaming | These fields are for plugin socket protocol, not SSH CLI path | Reading rpc/message.go + ssh.go | Medium — streaming uses plain text lines, not JSON-RPC envelopes |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The SSH transport simplifies the monitor protocol: no JSON-RPC envelope, no NUL framing, just lines on stdout. Each event is one line. This is better for piping to grep/jq and for the visual text format.
- The `StreamingExecutorFactory` pattern cleanly separates streaming concerns from the regular request-response path without modifying the Dispatcher interface.
- The visual text format is designed for grep-friendliness: fixed-width columns make it easy to `| match UPDATE` or `| match 10.0.0.1`.

## RFC Documentation

N/A — operational feature, no RFC requirements.

## Implementation Summary

### What Was Implemented
- MonitorManager + MonitorClient in `plugin/server/monitor.go` with subscription matching, backpressure (non-blocking send + dropped counter), and thread-safe access
- StreamingHandler registry in `plugin/server/handler.go` so loader doesn't import plugin implementations
- Monitor command handler + arg parser in `bgp/plugins/cmd/monitor/monitor.go` with keyword validation, StreamMonitor streaming loop, and authorization check
- Visual text formatter in `bgp/plugins/cmd/monitor/format.go` matching production ze-bgp JSON structure
- YANG schemas (`ze-bgp-cmd-monitor-api.yang` + `ze-monitor-cmd.yang`) and registration
- Monitor delivery in all 6 event functions in `bgp/server/events.go` with format cache reuse
- SSH streaming: `StreamingExecutorFactory` on SSH Server, monitor detection in `execMiddleware`, explicit error for nil factory
- CLI streaming: `StreamCommand` in sshclient, `StreamMonitor` in cli/main.go with pipe operator support
- Exported `Dispatcher.IsAuthorized()` for cross-package authorization checks

### Bugs Found/Fixed
- FormatMonitorLine parsed flat JSON but production uses nested `bgp.{state,update.nlri,...}` structure -- rewrote struct + tests
- extractMonitorArgs sliced raw input without lowercasing -- fixed to lowercase before slicing
- Streaming path bypassed authorization -- added username passthrough + IsAuthorized check
- Missing bidirectional cross-references in doc.go/format.go/monitor.go/server.go -- added
- Unused format parameter in CLI StreamMonitor -- removed
- onPeerNegotiated lacked hasMonitors early-return optimization -- added
- sshclient StreamCommand discarded session.Wait() error -- now returned
- Agent-introduced dumpGoroutines/quit code from parallel session -- reverted

### Documentation Updates
- Learned summary: `docs/learned/396-bgp-monitor.md`
- Spec fully revised for SSH-based transport (was Unix socket)
- `docs/architecture/api/architecture.md` -- Monitor Streaming section, RPC count 24->25
- `docs/architecture/api/commands.md` -- Monitor command syntax, keyword table, dispatch tree

### Deviations from Plan
- BubbleTea interactive mode in `model_monitor.go` (not `model_mode.go` as planned -- separate file for single concern)
- Added `handler.go` StreamingHandler registry (not in original plan, needed for import rule compliance)
- Added `command.go` exported `IsAuthorized()` (not in plan, needed for authz on streaming path)
- `encoding`/`format` keywords removed from grammar (spec revision: engine always sends JSON, display is client-side)
- Default encoding changed from `json` to `text` visual rendering via pipe operators

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `bgp monitor` streams live BGP events | ✅ Done | `monitor.go:StreamMonitor` | Blocks on select loop, writes to SSH session |
| Keyword-based filtering (event, peer, direction) | ✅ Done | `monitor.go:parseMonitorArgs` | 25+ unit tests cover all combinations |
| Connection stays open until Ctrl-C | ✅ Done | `ssh.go:execMiddleware` + `monitor.go:StreamMonitor` | Context cancellation on SSH disconnect |
| Pipe operators work on output | ✅ Done | `cli/main.go:StreamMonitor` | `ProcessPipesDefaultTable` applies per-line |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestStreamMonitor` (integration) | Delivers UPDATE event to monitor, verifies in output |
| AC-2 | ✅ Done | `TestStreamMonitorWithFilters` + `test/plugin/monitor-events.ci` | Unit + functional test |
| AC-3 | ✅ Done | `TestStreamMonitorWithFilters` + `test/plugin/monitor-peer.ci` | Unit + functional test |
| AC-4 | ✅ Done | `TestParseMonitorArgsMultipleEvents` + `TestBuildSubscriptions` | Comma-separated events expand correctly |
| AC-5 | ✅ Done | `TestStreamMonitor` | Verifies mm.Count()==0 after cancel |
| AC-6 | ✅ Done | CLI uses `ProcessPipesDefaultTable` | `\| json` applies to each event line |
| AC-7 | ✅ Done | `TestParseMonitorArgs/direction_received` + subscription matching | Direction filter validated and applied |
| AC-8 | ✅ Done | `TestStreamMonitorInvalidArgs` + `TestParseMonitorArgsInvalid` | Error returned, no registration on bad input |
| AC-9 | ✅ Done | `cli/main.go:StreamMonitor` | `ProcessPipesDefaultTable` handles match/count/etc |
| AC-10 | ✅ Done | `cli/model_monitor.go` | MonitorSession + MonitorFactory + polling + Escape to stop |
| AC-11 | ✅ Done | `cli/main.go:StreamMonitor` | `ProcessPipesDefaultTable` defaults to table format |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestParseMonitorArgs | ✅ Done | monitor_test.go | 6 subtests |
| TestParseMonitorArgsMultipleEvents | ✅ Done | monitor_test.go | 3 subtests |
| TestParseMonitorArgsInvalid | ✅ Done | monitor_test.go | 10 subtests |
| TestParseMonitorArgsDefaults | ✅ Done | monitor_test.go | |
| TestMonitorManagerAddRemove | ✅ Done | server/monitor_test.go | |
| TestMonitorManagerGetMatching | ✅ Done | server/monitor_test.go | |
| TestMonitorManagerCleanup | ✅ Done | server/monitor_test.go | |
| TestMonitorDelivery | ✅ Done | server/monitor_test.go | |
| TestMonitorBackpressure | ✅ Done | server/monitor_test.go | |
| TestFormatMonitorLine | 🔄 Changed | format_test.go | Split into 5 test functions covering all 8 event types + invalid JSON + truncation |
| TestHandleMonitor | ✅ Done | monitor_test.go | Added during review fix |
| TestBuildSubscriptions | ✅ Done | monitor_test.go | Added during review fix |
| TestFormatHeader | ✅ Done | monitor_test.go | Added during review fix |
| TestStreamMonitor | ✅ Done | monitor_test.go | Integration test with syncBuffer |
| TestStreamMonitorWithFilters | ✅ Done | monitor_test.go | Peer + event filter verification |
| TestStreamMonitorInvalidArgs | ✅ Done | monitor_test.go | Error path, no registration |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `bgp/plugins/cmd/monitor/monitor.go` | ✅ Done | Handler + arg parsing + StreamMonitor |
| `bgp/plugins/cmd/monitor/monitor_test.go` | ✅ Done | 17 top-level test functions |
| `bgp/plugins/cmd/monitor/format.go` | ✅ Done | Visual text formatter + production JSON struct |
| `bgp/plugins/cmd/monitor/format_test.go` | ✅ Done | All 8 event types + edge cases |
| `bgp/plugins/cmd/monitor/doc.go` | ✅ Done | Package doc + YANG import |
| `bgp/plugins/cmd/monitor/schema/embed.go` | ✅ Done | Embedded YANG |
| `bgp/plugins/cmd/monitor/schema/register.go` | ✅ Done | YANG registration |
| `bgp/plugins/cmd/monitor/schema/ze-bgp-cmd-monitor-api.yang` | ✅ Done | Monitor RPC |
| `bgp/plugins/cmd/monitor/schema/ze-monitor-cmd.yang` | ✅ Done | CLI tree |
| `plugin/server/monitor.go` | ✅ Done | MonitorManager + MonitorClient |
| `plugin/server/monitor_test.go` | ✅ Done | 6 test functions |
| `test/plugin/monitor-basic.ci` | ✅ Done | Tests RPC handler via dispatch-command |
| `test/plugin/monitor-events.ci` | ✅ Done | Tests event filter via dispatch-command |
| `test/plugin/monitor-peer.ci` | ✅ Done | Tests peer filter via dispatch-command |

### Audit Summary
- **Total items:** 43
- **Done:** 42
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (FormatMonitorLine test names restructured)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
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
