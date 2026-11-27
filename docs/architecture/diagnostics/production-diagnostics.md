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
| Live triage | single-call bounded daemon snapshot (`show system triage`) across version, health, doctor, reports, crashes, plugins, reload, recent logs and runtime (including watchdog, hung-handler, and bad-packet counts) |
| Execution trace | always-on Go flight recorder, on-demand snapshot capture, and snapshot retrieval (`show system trace`, `request system trace snapshot`) |
| Plugin runtime | cross-process `\| plugin <name>` inspection on goroutines, profiles (`\| summary`), Go memory, OS `/proc/<pid>` memory, and plugin log rings with non-destructive host fallback |
| Operation correlation | process-scoped `op-<N>` IDs (`internal/core/opid`) propagated across commands, reloads, logs, events, report-bus issues, and audit records, queried with `\| operation <id>` |
| Reload history | 32-entry leaf-locked reload transaction journal (`show system reload history`) with per-plugin verify/apply/rollback timings and errors |
| Liveness & wedge survival | bounded 60s operator handler timeouts (`doctor-command-handler-hung`) and always-on daemon liveness watchdog (`internal/core/watchdog`, `show system watchdog`, `doctor-daemon-stall-detected`) |
| Dataplane drift | read-only intent-vs-actual checks (`internal/core/drift`, `ze-plugin-callback:drift-check`) across internal and forked plugins (`firewall`, `fib-kernel`, `interface`), exposed via `show system drift [\| subsystem <name>]` and `show doctor` (`doctor-dataplane-drift`) |
| Decision trace | read-only offline firewall packet evaluation over `firewall.LastApplied()` (`show firewall trace`), alongside existing BGP best-path (`show bgp rib best \| reason`) and policy dry-run (`show policy test`) |
| Bad-packet ring | always-on 16-slot x 4 KiB circular ring (`internal/core/badpacket`) of malformed wire and panic-unwind frames queried via `show system bad-packets [count <N>] [\| protocol <name>]` |
| Support bundle modules | registrable `internal/core/supportreg` leaf combining 20 static modules with 7 core and 3 daemon-registered bounded live runtime modules in `request support archive` |
| MCP state resources | read-only `ze://triage` live snapshot over MCP `resources/list` and `resources/read` with command-scope authorization and a 5s live-state cache TTL |

### Single-call live triage (`show system triage`)

`show system triage` aggregates ten bounded sections in one read-only YANG RPC
(`ze-cmd:show-system-triage`): `version`, `health`, `doctor`, `errors`,
`warnings`, `crashes`, `plugins`, `reload`, `recent-logs`, and `runtime`.
Because it registers under the shared `show system` YANG tree, CLI, SSH, web,
REST, gRPC, and MCP all expose it automatically with no MCP-specific code.
MCP also publishes the same read-only snapshot as `ze://triage` (see below).

List sections cap their payloads newest-first (`32` errors, `64` warnings, `16`
crashes, `32` warn/error log entries) and report `total-count` and `truncated`
so operators and agents know when to drill into full listings. `plugins`
reports summary counts and expands only unhealthy or fault-bearing plugins.
`runtime` also reports `watchdog-running`, `stall-count`,
`hung-handler-count`, and `bad-packet-count` (`badpacket.Count()`) under leaf
locks or atomics without acquiring `Server.txLock`.
When an optional runtime source (`doctor`, `plugins`, or `reload`) cannot
answer, that section returns `"available": false` with its `reason` or `error`
while every other section still succeeds. `version` reads structured build and
appliance provenance from `version.BuildIdentity`.

<!-- source: internal/component/cmd/show/triage.go -- handleShowSystemTriage, triageRuntime -->
<!-- source: internal/core/version/version.go -- BuildIdentity -->
<!-- source: internal/core/badpacket/badpacket.go -- Count -->

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

### Dataplane drift (`show system drift`)

`internal/core/drift` provides a core leaf registry (`drift.Register`,
`drift.RegisterRemoteCollector`, `drift.RegisterRemoteLister`) for read-only
intent-vs-actual dataplane drift checks bounded to `5s` per subsystem and `64`
findings per subsystem (`MaxFindingsPerSubsystem`). On `Server.StartWithContext`,
`plugin/server` wires `collectPluginDrift` and `listPluginDriftSubsystems` into
`internal/core/drift` and fans out `ze-plugin-callback:drift-check` (`sdk.OnDriftCheck`)
across both internal (`DirectBridge`) and forked (`MuxConn`) plugins declaring
Stage 1 `drift-subsystems` (`firewall`, `fib-kernel`, `interface`).

`show system drift` (`ze-cmd:show-system-drift`) exposes every registered
subsystem under one uniform JSON shape (`status`, `subsystems`, `count`,
`drifted-count`) and registers the command-owned `| subsystem <name>` pipe
filter. `internal/core/drift/register.go` also registers the `dataplane-drift`
`DoctorPhaseRuntime` check under `doctor-dataplane-drift` (fix
`ze show system drift`). No drift checker mutates dataplane state or report-bus
warnings.

<!-- source: internal/core/drift/drift.go -- Register, RegisterRemoteCollector, CheckAll, Check, Subsystems, MaxFindingsPerSubsystem -->
<!-- source: internal/core/drift/register.go -- CodeDoctorDataplaneDrift, checkDataplaneDrift -->
<!-- source: internal/component/plugin/server/drift.go -- collectPluginDrift, queryPluginDrift, listPluginDriftSubsystems -->
<!-- source: internal/component/cmd/show/drift.go -- handleShowSystemDrift, subsystemPipeFilter -->

### Firewall decision trace (`show firewall trace`)

Alongside BGP's existing `show bgp rib best | reason` (`SelectBestExplain`) and
`show policy test` dry-run evaluators, `firewall.EvaluatePacket` provides a pure
read-only decision-trace evaluator over `firewall.LastApplied()`. Exposed as
`show firewall trace` (`ze-firewall:show-trace`, backend-independent across
kernel and VPP), it accepts `table` (mandatory), `chain`, `protocol`,
`source-address`, `destination-address`, `source-port`, `destination-port`,
`input-interface`, `output-interface`, and `connection-state`, walking terms
with a bounded call-stack depth (`maxTraceDepth = 16`) across jumps, gotos,
named sets, modifiers, and default policies. Predicates that require live kernel
runtime state (such as `MatchDestinationLocal`, `MatchIPsecTransform`, or
`MatchTCPFlags`) mark the step `unevaluated: true` and set `incomplete: true`
rather than fabricating a match or verdict.

<!-- source: internal/component/firewall/explain.go -- EvaluatePacket, TraceQuery, TraceStep, TraceResult, maxTraceDepth -->
<!-- source: internal/component/firewall/cmd_trace.go -- handleShowFirewallTrace, parseTraceQuery -->

### Bad-packet ring (`show system bad-packets`)

`internal/core/badpacket` is an always-on 16-slot (`MaxRecords`) circular ring
storing offending wire frames capped at `4096` bytes (`MaxPayloadBytes`, 64 KiB
max total memory) with monotonic `pkt-<N>` IDs, protocol, peer, message type,
action, reason, original length, truncation flag, and lowercase hex payload. It
runs only on cold error and panic paths — BGP RFC 7606 actions
(`session_validation.go`), BGP header/length parse errors on both standard
(`session_read.go`) and coalesced (`session_coalesce.go`) read paths, and
`taskguard` panic recovery (`taskguard.Context.RawFrame`, captured inside
`finishReadBuffer` via `capturePanicFrame` only when `!*kept` before
`returnReadBuffer` recycles the buffer) — adding zero allocations on happy
paths.

`show system bad-packets [count <N>]` (`ze-cmd:show-system-bad-packets`,
`count` `1..16`) returns captured records newest-first with command-owned pipe
filter `| protocol <name>`, and `show system triage` reports `bad-packet-count`
in `runtime`.

<!-- source: internal/core/badpacket/badpacket.go -- Record, Snapshot, Count, CopyBounded, MaxRecords, MaxPayloadBytes -->
<!-- source: internal/component/cmd/show/bad_packets.go -- handleShowSystemBadPackets, protocolPipeFilter -->
<!-- source: internal/component/bgp/reactor/session_read.go -- finishReadBuffer, capturePanicFrame -->
<!-- source: internal/component/bgp/reactor/session_validation.go -- recordBadUpdate -->

### Registrable support bundle modules (`internal/core/supportreg`)

`internal/core/supportreg` replaces the hardcoded central module map in
`internal/component/support/modules.go` with a stdlib-only core leaf registry
(`supportreg.Register(name, fn) func()`, `Names`, `Snapshot`).
`internal/component/support/register.go` registers all 20 existing static
modules plus 7 bounded core live runtime modules (`log-ring`, `report`,
`health`, `watchdog`, `bad-packets`, `drift`, `goroutines`). During
`Server.StartWithContext` / `Server.Stop`, `internal/component/plugin/server`
registers and unregisters 3 daemon-backed live modules (`event-ring`,
`plugins`, `reload-history`) through `internal/core/supportreg` without
importing `internal/component/support`.

<!-- source: internal/core/supportreg/registry.go -- Register, Names, Snapshot, Collector, CollectOptions -->
<!-- source: internal/component/support/modules.go -- ModuleNames, ModuleList, filterModules -->
<!-- source: internal/component/support/runtime_collectors.go -- collectLogRing, collectReport, collectHealth, collectWatchdog, collectBadPackets, collectDrift, collectGoroutines -->
<!-- source: internal/component/plugin/server/support_modules.go -- registerSupportModules, unregisterSupportModules -->

### MCP live triage resource (`ze://triage`)

When constructed with a non-nil command dispatcher (`cfg.Dispatch != nil`), the
MCP Streamable HTTP server appends read-only `ze://triage` to `cachedResources`
for `resources/list`. `resources/read` on `ze://triage` enforces caller command
authorization via `checkCommandScope("show system triage")` and dispatches
`"show system triage"` through `cfg.Dispatch`. Single-site `stampCacheHints`
stamps `ttlLiveStateMs` (`5000` ms) and `cacheScope: "private"` on `ze://`
reads while keeping `3600000` ms (1 h) on embedded `ui://` assets and
`resources/list` (`docs/architecture/mcp/overview.md`).

<!-- source: internal/component/mcp/resources.go -- triageResourceURI, resourcesList, resourcesRead, readTriageResource -->
<!-- source: internal/component/mcp/caching.go -- ttlLiveStateMs, stampCacheHints, resourceReadURI -->

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
