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
agent with no MCP code change. This held across all seven diagnostic areas and
is the reason none of them needed a bespoke transport.

| Area | What it exposes |
|------|-----------------|
| Runtime state | L2TP observer and CQM, interface counters, component and plugin status, traffic |
| Event history | global event ring, BGP peer FSM history, L2TP tunnel and session FSM history |
| Metrics query | Prometheus text filtered by name and labels, L2TP and BGP health summaries |
| Packet capture | decoded rings and raw byte capture (`packet-capture.md`) |
| Active probes | ping, traceroute, route lookup (`active-probes.md`) |
| Health | component health registry, HTTP `/health` returning 200 or 503 |
| Log query | slog ring handler filtered by level, component and count |

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
The fixed entry capacity and newest-first level/component/count filters are unchanged.

The ring is inside the existing level and debug-filter handler chain. It does not
bypass filtering, expand a value that its `LogValuer` redacts, or change command
authorization. Log producers remain responsible for excluding secrets, just as
they are for the configured text/syslog backend. Crash reports keep their existing
message-only recent-log header rather than adding structured fields implicitly.

<!-- source: internal/plugins/log/cmd/handlers.go -- log query attributes -->

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
