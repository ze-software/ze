# quiesce-peer-drain

Made `ze_api.wait_for_ack` sleepless by (1) adding a second reactor quiescer
`bgp-peer-sync` that drains the per-peer initial-sync opQueue the forward-pool
quiescer misses, and (2) fixing how the Python test harness INVOKES the barrier.
`nexthop.ci` (same-prefix ordered re-announce, ::1 pre-EOR then ::2/::1 post-EOR)
now passes with no `time.sleep`, 25/25 repeated and green across the full plugin
suite (472/474; the 2 reds were scratch debug files).

## The load-bearing finding: barriers are invoked via dispatch-command, not `_call_engine(wire-method)`

`wait_for_ack` had ALWAYS called `self._call_engine("ze-bgp:peer-flush", ...)`.
That RPC method **does not exist as a plugin-callable engine op**:
`dispatchPluginRPC` (`internal/component/plugin/server/dispatch.go:79`) routes only
`ze-plugin-engine:*` engineOps (`dispatch_registry.go:58` `engineOps`) plus codec
RPCs. `ze-bgp:peer-flush` / `ze-system:quiesce` are api-yang RPCs registered in the
**command dispatcher keyed by their YANG command PATH** (`command.go:53` LoadBuiltins;
`cmd/peer/peer.go:43` maps `ze-bgp:peer-flush` -> command `request peer <sel> flush`).
So `_call_engine("ze-bgp:peer-flush")` returns `unknown method`, and
`wait_for_ack`'s `except RuntimeError: pass` swallowed it. **The peer-flush barrier
never ran; the `time.sleep(0.2*count)` did 100% of the work.**

That is the whole reason `wait_for_ack` "kept changing" / yo-yo'd: every attempt to
"remove the sleep and rely on peer-flush" removed the ONLY thing that worked, because
the barrier was a silent no-op. The earlier "AC-6 is unachievable via a barrier"
conclusion was FALSE for the same reason -- that attempt drove `quiesce()` through the
same broken `_call_engine("ze-system:quiesce")` path, so its DrainPeerSync barrier
was never invoked either.

**Correct invocation (verified with a probe plugin):**
`api.dispatch("request quiesce")` -> `ze-plugin-engine:dispatch-command` -> dispatcher
-> `handleQuiesce` -> runs both quiescers, returns
`{status: done, data: {quiesced: [bgp-forward-pool, bgp-peer-sync]}}`.
`quiesce()` and `wait_for_ack()` now dispatch the command; no sleep.

## Why the second quiescer (`bgp-peer-sync`) is required

Routes a plugin `send()`s while a peer is not-yet/just Established are queued in the
peer's **opQueue** (`peer.go:844` `ShouldQueue`) and drained DIRECT to the session in
`sendInitialRoutes` (`peer_initial_sync.go:348-405`), bypassing the forward pool.
`bgp-forward-pool` (`FlushForwardPool` -> `fwdPool.Barrier`) never sees them. Only
`DrainPeerSync` (polls `!PendingSync()` = `sendingInitialRoutes==0 && opQueue empty`)
waits until the initial-sync EOR is on the wire, so the NEXT `send()` lands as a
post-EOR incremental update. The two queues are independent, so `quiesceAll` runs the
two quiescers concurrently.

## Diagnosis method that worked (after two wrong theories)

Log-capture from `.ci` runs is unreliable (RPC-dispatch logs weren't in the captured
ClientOutput; a passing test dumps nothing). What settled it: a **probe plugin** that
calls each candidate invocation and reports the literal result/error via
`runtime_fail(...)` (its `ZE-OBSERVER-FAIL` sentinel is guaranteed relayed and shown
on failure). That surfaced `unknown method: ze-bgp:peer-flush` directly. Use
`ze.bin=<abs path>/bin/ze` to make `ze-test` run a prebuilt daemon when its internal
build is broken by an unrelated in-flight file.

## Traps for the next agent

- A plugin's `_call_engine("<module>:<rpc>")` only works for `ze-plugin-engine:*` ops
  and codec RPCs. Everything else (peer-flush, quiesce, peer teardown, ...) is a
  **command**: `api.dispatch("request ...")` / `api.send("peer * ...")`.
- `wait_for_ack`'s `except RuntimeError: pass` hid a dead barrier for a long time. When
  a "barrier + sleep" is suspicious, delete the sleep and check the test still passes --
  if it doesn't, the barrier was never doing anything.
- `ze_api.py` is a deployed test script, not compiled into `ze`; swap it and re-run
  with the SAME `bin/ze` to isolate a harness-side change from an engine change.

## Files

None recorded.
