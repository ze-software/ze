# 497 -- check.ci Test Slowness (6.2s to 1.4s)

## Context

The `test/plugin/check.ci` functional test consistently took 6.2s in isolation and timed out (10s) under concurrent `make ze-verify` load. The test exercises the full external plugin lifecycle: ze forks a Python plugin, runs the 5-stage IPC handshake, connects to ze-peer, exchanges routes, and the plugin responds with a route announcement. The initial hypothesis (documented in `plan/investigate-check-ci-slowness.md`) was that a failed first TCP connect triggered a 5s reconnect backoff (`DefaultReconnectMin`). This was wrong.

The actual root cause was a race condition in the Python SDK's TLS muxing layer that caused the engine to wait 5s for a response the plugin would never send. After fixing the SDK and adding TCP_NODELAY to plugin connections, the test runs in 1.4s (4.4x faster). All 202 plugin tests pass.

## Decisions

- **Instrumented before theorizing** -- added debug timing logs and Prometheus metrics at every stage of the startup and connection path before proposing fixes. This disproved the initial hypothesis in 30 minutes instead of spending hours on the wrong fix. The timing logs revealed the 5s gap was between `OnMessageSent` call (41.451s) and its timeout (46.456s), not between connection attempts.

- **Kept timing infrastructure permanent** over removing it after investigation. Debug timing logs (`peerLogger().Debug("timing: ...")`) and 6 Prometheus histograms (`ze_plugin_startup_seconds`, `ze_peer_dial_seconds`, etc.) remain in the codebase. The next time someone reports slowness, the data is already there -- no re-instrumentation needed.

- **Fixed the SDK race condition in `read_line()`** over restructuring the engine's startup ordering. The engine's ordering (`SignalAPIReady` before sending ready OK) is architecturally correct -- it ensures the reactor doesn't wait for plugins that are already ready. The bug is that the Python SDK doesn't handle the consequence: callbacks arriving before the ready OK on the shared TLS connection.

- **Added TCP_NODELAY on plugin TLS connections** over leaving default Nagle behavior. Plugin IPC uses small request-response messages (typically under 1KB). Nagle's algorithm delays these by up to 200ms waiting for batching that never comes. While Nagle was not the 5s root cause, it added unnecessary latency to every plugin RPC. The BGP session code already set TCP_NODELAY (`session_connection.go:230`); plugin connections were the gap.

- **Chose `setTCPNoDelay` helper with `NetConn()` unwrap** over raw syscall access. TLS connections wrap the underlying TCP socket. Go's `tls.Conn` exposes `NetConn()` (since Go 1.18) to access the raw `*net.TCPConn`. This is cleaner than `SyscallConn().Control()` with `setsockopt`.

## Consequences

- **All plugin tests benefit**, not just check.ci. Any external plugin test that triggers `OnMessageSent` during the startup window was vulnerable to this race. The 5s timeout was the default `ze.plugin.delivery.timeout`. Tests that happened to avoid the race (plugin enters `read_line()` before the first route send) were unaffected, which made the issue appear intermittent under load.

- **The `_pending_requests` queue in the Python SDK is now drained at runtime.** Previously it was only consumed by `_serve_one()` during the 5-stage startup protocol. Any callback that arrived during a `_call_engine()` call at runtime (not just startup) would have been silently lost. This fix closes that broader class of bugs.

- **TCP_NODELAY on plugin connections reduces latency for all plugin RPCs.** Event delivery, command dispatch, filter evaluation -- every RPC round-trip is faster. The improvement is most visible on localhost (where kernel buffering dominates) but also matters for remote plugins over LAN.

- **The `OnMessageSent` synchronous wait pattern remains unchanged.** The function at `events.go:697-702` still waits for all delivery results before returning. This is correct for the cache consumer pattern (where the caller needs to know if the cache consumed the event). The fix ensures the plugin responds promptly, making the wait short rather than eliminating it.

- **The timing infrastructure adds negligible overhead in production.** The debug logs are filtered by log level (default WARN). The Prometheus metrics use pre-allocated histograms with no per-observation allocation. The `time.Now()` calls add ~25ns each.

## Gotchas

- **The initial hypothesis was completely wrong.** "5s delay = reconnect backoff" was plausible and matched the timing, but the first connect succeeded in 670us on attempt 1. The lesson: **instrument first, hypothesize second**. Adding 8 timing log lines took 5 minutes and immediately ruled out the reconnect theory.

- **The race is timing-dependent and hard to reproduce manually.** When running ze directly (not through the test runner), the ready OK is sent and received before any route sends. The race requires the reactor to start peers, connect, and send a route within ~2ms of `SignalAPIReady()` -- which happens reliably in the test runner but not always in manual testing. This is why the test was "consistently slow" but not obviously broken.

- **`_pending_requests` vs `_pending_events` naming confusion.** The Python SDK has two queues: `_pending_requests` (RPCs from `_call_engine()` muxing) and `_pending_events` (events from a previous `deliver-batch`). `read_line()` checked `_pending_events` but not `_pending_requests`. The names suggest different concerns but both contain work that `read_line()` should process.

- **The `OnMessageSent` log showed the symptom, not the cause.** The log said "write failed: context deadline exceeded" which pointed at the delivery path. But the actual bug was in the Python SDK's `read_line()` not draining `_pending_requests`. Tracing the full call chain (events.go -> delivery.go -> rpc.go -> mux.go -> TLS -> Python SDK) was necessary to find the root cause.

- **TCP_NODELAY was a separate issue that compounded.** Even without the race condition, Nagle's algorithm added 40-200ms of latency to every plugin RPC on localhost. This was invisible because the 5s timeout dominated, but after fixing the race, it would have been the next bottleneck. Fixing both in the same change was the right call.

- **The test runner captures ze's stderr but only shows it on failure.** To see debug timing from a passing test, we had to use `ze_log=debug` in the parent environment and grep the runner's stderr for `clientOutput`. The `-s` (save) flag didn't work for the new cmd-based test format. This cost ~15 minutes of investigation overhead.

- **`SignalAPIReady()` before `SendResult()` is intentional.** The comment at startup.go:510-513 explains: "Move the barrier BEFORE the OK response below. This ensures all plugins in the tier have registered their commands and reached StageReady before any of them receive OK and start their runtime event loop." The engine deliberately releases the reactor before the plugin knows it's released. The SDK must handle this.

## Files

| File | Change |
|------|--------|
| `test/scripts/ze_api.py` | Drain `_pending_requests` in `read_line()` -- root cause fix |
| `internal/component/plugin/ipc/tls.go` | `setTCPNoDelay()` helper, called on accepted connections |
| `pkg/plugin/sdk/sdk.go` | TCP_NODELAY on dialed TLS connections |
| `internal/component/bgp/reactor/peer_run.go` | Debug timing logs + Prometheus metrics in run loop |
| `internal/component/bgp/reactor/reactor.go` | Debug timing logs + metrics for plugin/API waits |
| `internal/component/bgp/reactor/reactor_metrics.go` | 6 new Prometheus metrics for startup/connection timing |
| `internal/component/plugin/process/delivery.go` | Slow delivery warning log |
| `internal/component/bgp/server/events.go` | Slow OnMessageSent warning log |
| `docs/guide/monitoring.md` | Added Startup and Connection Timing section with 6 new metrics |
