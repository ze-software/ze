# 294 — In-Process Direct Transport

## Objective

Eliminate JSON serialization and socket I/O overhead for internal plugins by replacing the transport layer with direct Go function calls, motivated by profiling showing ~27% CPU in syscall and ~36% in goroutine scheduling from plugin IPC.

## Decisions

- Bridge reference travels via `BridgedConn` (net.Conn wrapper) through the existing `InternalPluginRunner func(engineConn, callbackConn net.Conn) int` signature — no signature changes, no plugin code changes needed; SDK discovers the bridge via type assertion in `NewWithConn()`
- 5-stage startup stays on sockets (cold path, 5 round-trips total) — avoids complex handshake synchronization, keeps bridge simple for the hot path only
- JSON event strings remain (`onEvent(string)`) — eliminating them would change the plugin API contract, a larger change; direct transport eliminates only the transport wrapping (JSON-RPC envelope, NUL framing, pipe I/O, response ack)
- Nil bridge = socket fallback, atomic.Bool for ready signal — same sequential execution model as the existing socket eventLoop
- `wireBridgeDispatch` must be called BEFORE Stage 5 OK — race fix: engine must register `DispatchRPC` before SDK calls `SetReady()`
- Direct handlers return `json.RawMessage` only (not `(json.RawMessage, error)`) — `nilerr` linter fires on `if err != nil { return ..., nil }` pattern; caller adds `, nil`
- Bridge creation and conn wrapping moved to `process.go:startInternal()` instead of `inprocess.go` — collocates bridge lifecycle with process lifecycle

## Patterns

- BridgedConn: wrap net.Conn + carry bridge reference, implement net.Conn by delegation — bridge reference travels transparently through existing interface
- Type assertion for feature discovery (Bridger interface) — explicit, no hidden magic
- Bridge has no shutdown resources — `Process.Stop()` closes sockets, delivery goroutine exits naturally

## Gotchas

- Previous attempts (bufio.Writer on TCP, 16MB SO_SNDBUF/SO_RCVBUF) showed the bottleneck is plugin IPC transport, not TCP wire writes — confirmed by profiling before implementing
- Race condition: SDK calls `SetReady()` at end of Stage 5; engine must wire `DispatchRPC` before that point, not after the startup loop

## Files

- `pkg/plugin/rpc/bridge.go` — `DirectBridge`, `BridgedConn`, `Bridger` interface
- `internal/component/plugin/process.go` — bridge field, startInternal() creates bridge, deliverBatch() uses bridge
- `internal/component/plugin/server_dispatch.go` — direct handler extraction, `wireBridgeDispatch()`
- `internal/component/plugin/server_startup.go` — `wireBridgeDispatch` call before Stage 5 OK
- `pkg/plugin/sdk/sdk.go` — bridge discovery, `callEngineRaw()` uses bridge when ready
