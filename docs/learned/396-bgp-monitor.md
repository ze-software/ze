# 396 -- bgp-monitor

## Objective

Add a `bgp monitor` CLI command that streams live BGP events via SSH, with keyword-based filtering for event type, peer, and direction.

## Decisions

- **SSH-based streaming, not Unix socket JSON-RPC:** The original spec assumed a clientLoop/NUL-framed JSON-RPC transport that doesn't exist. The CLI uses SSH exec. Streaming keeps the SSH session open and writes events line-by-line.
- **JSON wire format, text default display:** Engine always sends JSON (NDJSON). Default display is visual text one-liner rendered client-side. Pipe operators (`| json`, `| table`, `| yaml`, `| match`) work because underlying data is JSON.
- **MonitorManager parallel to SubscriptionManager:** CLI monitors have no `*process.Process`, so a parallel manager keyed on client ID (not process pointer) was needed.
- **StreamingHandler registry:** Infrastructure (`loader.go`) must not import plugin implementations. The monitor plugin registers a `StreamingHandler` via init(), and the loader looks it up from the registry.
- **Authorization on streaming path:** The SSH streaming executor passes the username to the handler, which checks `Dispatcher.IsAuthorized()` before starting the stream. Without this, the streaming path would bypass authz.

## Patterns

- **Non-blocking backpressure:** `enqueue()` uses `select/default` with atomic `Dropped` counter. Warning piggybacked on next successfully delivered event.
- **Format cache reuse:** Monitor delivery reuses the `json+parsed` entry from the plugin format cache (key `monitorFormatKey`), avoiding duplicate formatting when plugins and monitors share the same format.
- **Context-based cleanup:** `StreamMonitor` creates a client-scoped context with deferred `cancel()` + `mm.Remove(id)`. All exit paths (write error, context cancel) trigger cleanup automatically.
- **Thread-safe test buffer:** Integration tests for StreamMonitor use a `syncBuffer` (mutex-protected `bytes.Buffer`) to avoid races between the writing goroutine and polling assertions.

## Gotchas

- **Spec assumed nonexistent transport:** Original spec referenced `client.go`, `clientLoop`, NUL-framed JSON-RPC, `Request.More`/`RPCResult.Continues` -- none of which exist for CLI clients. SSH exec is the actual path.
- **FormatMonitorLine JSON nesting:** First implementation parsed a flat JSON structure but production ze-bgp JSON nests everything under `"bgp":{}`. Tests passed because they used fake flat JSON (self-consistent but wrong). Deep review caught this.
- **RPC count tests break on new RPCs:** Multiple test files (`rpc_registration_test.go`, `cmd/ze/schema/main_test.go`) hardcode expected RPC counts. Any new RPC (from any session) breaks these.
- **Parallel sessions cause conflicts:** Another session's CI changes (adding `daemon-quit` RPC, `dumpGoroutines`) bled into our agent-edited files. Always verify agent changes don't include unrelated code.

## Files

- `internal/component/bgp/plugins/cmd/monitor/` -- monitor command handler, format, YANG schemas (new)
- `internal/component/plugin/server/monitor.go` -- MonitorManager type (new)
- `internal/component/plugin/server/handler.go` -- StreamingHandler registry (modified)
- `internal/component/plugin/server/server.go` -- monitors field + Monitors() accessor (modified)
- `internal/component/bgp/server/events.go` -- monitor delivery in all 6 event functions (modified)
- `internal/component/ssh/ssh.go` -- streaming executor, execMiddleware monitor detection (modified)
- `internal/component/bgp/config/loader.go` -- streaming executor factory wiring (modified)
- `cmd/ze/cli/main.go` -- StreamMonitor, blank import (modified)
- `cmd/ze/internal/sshclient/sshclient.go` -- StreamCommand (modified)
- `internal/component/cli/model_monitor.go` -- BubbleTea interactive monitor mode (new)
- `docs/architecture/api/architecture.md` -- Monitor Streaming section (modified)
- `docs/architecture/api/commands.md` -- Monitor command entry (modified)
- `test/plugin/monitor-{basic,events,peer}.ci` -- functional tests (new)
