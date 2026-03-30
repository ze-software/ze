# Investigation: check.ci test slowness and intermittent timeout

## Symptom

- `test/plugin/check.ci` consistently takes ~6.2s in isolation (5 runs: 6.2-6.4s)
- Under concurrent `make ze-verify` load, it times out at 10s
- Failure: "expected messages: 1, received messages: 0" -- ze never sent the plugin's response

## What the Test Does

1. ze-peer starts (background), listens for BGP connections
2. ze starts via stdin pipe, with an **external Python plugin** (`check.run`)
3. ze forks Python, runs 5-stage IPC protocol (declare/config/capability/registry/ready)
4. `WaitForPluginStartupComplete()` blocks peer startup until plugin is ready
5. ze connects to ze-peer, OPEN exchange
6. ze-peer sends default route (`0.0.0.0/32`) immediately after OPEN
7. ze delivers UPDATE event to plugin as JSON
8. Plugin sends back `1.2.3.4/32` via API command
9. ze forwards as BGP wire bytes, ze-peer validates

## Suspected Root Cause: 5s Reconnect Delay

`DefaultReconnectMin = 5s` at `internal/component/bgp/reactor/peer.go:74`.

If the first TCP connect from ze to ze-peer fails, `peer_run.go:94` waits 5s before retry.
The test config has no `connect-retry` override. 6.2s = ~1s startup + 5s reconnect + ~0.2s exchange.

**Open question:** Why would the first connect fail? The runner waits for ze-peer to be listening
(`runner_exec.go:576-587`) before starting ze, and ze itself waits for the plugin before connecting
(`reactor.go:755`). By the time ze tries to connect, ze-peer should have been listening for seconds.

## Key Code Locations

| File:Line | What |
|-----------|------|
| `internal/component/bgp/reactor/peer.go:74` | `DefaultReconnectMin = 5s` |
| `internal/component/bgp/reactor/peer_run.go:30-94` | Peer run loop with backoff, delay at line 94 |
| `internal/component/bgp/reactor/reactor.go:755` | `WaitForPluginStartupComplete()` |
| `internal/test/runner/runner_exec.go:576-587` | Runner waits for ze-peer to be listening |
| `internal/test/runner/runner_exec.go:234` | `ze_plugin_stage_timeout=10s` |
| `test/scripts/ze_api.py:62-63` | Plugin poll: `read_line(0.5)` for 8s max |
| `test/plugin/check.ci:175` | `timeout=10s` on the foreground ze process |

## Investigation Steps

1. **Instrument timing** -- add timestamps to peer connection loop to find where 5-6s goes:
   is there a failed TCP connect + 5s retry? How long is the 5-stage protocol?

2. **Test with `connect-retry 1`** -- add to peer config in check.ci, measure: does it drop to ~1.5s?

3. **Test without Python plugin** -- minimal check.ci using internal plugin, isolate plugin vs connection

4. **Check port reuse under load** -- shifted ports under concurrent tests could cause stale connections

5. **Profile `read_line(0.5)` poll** -- each missed poll = 0.5s latency on event delivery

## Potential Fixes

| Fix | Effort | Impact |
|-----|--------|--------|
| Add `connect-retry 1` to test config | trivial | Confirms/fixes if reconnect is the bottleneck |
| Reduce `DefaultReconnectMin` for tests | small | Faster tests, may mask real issues |
| Replace Python plugin with internal | medium | Eliminates fork + IPC overhead |
| Increase `timeout=10s` to `timeout=15s` | trivial | Band-aid only |
| Reduce `read_line` poll interval | small | Faster event pickup |
