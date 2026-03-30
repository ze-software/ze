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

## Suspected Root Cause: 5s Reconnect Delay -- DISPROVED

~~`DefaultReconnectMin = 5s` at `internal/component/bgp/reactor/peer.go:74`.~~

Debug timing shows the first connect **succeeds immediately** (670us dial, attempt 1).
Plugin startup takes ~180ms, full dial+OPEN+ESTABLISHED under 2ms.
No reconnect occurs. The reconnect hypothesis was wrong.

## Actual Root Cause: OnMessageSent Blocks for 5s

**`internal/component/bgp/server/events.go:697-702` -- synchronous wait on event delivery.**

Timeline from debug logs (all times UTC, 2026-03-30):

| Time | Delta | Event |
|------|-------|-------|
| 21:23:41.268 | T+0 | Plugin startup begins |
| 21:23:41.449 | +181ms | `WaitForPluginStartupComplete` done (180ms) |
| 21:23:41.449 | +0ms | `WaitForAPIReady` done (<1us) |
| 21:23:41.450 | +1ms | Dial succeeded (670us) |
| 21:23:41.451 | +1ms | FSM ESTABLISHED |
| 21:23:41.451 | +0ms | `sendInitialRoutes`: sends first static route (127.0.0.1/32) |
| 21:23:41.451 | +0ms | `OnMessageSent` called -- notifies plugin about **sent** UPDATE |
| **21:23:46.456** | **+5.005s** | `OnMessageSent` **write failed: context deadline exceeded** |
| 21:23:46.456 | +0ms | Second static route (::1/128) sent |
| 21:23:46.457 | +1ms | 500ms API sleep starts |
| 21:23:46.957 | +500ms | Queue drain: plugin's 1.2.3.4/32 route sent + EOR |
| 21:23:46.958 | +1ms | ze-peer validates all messages, test passes |

**Total: 5.7s = 5.005s (OnMessageSent timeout) + 0.5s (API sleep) + 0.2s (startup+exchange)**

### Why OnMessageSent blocks

1. Python plugin calls `subscribe(['update'])` -- subscribes to UPDATE events
2. This includes **sent** direction, not just received
3. When ze sends its first static route, `OnMessageSent` delivers a "sent UPDATE" notification
4. Delivery to external plugin: write notification to plugin's stdin pipe
5. Python plugin is in `read_line(0.5)` poll loop, hasn't consumed previous messages yet
6. Pipe write blocks until plugin reads or context expires (~5s)
7. `events.go:697-702` waits synchronously: `for range sent { r := <-results }`
8. After 5s timeout, the blocking call finally returns and `sendInitialRoutes` continues

## Key Code Locations

| File:Line | What |
|-----------|------|
| **`internal/component/bgp/server/events.go:697-702`** | **ROOT CAUSE: synchronous wait on delivery results** |
| **`internal/component/bgp/server/events.go:644`** | **GetMatching with DirectionSent -- finds plugin subscriber** |
| **`internal/component/bgp/reactor/peer_initial_sync.go:148-152`** | **500ms API sleep (secondary delay)** |
| `internal/component/bgp/reactor/peer.go:74` | `DefaultReconnectMin = 5s` (not the issue) |
| `internal/component/bgp/reactor/peer_run.go:30-94` | Peer run loop with backoff (not triggered) |
| `internal/component/bgp/reactor/reactor.go:755` | `WaitForPluginStartupComplete()` (180ms, fast) |
| `internal/component/bgp/reactor/api_sync.go:14` | `DefaultAPITimeout = 5s` (context for delivery) |
| `internal/test/runner/runner_exec.go:576-587` | Runner waits for ze-peer to be listening |
| `internal/test/runner/runner_exec.go:234` | `ze_plugin_stage_timeout=10s` |
| `test/scripts/ze_api.py:62-63` | Plugin poll: `read_line(0.5)` for 8s max |
| `test/plugin/check.ci:175` | `timeout=10s` on the foreground ze process |

## Investigation Steps (completed)

1. ~~Instrument timing~~ **Done** -- Added debug timing to `peer_run.go` and `reactor.go`.
   Result: dial succeeds on attempt 1 in <1ms. Plugin startup 180ms.

2. ~~Test with `connect-retry 1`~~ **Not needed** -- reconnect is not the issue.

3. ~~Test without Python plugin~~ **Done** -- Without plugin, everything completes in <1s.

4. ~~Check port reuse under load~~ **Not the issue** -- port is correctly set via `ze_bgp_tcp_port` env var.

5. ~~Profile `read_line(0.5)` poll~~ **Identified as secondary** -- the pipe write timeout is the primary issue.

## Potential Fixes (updated)

| Fix | Effort | Impact |
|-----|--------|--------|
| **Make OnMessageSent async for external plugins** | medium | Eliminates 5s block. Main fix. |
| **Don't subscribe to sent events by default** | small | `subscribe(['update'])` should default to received only |
| **Add write timeout to event delivery** | small | Reduce 5s to e.g. 100ms with drop-on-timeout |
| Use non-blocking pipe writes | medium | Prevents blocking but may lose events |
| Replace Python plugin with internal for test | medium | Eliminates IPC overhead, fast test |
| Reduce `read_line` poll interval | small | Marginal improvement (not the bottleneck) |

## Recommended Fix

The `OnMessageSent` path in `events.go:697-702` synchronously waits for ALL plugin
deliveries to complete before returning. This blocks `sendInitialRoutes` for 5s when
an external plugin has a full pipe buffer.

**Option A (targeted):** Make `OnMessageSent` non-blocking -- don't wait for delivery results.
Sent notifications are informational; losing one is acceptable.

**Option B (broader):** Add a short delivery timeout (e.g., 200ms) for external plugin event
delivery. If the plugin can't consume in time, drop the event and log a warning.

**Option C (test-only):** The Python plugin subscribes to `['update']` which includes sent
direction. If `subscribe(['update'])` only subscribed to received UPDATEs, the sent notification
would have zero subscribers and `OnMessageSent` would return immediately (line 647).
