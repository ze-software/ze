# 414 -- Forward Barrier

## Context

Functional tests using Python plugin scripts had fragile `time.sleep()` calls between `send()` calls to wait for BGP UPDATE delivery. The `send()` RPC returns when the command is dispatched to the reactor, but the forward pool writes to peer sockets asynchronously. Under parallel test load, 0.2-0.5s sleeps were insufficient, causing transient failures (ipv6.ci was the trigger). The goal was a deterministic barrier command that blocks until forward pool items are on the wire.

## Decisions

- Chose sentinel pattern over polling for the barrier. A sentinel `fwdItem` (done callback only, no data) is dispatched to each worker's channel. FIFO ordering guarantees all prior items are processed when the sentinel's callback fires. Zero overhead on the hot path, no busy-wait.
- Chose `peer <selector> flush` as the command name over `drain` or `barrier`. "Flush" matches buffer-flush semantics (`fflush()`/`fsync()`), not "remove all entries" (`ip route flush`).
- Chose to keep inter-message `time.sleep()` in `.ci` test files. Removing them broke 5 tests because `.ci` tests have two command sources: the plugin script AND ze-peer's `cmd=api` lines. The sleeps ensure ordering between these independent sources.
- Chose to change `wait_for_ack()` implementation (flush RPC + post-flush delay) over creating a new `flush()` function. `wait_for_ack()` already exists at every call site that needs the barrier.
- ExaBGP bridge transparent barrier blocked on bridge MuxConn incompatibility (tracked in `spec-exabgp-bridge-muxconn.md`).

## Consequences

- Plugins can now call `wait_for_ack()` for deterministic route delivery confirmation. The flush RPC guarantees forward pool drain; the post-flush delay covers ze-peer `cmd=api` interleaving.
- The `peer <selector> flush` RPC is available to any plugin, not just test scripts. Production plugins could use it for ordered operations (announce then withdraw).
- The ExaBGP bridge's runtime I/O was discovered to be non-functional with MuxConn. Both command dispatch and event reception silently fail after the 5-stage protocol. This predates the barrier work but was only discovered by tracing the flush path through the bridge.
- The sentinel pattern adds a nil-peer guard to `fwdBatchHandler`. Sentinel items (nil peer, no data) return early from the handler; their `done` callback is still called by `safeBatchHandle`.

## Gotchas

- `cmd=api` lines in `.ci` tests are commands that ze-peer sends to ze via the API, not just declarations. The EOR in ipv4.ci comes from ze-peer, not the plugin. Removing inter-message sleeps caused ordering races between these two independent command sources.
- The forward pool's `TryDispatch` creates workers lazily. If `flush()` is called for a peer with no worker (session not established), the barrier returns immediately (AC-3). Routes may be cached but not forwarded yet.
- `fwdBatchHandler` accesses `items[0].peer` -- a sentinel as the first batch item would crash without the nil-peer guard.
- The ExaBGP bridge writes raw text to os.Stdout after stage 5, but ze wraps the connection in MuxConn which drops lines without `#<id>` prefix. The bridge was written before MuxConn adoption.

## Files

- `internal/component/bgp/reactor/forward_pool_barrier.go` -- `Barrier()`, `BarrierPeer()` (sentinel pattern)
- `internal/component/bgp/reactor/forward_pool_barrier_test.go` -- 6 unit tests
- `internal/component/bgp/reactor/forward_pool.go` -- nil-peer guard in `fwdBatchHandler`
- `internal/component/plugin/types.go` -- `FlushForwardPool`, `FlushForwardPoolPeer` on `ReactorPeerController`
- `internal/component/bgp/reactor/reactor_api.go` -- `reactorAPIAdapter` wiring
- `internal/component/bgp/plugins/cmd/peer/peer.go` -- `handleBgpPeerFlush` handler
- `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` -- `flush` container
- `internal/component/bgp/schema/ze-bgp-api.yang` -- `peer-flush` RPC
- `test/scripts/ze_api.py` -- `wait_for_ack()` sends flush RPC + post-flush delay
- `internal/exabgp/migration/migrate_family.go` -- add default prefix maximum (10000) to migrated families
- `internal/exabgp/migration/migrate_serialize.go` -- serialize family prefix blocks in migration output
- `plan/spec-exabgp-bridge-muxconn.md` -- created (skeleton, bridge I/O fix)
