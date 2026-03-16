# 276 — RPC Multiplexing (Concurrent Plugin→Engine Calls)

## Objective

Eliminate the `callMu` serialization bottleneck in `pkg/plugin/rpc/conn.go` that caused concurrent plugin-to-engine RPC calls to serialize, leading to silent route drops under heavy load (1M+ routes from one peer starving workers for other peers).

## Decisions

- Chose `MuxConn` as a separate wrapper type rather than modifying `Conn` directly: `Conn.ReadRequest()` is used by the engine on Socket A and the SDK event loop on Socket B — starting a background reader inside `Conn` would conflict with those callers. Composition avoids this.
- `MuxConn` is in the same package as `Conn` (`pkg/plugin/rpc`), so it accesses `conn.reader` directly without exporting it — no package boundary change needed.
- Startup stages (5-stage protocol) remain sequential using `Conn.CallRPC`. `MuxConn` is only created after Stage 5, when concurrent engine calls become relevant. One execution thread before the event loop means multiplexing adds no value there.
- Used `sync.Map` for pending responses: access pattern is write-once-delete-once with no iteration, and keys are unique request IDs. This is exactly `sync.Map`'s ideal use case.
- Both sides require concurrent dispatch: multiplexing on the plugin side alone is insufficient — if the engine still processes requests sequentially, multiplexed requests queue in the socket buffer. `handleSingleProcessCommandsRPC` must also dispatch in goroutines.
- Raised RR `updateRoute` timeout from 10s to 60s as defense-in-depth. Even with multiplexing, transient congestion can delay responses.

## Patterns

- Background reader goroutine routes responses to callers by request ID via buffered channels (capacity 1). Callers wait on their own channel — no global lock held during write+wait.
- Socket B (engine→plugin event delivery) is a separate concern and kept sequential — only Socket A (plugin→engine RPCs) needed multiplexing in this spec.

## Gotchas

- `blocking_test.go` in bgp-rib documents Socket B head-of-line blocking — that is a separate issue from Socket A multiplexing. The test was correctly left unchanged; the blocking it documents is orthogonal to this fix.
- Orphaned responses (response arrives after caller timed out or canceled) are logged as warnings, not errors. This is a normal race condition during cancellation — the response arrived just after the pending entry was removed.

## Files

- `pkg/plugin/rpc/mux.go` — `MuxConn` type with background reader and `CallRPC`
- `pkg/plugin/rpc/mux_test.go` — 7 tests covering concurrency, cancellation, close, reader errors
- `pkg/plugin/sdk/sdk.go` — `engineMux` field, `callEngineRaw` dispatch, `Close` cleanup
- `internal/component/plugin/server.go` — `wg.Go` concurrent dispatch in `handleSingleProcessCommandsRPC`
- `internal/component/bgp/plugins/rs/server.go` — `updateRouteTimeout` constant, 10s→60s
