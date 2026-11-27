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
agent with no MCP code change. This held across all eight diagnostic areas and
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
| Live triage | single-call bounded daemon snapshot (`show system triage`) across version, health, doctor, reports, crashes, plugins, reload, recent logs and runtime |
| Execution trace | always-on Go flight recorder, on-demand snapshot capture, and snapshot retrieval (`show system trace`, `request system trace snapshot`) |

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
When an optional runtime source (`doctor`, `plugins`, or `reload`) cannot
answer, that section returns `"available": false` with its `reason` or `error`
while every other section still succeeds. `version` reads structured build and
appliance provenance from `version.BuildIdentity`.

<!-- source: internal/component/cmd/show/triage.go -- handleShowSystemTriage -->
<!-- source: internal/core/version/version.go -- BuildIdentity -->

### Execution flight recorder (`show system trace`)

Daemon startup starts the process-global Go execution flight recorder in
`internal/core/flightrec` (`10s` / `4 MiB` rolling window) and stops it on
shutdown. If `flightrec.Start` returns an error at startup, the daemon logs a
warning and continues to start. Recovered `taskguard` panics and slow plugin
deliveries call `flightrec.CaptureAuto` with a 30-second cooldown to freeze the
preceding execution window into a 4-slot in-memory snapshot ring (4 MiB cap per
snapshot).

Operators and AI agents capture a snapshot on demand with
`request system trace snapshot` (`ze-cmd:request-system-trace-snapshot`), list
recorder status and retained snapshot metadata without raw trace bytes with
`show system trace` (`ze-cmd:show-system-trace`), and fetch one snapshot's
`go-trace-base64` payload with `show system trace id <id>`. Refusals for a
stopped recorder or an unknown identifier return an operator error naming
`ze show system trace`.

<!-- source: internal/component/cmd/show/trace.go -- handleShowSystemTrace, handleRequestSystemTraceSnapshot -->
<!-- source: internal/core/flightrec/flightrec.go -- Start, Stop, CaptureAuto, CaptureNow -->

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
