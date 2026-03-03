# 210 — YANG IPC Plugin Protocol

## Objective

Replace the plugin startup protocol from stdin/stdout text pipes to YANG RPC calls over two Unix socket pairs, and provide a Go plugin SDK implementing the new protocol.

## Decisions

- Two socket pairs required (not one): one socket per direction prevents deadlock — if a single socket were used, both sides could block waiting for the other to respond while simultaneously holding an unread request.
- `net.Pipe()` has zero kernel buffering: writes block until the reader reads. Tests using `net.Pipe` must wrap writes in goroutines to avoid deadlocks that do not occur with OS socketpairs (which have kernel buffers).
- Shared `rpc.Conn` in `pkg/plugin/rpc/conn.go` eliminates code duplication between the engine's `PluginConn` and the SDK's `Plugin` — the framing protocol is symmetric; only the typed method wrappers differ.
- `sdk/types/types.go` skipped (YAGNI): types are defined inline in `sdk.go` and `rpc_plugin.go`; a separate shared types package added complexity with no benefit at this stage.
- stdin/stdout replacement deferred: deleting the old protocol simultaneously across all 8 plugins was blocked on per-plugin conversion specs. Infrastructure was completed; the removal happens alongside each plugin conversion.

## Patterns

- Two-socket bidirectional RPC: Socket A (plugin→engine) and Socket B (engine→plugin) cleanly separate who initiates each call and prevent deadlock at the protocol level.
- `connFromFD` helper keeps FD→net.Conn conversion with proper resource cleanup in one place; callers never manage raw FDs.
- `handleNLRICallback` factory generalizes any request→result RPC (encode-nlri, decode-nlri, decode-capability) — avoids per-RPC boilerplate.
- Startup synchronization in tests: before calling runtime methods on Socket A, prove the event loop is running by sending an event on Socket B first, to avoid data races on the shared reader.

## Gotchas

- `net.Pipe()` zero-buffering deadlock: sequential write-then-read on `net.Pipe` blocks because there is no buffer. Always wrap writes in goroutines in tests using `net.Pipe`.
- Response ID verification must be symmetric: if the SDK verifies IDs, the engine must too. Missing verification on the engine side was found and added during review.
- `NewFromEnv` must be implemented if documented — it was documented in the package doc but missing from the code. External plugins reading env vars would have had no constructor.

## Files

- `internal/plugin/socketpair.go` — dual socket pair creation for internal and external plugins
- `internal/plugin/rpc_plugin.go` — `PluginConn` with typed methods for all 5 startup stages + runtime RPCs
- `pkg/plugin/rpc/conn.go` — shared NUL-framed JSON RPC connection (used by engine and SDK)
- `pkg/plugin/rpc/types.go` — canonical shared RPC types
- `pkg/plugin/sdk/sdk.go` — plugin SDK with callback-based API and `Run()` lifecycle
- `internal/component/config/yang/modules/ze-plugin-engine.yang` — 6 engine-serves-plugin RPCs
- `internal/component/config/yang/modules/ze-plugin-callback.yang` — 8 plugin-serves-engine RPCs
