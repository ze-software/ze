# Production Diagnostics

Ze answers operational questions from inside the daemon: runtime state, event
history, metrics, packet capture, active probes, component health and log
query. A gokrazy appliance carries none of the usual tools, so every diagnostic
is part of the binary.

<!-- source: internal/plugins/diag/cmd/register.go -- diag RPC registration -->
<!-- source: internal/core/health/registry.go -- component health aggregation -->
<!-- source: internal/core/slogutil/ring.go -- structured log ring -->

## CLI first, MCP for free

Every diagnostic is a YANG RPC registered as a CLI command. The MCP tool list
is generated from the registered RPCs, so a new diagnostic appears to an AI
agent with no MCP code change. This held across every diagnostic area and
is the reason none of them needed a bespoke transport.

| Area | What it exposes |
|------|-----------------|
| Runtime state | L2TP observer and CQM, interface counters, component and plugin status, traffic |
| Event history | global event ring, BGP peer FSM history, L2TP tunnel and session FSM history |
| Metrics query | Prometheus text filtered by name and labels, L2TP and BGP health summaries |
| Packet capture | decoded rings and raw byte capture (`packet-capture.md`) |
| Active probes | ping, traceroute, route lookup (`active-probes.md`) |
| Health | component health registry, HTTP `/health` returning 200 or 503 |
| Log query | slog ring handler split into `general` (512 `debug`/`info`) and `faults` (256 `warn`/`error`) buffers filtered by level, component and count |
| Live triage | single-call bounded daemon snapshot (`show system triage`) across version, health, doctor, reports, crashes, plugins, reload, recent logs and runtime (including watchdog and hung-handler counts) |
| Execution trace | always-on Go flight recorder, on-demand snapshot capture, and snapshot retrieval (`show system trace`, `request system trace snapshot`) |
| Plugin runtime | cross-process `\| plugin <name>` inspection on goroutines, profiles (`\| summary`), Go memory, OS `/proc/<pid>` memory, and plugin log rings with non-destructive host fallback |
| Operation correlation | process-scoped `op-<N>` IDs (`internal/core/opid`) propagated across commands, reloads, logs, events, report-bus issues, and audit records, queried with `\| operation <id>` |
| Reload history | 32-entry leaf-locked reload transaction journal (`show system reload history`) with per-plugin verify/apply/rollback timings and errors |
| Liveness & wedge survival | bounded 60s operator handler timeouts (`doctor-command-handler-hung`) and always-on daemon liveness watchdog (`internal/core/watchdog`, `show system watchdog`, `doctor-daemon-stall-detected`) |

### Single-call live triage (`show system triage`)

`show system triage` aggregates ten bounded sections in one read-only YANG RPC
(`ze-cmd:show-system-triage`): `version`, `health`, `doctor`, `errors`,
`warnings`, `crashes`, `plugins`, `reload`, `recent-logs`, and `runtime`.
Because it registers under the shared `show system` YANG tree, CLI, SSH, web,
REST, gRPC, and MCP all expose it automatically with no MCP-specific code.

List sections cap their payloads newest-first (`32` errors, `64` warnings, `16`
crashes, `32` warn/error log entries) and report `total-count` and `truncated`
so operators and agents know when to drill into full listings. `plugins`
reports summary counts and expands only unhealthy or fault-bearing plugins.
`runtime` also reports `watchdog-running`, `stall-count`, and
`hung-handler-count` under leaf locks without acquiring `Server.txLock`.
When an optional runtime source (`doctor`, `plugins`, or `reload`) cannot
answer, that section returns `"available": false` with its `reason` or `error`
while every other section still succeeds. `version` reads structured build and
appliance provenance from `version.BuildIdentity`.

<!-- source: internal/component/cmd/show/triage.go -- handleShowSystemTriage, triageRuntime -->
<!-- source: internal/core/version/version.go -- BuildIdentity -->

### Execution flight recorder (`show system trace`)

Daemon startup starts the process-global Go execution flight recorder in
`internal/core/flightrec` (`10s` / `4 MiB` rolling window) and stops it on
shutdown. If `flightrec.Start` returns an error at startup, the daemon logs a
warning and continues to start. Recovered `taskguard` panics, slow plugin
deliveries, hung operator command handlers (`"hung-command"`), and daemon
liveness watchdog stalls (`"watchdog-stall"`) call `flightrec.CaptureAuto` with
a 30-second cooldown to freeze the preceding execution window into a 4-slot
in-memory snapshot ring (4 MiB cap per snapshot).

Operators and AI agents capture a snapshot on demand with
`request system trace snapshot` (`ze-cmd:request-system-trace-snapshot`), list
recorder status and retained snapshot metadata without raw trace bytes with
`show system trace` (`ze-cmd:show-system-trace`), and fetch one snapshot's
`go-trace-base64` payload with `show system trace id <id>`. Refusals for a
stopped recorder or an unknown identifier return an operator error naming
`ze show system trace`.

<!-- source: internal/component/cmd/show/trace.go -- handleShowSystemTrace, handleRequestSystemTraceSnapshot -->
<!-- source: internal/core/flightrec/flightrec.go -- Start, Stop, CaptureAuto, CaptureNow -->

### Operation correlation (`internal/core/opid` and `| operation <id>`)

`internal/core/opid` is a stdlib-only leaf package that mints monotonic,
process-scoped correlation IDs (`op-<N>`) and attaches them to
`context.Context` (`New`, `With`, `Ensure`, `From`). `Dispatcher.Dispatch`
ensures an `operation-id` on `CommandContext.RequestContext` at dispatch entry,
and `doReloadContext` ensures an `operation-id` for every config reload trigger
(SIGHUP, queued SIGHUP, CLI commit, `request reload`, and confirmed-commit
revert).

The four diagnostic stores carry `operation-id` automatically without breaking
uncorrelated callers:

- `slogutil.LogRing` captures `opid.From(ctx)` (or an explicit `"operation-id"`
  attribute) on `LogEntry.OpID` and in encoded attributes while preserving
  redaction and `general`/`faults` sub-ring separation, and filters newest-first
  via `SnapshotWithOp`.
- `pluginserver.EventRing` captures `operation-id` via `appendWithOp` and
  filters via `SnapshotWithOp` (`docs/architecture/diagnostics/event-history-rings.md`).
- `report.Issue` carries optional `operation-id` (`omitempty`), populated by
  `RaiseErrorContext` / `RaiseWarningContext` or promoted from
  `detail["operation-id"]`.
- `audit.Entry` persists optional `operation-id` (`omitempty`) in JSON-lines
  audit logs and filters via `audit.Filter.OpID`.

`show log recent` (registered alongside `| plugin <name>` in one
`command.RegisterPipeFilters` call so both compose), `show event recent`,
`show errors`, `show audit`, and `show system reload history` register the
command-owned `| operation <id>` pipe filter. Because pipe filters propagate
from the command registry, MCP exposes it automatically as
`pipe: [{"name": "operation", "args": ["<id>"]}]` with zero MCP-specific code.

<!-- source: internal/core/opid/opid.go -- New, With, Ensure, From -->
<!-- source: internal/core/slogutil/ring.go -- LogEntry, ringHandler.Handle, SnapshotWithOp -->
<!-- source: internal/core/report/report.go -- Issue, RaiseErrorContext, RaiseWarningContext -->
<!-- source: internal/core/audit/audit.go -- Entry -->
<!-- source: internal/core/audit/query.go -- Filter, Log.Query -->
<!-- source: internal/component/cmd/show/op_pipe.go -- operation pipe filter registration and parsing -->
<!-- source: internal/plugins/log/cmd/register.go -- RegisterPipeFilters for show log recent -->

### Reload transaction history (`show system reload history`)

`reloadGeneration` retains a 32-entry circular history ring protected by its
existing leaf mutex `reloadGen.mu` (never `Server.txLock`). During
`doReloadContext`, a context-attached `ReloadTrace` collects sorted
`changed-roots` (capped at 32), `tx-state` (`committed`, `aborted`,
`rolled-back`, or `no-op`), and per-plugin `verify-ms`, `apply-ms`,
`rollback-ms`, `status`, and bounded `error` steps from
`transaction.TxResult.PluginSteps`. When `runReloadContext` finishes (for every
processed reload, excluding `ErrReloadInProgress`),
`Server.MarkReloadProcessedWithRecord` advances `generation`, updates the fence,
and appends the `ReloadRecord` (`generation`, `operation-id`, `trigger`,
`outcome`, `started-at`, `finished-at`, `duration-ms`, `changed-roots`,
`tx-state`, `plugins`, `error`).

Read-only `show system reload history [count <N>]` (supporting
`| operation <id>`) reads the ring newest-first under `reloadGen.mu`, while
`show system reload status` keeps the reload fence semantics unchanged and adds
optional `last-operation-id`.

<!-- source: internal/component/plugin/server/reload_generation.go -- MarkReloadProcessedWithRecord, ReloadStatusWithOp, ReloadHistory -->
<!-- source: internal/component/plugin/server/reload_history.go -- ReloadRecord, ReloadPluginStep, ReloadTrace, WithReloadTrace -->
<!-- source: internal/component/config/transaction/orchestrator.go -- PluginStepResult, TxResult.PluginSteps -->
<!-- source: internal/component/cmd/show/reload_history.go -- handleShowSystemReloadHistory -->
<!-- source: internal/component/cmd/show/reload_status.go -- handleShowReloadStatus -->

### Bounded handler timeouts and daemon liveness watchdog (`show system watchdog`)

Core diagnostic commands (`show system triage`, `show system goroutines`,
`show system profile`, `show system trace`, `show runtime memory`,
`show system memory`, `show log recent`, `show event recent`, `show errors`,
`show warnings`, `show system reload status`, `show system reload history`, and
`show system watchdog`) read only leaf mutexes or atomics and never acquire
`Server.txLock` or config-write locks, keeping diagnostics responsive even if a
config transaction wedges.

`Dispatcher` enforces a bounded execution deadline (`60s` default, configurable
via `setHandlerTimeout` for tests) on operator/external builtin dispatches
(`ctx == nil || ctx.Process == nil`). Command owners whose work legitimately
takes longer declare a generic `HandlerTimeout func(command.ValidatedArgs) time.Duration`
hook on `RPCRegistration` / `RegisterOptions` — `show system profile` declares
`duration + 15s` — keeping command-specific logic out of generic dispatch.
Plugin-originated internal RPC dispatches (`ctx.Process != nil`, such as RS/RR
source-DOWN departures) stay governed solely by their plugin lifecycle context
without an unrelated `60s` expiry. When a timed handler ignores cancellation
past its deadline and a `100ms` grace window, `Dispatcher` returns a structured
operator error naming `operation-id` and `ze show system goroutines mode blocked`,
records a `HungHandlerRecord` in `Server`'s 16-slot leaf-locked ring, raises
`doctor-command-handler-hung` on the report bus, and invokes
`flightrec.CaptureAuto("hung-command", cmdName)`. Cooperative caller
cancellation returns cleanly without raising `doctor-command-handler-hung`.

`internal/core/watchdog` runs an always-on liveness goroutine started beside
`flightrec.Start()` in `cmd/ze/hub/main.go` and joined on `Stop()`. Every `2s`,
it checks scheduler wake-up drift (`5s` stall threshold) and probes registered
via `watchdog.RegisterProbe` (preserving the core-tier boundary so
`plugin/server` registers `"config-tx-lock"` flagging `txLock` held past `45s`
and `"command-handlers"` flagging active hung handlers). On stall detection
(rate-limited by a `30s` cooldown), the watchdog captures bounded goroutine
stacks (`256 KiB` cap) and calls `flightrec.CaptureAuto("watchdog-stall", probe)`
into an 8-slot stall ring, and raises `doctor-daemon-stall-detected`.
`show system watchdog` (`ze-cmd:show-system-watchdog`) exposes live watchdog
status, probe results, `hung-handlers`, and `recent-stalls`.

<!-- source: internal/component/plugin/server/command.go -- Dispatcher.dispatchLine, HandlerTimeout -->
<!-- source: internal/component/plugin/server/command_timeout.go -- HungHandlerRecord, HungHandlers, watchdog probes -->
<!-- source: internal/component/cmd/show/profile.go -- profileHandlerTimeout -->
<!-- source: internal/core/watchdog/watchdog.go -- Start, Stop, RegisterProbe, Status, StallRecord -->
<!-- source: internal/core/watchdog/doctor_codes.go -- CodeDoctorDaemonStallDetected -->
<!-- source: internal/component/cmd/show/watchdog.go -- handleShowSystemWatchdog -->
<!-- source: cmd/ze/hub/main.go -- watchdog.Start, watchdog.Stop -->

### External plugin runtime introspection (`| plugin <name>`)

`show system goroutines`, `show system profile` (composable with `| summary`),
`show runtime memory`, `show system memory`, and `show log recent` register a
command-owned `| plugin <name>` pipe filter via `command.RegisterPipeFilters`.
Because pipe filters propagate automatically from the command registry, CLI,
SSH, web, REST, gRPC, and MCP all expose plugin targeting
(`pipe: [{"name": "plugin", "args": ["<name>"]}]`, or combined with
`{"name": "summary"}` on `show system profile`) with zero MCP-specific code.
Queries without `| plugin` keep their existing daemon-local behavior unchanged.

Go-runtime queries route identically over `DirectBridge` (in-daemon plugins)
and TLS `MuxConn` (forked external plugins) through `Server.InspectPlugin` and
the SDK's auto-started independent `ze-plugin-callback:runtime-inspect` worker,
so inspection still answers when a plugin's ordinary `eventLoop` is deadlocked.
Every response tags `plugin`, `process-mode` (`internal` or `external`), and
`pid`. When a forked external plugin cannot answer (wedged runtime, timeout,
disconnected transport, or non-Go binary), `Server.InspectPlugin` never sends
`SIGQUIT` or any signal; it returns a structured `fallback: true` snapshot with
`fallback-reason`, `rpc-error`, host Linux `/proc/<pid>` metrics (`state`,
`wchan`, `threads`, `vm-rss-kb`, `fd-count`), and the plugin's last 32
`stderr-tail` lines.

<!-- source: internal/component/plugin/server/runtime_inspect.go -- Server.InspectPlugin, Server.InspectPluginHost -->
<!-- source: internal/component/cmd/show/plugin_inspect.go -- plugin pipe filters and fallback builders -->
<!-- source: internal/plugins/log/cmd/handlers.go -- handleLogRecent -->

### Structured log fields

`show log recent` keeps each record's `timestamp`, `level`, `component` and
`message`, with its structured fields in `attributes` (an object, or null when
absent). JSON and the normal CLI rendering expose the same fields in the same
entry: an error or protocol rejection reason is not inferred from another log message.

The ring captures logger-bound attributes, nested groups and resolved `LogValuer`
values. JSON scalar fields stay typed. Opaque `Any` values retain the standard slog
text handler's presentation as strings, including `TextMarshaler` and `Stringer`
redaction; querying JSON must not reveal fields the configured text log hid.
Resolution and text formatting run before shared capture locks. A standard slog
JSON handler then captures an immutable snapshot, without retaining caller-owned
maps, slices or attribute arrays. Attribute decoding happens only when queried.
`slogutil.LogRing` splits storage into two circular buffers with a shared
sequence counter: `general` (512 entries for `debug` and `info`) and `faults`
(256 entries for `warn` and `error`). A burst of `info` or `debug` logs rotates
inside `general` and never evicts earlier `warn` or `error` entries from
`faults`. Unfiltered queries merge both buffers newest-first by sequence number,
and level-filtered queries read only the matching buffer.
<!-- source: internal/core/slogutil/ring.go -- LogRing, GlobalLogRing -->

The ring is inside the existing level and debug-filter handler chain. It does not
bypass filtering, expand a value that its `LogValuer` redacts, or change command
authorization. Log producers remain responsible for excluding secrets, just as
they are for the configured text/syslog backend. Crash reports keep their existing
message-only recent-log header rather than adding structured fields implicitly.

<!-- source: internal/plugins/log/cmd/handlers.go -- log query attributes -->

Browser command recall reports failed submission recording and failed JSON
delivery under separate doctor codes. Web network cleanup reports its own
failure before history cleanup starts. These bounded report-bus events also
appear in the daemon's `show doctor` recent-events check. The recall events
exclude commands, usernames and raw errors. The network event directs the
operator to `ze show log recent level err component web.server`.
<!-- source: internal/component/web/register_cli_history_diagnostic.go -- reportBrowserHistoryUnavailable, reportBrowserHistoryResponseFailure -->
<!-- source: cmd/ze/hub/register_web_history_diagnostic.go -- reportWebNetworkCleanupFailure -->
<!-- source: internal/component/doctor/checks_recent_events.go -- checkRecentEvents -->

## Ring buffers with atomic opt-in

A ring that is off costs nothing and takes no lock when on:

```
atomic.Pointer[Ring] field
CompareAndSwap to enable, Store(nil) to disable, Load() at every read site
```

The activation happens on an RPC handler goroutine while a reactor goroutine
reads the pointer on the hot path, so the pointer, not a mutex, is what makes
it race-free.

<!-- source: internal/component/bgp/reactor/raw_capture.go -- atomic ring activation -->

Decoded rings store numeric value types only, so appending allocates nothing.
String formatting is deferred to snapshot time. The L2TP observer's
pre-allocated fixed-size ring, whose snapshot returns copies, was reused for
every new ring with no change.

## VPP trace goes through the CLI socket

VPP exposes no typed govpp API for the data-plane trace. The trace command
dials the VPP CLI socket with `net.DialTimeout("unix", path, timeout)`, writes
the command, reads with a `bufio.Scanner` at a 4 MB buffer, and closes per
invocation. An environment variable overrides the socket path for tests.

<!-- source: internal/component/vpp/trace_linux.go -- CLI socket trace -->
<!-- source: internal/component/vpp/trace_other.go -- non-Linux stub -->

The response is unstructured text, which is the cost of this route. The command
input is validated with a regular expression, because the text goes to a shell
grammar the daemon does not own.

## Health registry

Components register a health check and the registry aggregates them. The HTTP
handler answers 200 when every check passes and 503 otherwise, so an external
supervisor needs no ze-specific client. `doctor-and-health-checks.md` covers
the check set and the offline `ze doctor` side.
