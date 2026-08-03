# 824 -- RIB Reconnect Replay Batching

## Context

When a BGP peer reconnects, the RIB plugin replays its stored Adj-RIB-Out back to the engine. The old path issued one `UpdateRoute` RPC per route (JSON marshal/unmarshal, text tokenization, NLRI parsing), costing ~100-200us per call. For a peer holding 100K routes, this meant ~10s of blocking replay. The dominant cost was per-call RPC overhead, not attribute encoding. The goal was to reduce call count from O(routes) to O(distinct-attribute-sets).

## Decisions

- Chose `update cursor` (stateful text protocol with delta encoding) over DirectBridge binary protocol, because it stays within the existing dispatch path, works for external plugins, and is debuggable/loggable.
- Chose `update cursor` as a fourth encoding mode in `handleUpdate` switch over a separate `replay` command tree, because it reuses existing YANG registration and dispatch with a one-line switch case.
- Chose package-level `sync.Map` for cursor state over attaching state to `CommandContext` or `Process`, because handlers are stateless (existing pattern) and this avoids coupling to infrastructure.
- Chose `del <attr>` reintroduction only in cursor mode over supporting it in `update text`, because `text` mode intentionally removed `set`/`del` for simplicity.
- Chose `RegisterProcessCleanup` callback pattern over direct import of cursor package in server, to avoid import cycle (server -> cmd/update).
- Chose wire-hash-sans-AS_PATH for sort key over MsgID-only sorting, because consecutive groups with same hash need only `as-path [...]` delta (most common transition between source peers).

## Consequences

- Replay cost drops from O(N * 100-200us) to O(M * 100-200us) where M = distinct attribute sets (typically <<< N). Benchmark: 1.8ms for 1K groups / 100K routes.
- `collectAllRibOutRoutes` (per-route reconstruction) is no longer called from replay paths. The grouped variant decodes each `AttrHandle` once.
- External plugins can use `update cursor` protocol for their own batched updates.
- `sendRoutes` (manual resend) now also uses grouped cursor mode via `resendRoutesWithCursor` (commit b00238505), groups by `(family, AttrHandle, pathID, StaleLevel)`.
- `RegisterProcessCleanup` hook is now available for any command package needing per-process cleanup without import cycles.

## Gotchas

- The `ipv4Unicast` package-level var already existed in `rib_metrics_test.go` (as `family.IPv4Unicast`). Redeclaring it as a function in test files causes compile errors.
- The hook `block-init-register.sh` blocks `init()` registration of cleanup callbacks. Solution: add to existing `init()` in `update_text.go` where RPCs are already registered.
- `sort` import in `rib.go` becomes unused after removing `replayRoutes` (which used `sort.Slice`). The new sort lives in `rib_replay.go`.
- RFC 9494 stale metadata is only threaded, not yet honored end to end. `resendRoutesWithCursor` (`rib_replay.go`) routes stale groups through `updateRouteWithMeta`, which carries `map[string]any{"stale": level}` into `CommandContext.Meta` (set at `dispatch.go` and `:565` from `input.Meta`). But no update handler or egress filter reads `CommandContext.Meta`: it is written and never consumed. The only egress reads of `.Meta` are `ReceivedUpdate.Meta` on the peer-to-peer forwarded path (`reactor_api_forward.go,556`, `forward_rs.go,367`), not the plugin-originated cursor path. So the cursor carries the metadata but the update egress path does not act on it yet. Documented gap: `pkg/plugin/sdk/sdk_engine.go`. Do not describe this as preserving RFC 9494 metadata end to end.

## Files

- `internal/component/bgp/plugins/cmd/update/cursor.go` (new) -- cursor handler, state, ClearProcessCursors
- `internal/component/bgp/plugins/cmd/update/cursor_test.go` (new) -- 13 cursor tests
- `internal/component/bgp/plugins/cmd/update/update_text.go` (modified) -- add cursor case + cleanup registration
- `internal/component/bgp/plugins/rib/rib_replay.go` (new) -- grouped collection, delta formatting, sorted replay
- `internal/component/bgp/plugins/rib/rib_replay_test.go` (new) -- 7 replay tests + benchmark
- `internal/component/bgp/plugins/rib/rib.go` (modified) -- use grouped cursor replay in handleState/handleStructuredState
- `internal/component/plugin/server/dispatch.go` (modified) -- call cleanup hooks in cleanupProcess
- `internal/component/plugin/server/rpc_register.go` (modified) -- RegisterProcessCleanup hook registry
- `docs/architecture/api/commands.md` (modified) -- document update cursor protocol
