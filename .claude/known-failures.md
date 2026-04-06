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
