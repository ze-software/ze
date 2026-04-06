# Known Test Failures

Pre-existing failures that need fixing. Each entry includes failure output and root-cause hypothesis.
Sessions should attempt to fix entries here before logging new ones.

Remove entries once fixed.

## TestBackpressureNoResumeAbove10Percent (flaky)

**File:** `internal/component/bgp/plugins/rs/worker_test.go:1026`
**Failure:** `require.Eventually` at line 1065: "low-water callback never fired after draining below 10%" (2s timeout)
**Frequency:** ~1 in 3 runs (confirmed with `-count=3`)
**Hypothesis:** After `close(blockCh)` unblocks remaining items and `waitForCount` confirms all 11 processed, the `onLowWater` callback depends on the worker pool's backpressure check loop timing. Under race detector / CPU load, the check may not fire within the 2s `require.Eventually` window. The 2s timeout or the check interval may need increasing, or the test needs to synchronize on the actual low-water trigger rather than polling.
**Not caused by:** protocol genericity refactor (2026-04-06) -- confirmed by running test in isolation.
