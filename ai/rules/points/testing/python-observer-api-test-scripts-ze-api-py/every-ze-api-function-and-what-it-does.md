---
kind: table
level:
stage:
---
| Function | Purpose |
|----------|---------|
| `ready()` | Complete all 5 stages, enter event loop (simple usage) |
| `send(cmd)` | Send a text command to the engine |
| `dispatch(api, cmd)` | Send command via API connection |
| `runtime_fail(msg)` | Signal assertion failure (replaces `sys.exit(1)`) |
| `wait_for_shutdown()` | Block until engine shuts down |
| `wait_for_event(timeout, predicate=None)` | Wait for the next event, or (with `predicate`) the first event whose decoded form satisfies it |
| `wait_until(predicate, attempts=20, delay=0.25)` | Poll an arbitrary `predicate()` (e.g. kernel FIB state) until true; returns bool |
| `dispatch_until(api, cmd, predicate, ...)` | Re-dispatch `cmd` until `predicate(result)` is true; returns the winning result dict (also `api.dispatch_until(cmd, predicate, ...)`) |
| `dispatch_until_done(cmd, ...)` | `dispatch_until` with the fixed `status=="done"` predicate |
| `run_rs_observer(expected_peers, forward_prefix=None)` | The standard route-server observer, one line: handshake, wait (event-driven) until every peer's EOR (and `forward_prefix`'s route, when given) is on the wire, then fire-and-forget shutdown. Load-robust successor to the `show bgp` `eor-sent` poll |
| `wait_rs_replayed(expected_peers, forward_prefix=None)` | The readiness half of `run_rs_observer`: block on the async event stream until N EORs (and optionally a route carrying `forward_prefix`) are sent. Returns bool |
| `shutdown_fire_and_forget()` | Send `request shutdown` without blocking on its RPC response (ze may close the connection before replying under load) |
