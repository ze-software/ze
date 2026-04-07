# Known Test Failures

Pre-existing failures that need fixing. Each entry includes failure output and root-cause hypothesis.
Sessions should attempt to fix entries here before logging new ones.

Remove entries once fixed.

## TestSSEMultiLineData (timeout)

**File:** `internal/chaos/web/sse_test.go:254`
**Failure:** Test times out at 600s in `ServeHTTP` at `sse.go:130`.
**Frequency:** Intermittent under race detector with full test suite.
**Hypothesis:** SSE broker hangs waiting for client to read. Likely a test cleanup issue where the HTTP response writer is not closed, causing ServeHTTP to block indefinitely.
**Not caused by:** config transaction protocol work (2026-04-06) -- no chaos/web files modified.

## TestBackpressureNoResumeAbove10Percent (flaky)

**File:** `internal/component/bgp/plugins/rs/worker_test.go:1026`
**Failure:** `require.Eventually` at line 1065: "low-water callback never fired after draining below 10%" (2s timeout)
**Frequency:** ~1 in 3 runs (confirmed with `-count=3`)
**Hypothesis:** After `close(blockCh)` unblocks remaining items and `waitForCount` confirms all 11 processed, the `onLowWater` callback depends on the worker pool's backpressure check loop timing. Under race detector / CPU load, the check may not fire within the 2s `require.Eventually` window. The 2s timeout or the check interval may need increasing, or the test needs to synchronize on the actual low-water trigger rather than polling.
**Not caused by:** protocol genericity refactor (2026-04-06) -- confirmed by running test in isolation.

## TestInProcessSpeed (race)

**File:** `internal/chaos/inprocess/runner_test.go:218`
**Failure:** DATA RACE between `bufio.(*Reader).Read` in `session_read.go:104` (goroutine running `readAndProcessMessage`) and `runtime.slicecopy` in another goroutine accessing the same `bufio.Reader` buffer.
**Frequency:** Intermittent under race detector.
**Hypothesis:** Two goroutines share a `bufio.Reader` on the same `net.Conn` without synchronization. The session read goroutine and the peer run goroutine both access the reader concurrently. Likely needs a mutex around session reader access or separate readers per goroutine.
**Not caused by:** bridge/dispatch refactoring (2026-04-06) -- race is in reactor session read path, not plugin dispatch.

## Mass functional test failures: encode + plugin + reload (~200 tests) -- FIXED 2026-04-07

**Suites:** `make ze-encode-test`, `make ze-plugin-test`, `make ze-reload-test`
**Files:** all of `test/encode/*.ci`, most of `test/plugin/*.ci`, all of `test/reload/*.ci`
**Symptom:** All tests timed out with `received messages: 0`. ze daemon exited within ~70ms after `"bgp peers started"`, with every plugin logging `"rpc runtime: read failed" error="mux conn closed"`.
**Root cause:** Regression introduced in commit `58564e0a` (refactor: generic callback registry). That commit added bridge transport negotiation: the SDK closes its end of the mux conn after switching to bridge mode (so engine->plugin callbacks flow via `DirectBridge` exclusively). However, the server-side dispatcher `handleSingleProcessCommandsRPC` in `internal/component/plugin/server/dispatch.go` was NOT updated to account for this. It kept reading from the mux conn, saw the planned close as `ErrMuxConnClosed`, logged `"rpc runtime: read failed"`, and returned, triggering `cleanupProcess` via defer and decrementing `s.wg`. When ALL internal plugins simultaneously transitioned to bridge mode during startup, they all decremented `s.wg` at once, `apiServer.Wait()` returned, `waitLoop` exited via `doneCh`, and the daemon shut down before peer connections could be established.
**Fix:** Added `HasBridge()` accessor to `internal/component/plugin/ipc/rpc.go:PluginConn`. In `handleSingleProcessCommandsRPC`, when `conn.HasBridge()` is true, skip the mux read loop entirely and block on `s.ctx.Done()` instead. Plugin->engine RPCs continue to flow via `DirectBridge` (wired by `wireBridgeDispatch`), so no runtime traffic is lost. Cleanup still runs via the existing `defer s.cleanupProcess(proc)` when the server context is canceled during shutdown.
**Verification (2026-04-07):**
- `bin/ze-test bgp encode --all` -> 47/48 passing (the one remaining failure is an unrelated `shared-join` mvpn prefix parsing issue in test Q, not the startup regression).
- `bin/ze-test bgp plugin --all` -> 217/218 passing (the one remaining failure is a loopback alias setup issue, not the startup regression).
- `bin/ze-test bgp reload --all` -> 12/19 passing. The remaining 7 reload failures are SIGHUP/transaction-protocol specific (not the startup regression). Left for a separate follow-up investigation.
- Before the fix, the same suites were at 0/96, 0/~218, 0/19 respectively.
- Manual reproduction: `ZE_LOG=debug bin/ze tmp/test-config.conf` now runs until SIGTERM without any `mux conn closed` messages, dials the peer, and stays alive until shutdown.
- Unit tests pass: `go test -race ./internal/component/plugin/server/... ./internal/component/plugin/ipc/...`.

## Remaining reload failures: SIGHUP + tx-protocol (7 tests)

**Tests:**
- `1 persist-across-restart` -- expects 2 messages, receives 3 (extra unexpected message after SIGHUP reload)
- `3 reload-add-peer`
- `4 reload-add-route`
- `7 reload-rapid-sighup`
- `A reload-restart-peer`
- `G test-tx-protocol-exclusion`
- `I test-tx-protocol-sighup`
**Pattern:** Tests involving SIGHUP reload or the config transaction protocol. After the startup-regression fix, the `bgp peers started` log appears, the peer connects, initial exchange works, but SIGHUP reload either fails to push the expected state or pushes too many state updates.
**Hypothesis:** Related to the config-transaction-protocol commits (`559b3e9b`, `7c5f8d89`) changing the reload path. Deferred to a separate investigation session.
