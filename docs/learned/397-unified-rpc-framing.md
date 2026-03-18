# 397 -- Unified RPC Framing

## Objective

Replace the dual plugin RPC protocol (NUL-delimited JSON with embedded ID + text-mode line protocol) with a single `#<id> <verb> [<json>]\n` wire format. Eliminate text-mode code, merge two multiplexer implementations, and simplify error handling.

## Decisions

- **`#id` prefix for routing, `ok`/`error` as verb, JSON as payload** -- three clean layers (transport, framing, payload) where each layer is independently testable. The mux layer never touches JSON.
- **`CallRPC` returns payload directly, RPC errors as Go errors** -- eliminates the two-step `CallRPC` + `CheckResponse`/`ParseResponse` pattern. Every caller becomes one error check instead of two.
- **Bridge dispatch returns `(payload, error)` matching `CallRPC` semantics** -- the bridge path and socket path produce identical return values for the SDK.
- **Python `ze_api.py` wraps return in `{"result": payload}`** -- maintains backward compatibility with 42 test scripts that do `resp.get('result', {})` without mass test rewrite.
- **`SetWriteDeadline` moved inside write mutex** -- fixes pre-existing race where concurrent dispatch goroutines interleaved deadlines.

## Patterns

- **Newline-safe JSON framing**: compact JSON (output of `json.Marshal`) never contains unescaped newlines, so `\n` is as safe as NUL but far more debuggable (`cat`, `grep`, `tail -f` all work).
- **Protocol-agnostic mux**: `readLoop` extracts `#id` with `strings.Cut` -- no JSON parsing needed for routing. The mux doesn't know or care what the payload contains.
- **Flood protection**: consecutive malformed line counter in `MuxConn.readLoop` disconnects after 100 bad lines, preventing log flooding from malicious plugins.
- **Pool buffer cap**: `WriteBatchFrame` doesn't return oversized buffers (>64KB) to the pool, preventing a single large batch from permanently inflating memory.

## Gotchas

- **Python test library must match wire format**: `test/scripts/ze_api.py` is the bridge between the Go engine and Python plugin test scripts. Changing the Go protocol without updating the Python library causes all 129 functional tests to time out silently -- no error message, just stuck handshake.
- **`InitConns()` was called inside text-mode detection**: the agent that removed text-mode detection also removed the `InitConns()` call, leaving `ConnA()`/`ConnB()` returning nil. Caused chaos/inprocess test to hang. Fix: add `InitConns()` back in both `startup.go` and `subsystem.go`.
- **`directResultResponse` returned `(nil, nil)` on marshal error**: old code encoded marshal errors in the JSON envelope; new code silently returned nil. SDK interpreted `(nil, nil)` as success. Fix: return the error as `*RPCCallError`.

## Files

- **Core**: `pkg/plugin/rpc/` (framing.go, message.go, conn.go, mux.go, batch.go, types.go)
- **SDK**: `pkg/plugin/sdk/sdk.go`
- **Server**: `internal/component/plugin/server/` (dispatch.go, startup.go, subsystem.go)
- **IPC**: `internal/component/plugin/ipc/rpc.go`, `internal/core/ipc/` (message.go, dispatch.go)
- **Process**: `internal/component/plugin/process/` (process.go, delivery.go)
- **Test lib**: `test/scripts/ze_api.py`
- **Docs**: `docs/architecture/api/wire-format.md`, `ipc_protocol.md`
- **Deleted (9 files, ~2,500 lines)**: text.go, text_conn.go, text_mux.go, sdk_text.go, startup_text.go, subsystem_text.go, and their tests
