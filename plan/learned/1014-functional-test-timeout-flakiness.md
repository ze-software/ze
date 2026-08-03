# 1014 - Functional-test timeout flakiness (fix at the source, not per-test caps)

## Context

`make ze-verify` flaked: across three prior runs, three *different* functional
tests timed out (run 1: a bgp `plugin` test + an rsvpte test; run 3: ui
`doctor-update-archive`). Each passed on isolated rerun, so the failures read as
random load. They were not random.

## Root cause (two measurable patterns, both "test runs near its wall-clock cap")

1. **doctor tests do real network probes with multi-second timeouts, run
   sequentially.** `runChecks` (`internal/component/doctor/doctor.go,152,154`)
   calls `checkClockSkew` (dials `pool.ntp.org`), `checkUpdateCheckURL`, and
   `checkArchiveDestinations` (each `httpHead(..., 5*time.Second)`) one after
   another. Against deliberately-unreachable fixtures (TEST-NET `198.51.100.99`)
   every probe waits out its full timeout: `doctor-update-archive` measured
   **10.13s** of pure wall-clock (two 5s HEADs). All ~25 doctor tests carried a
   multi-second floor. 10-14s inside a 30s budget tips over under load.

2. **The plugin suite's explicit `timeout=N` caps were set ≈ the uncontended
   runtime, with zero load headroom -- and explicit caps bypass the adaptive
   timeout machinery.** `runTest`/`runOrchestrated`
   (`internal/test/runner/runner_exec.go`) resolve the budget as: explicit cmd
   `timeout=` (or `option=timeout`) OVERRIDES the baseline-derived
   `SuggestedTimeout`. Recorded `max-ms` (the *uncontended* worst case, since the
   baseline is suppressed when contended) sat at 65-100% of the cap across ~30
   plugin tests (`custom-flowspec` 1.00, `rpki-decorator-merge` 0.97,
   `-register` 0.99). Any added load tips dozens over. "bgp plugin 2/432" was a
   lottery among them.

## Fixes applied (both can only ever help a passing test)

1. **doctor probes fail fast in tests.** New `reachProbeTimeout(def)`
   (`checks_reach.go`) caps every reach-probe timeout by a `Private` env knob
   `ze.test.doctor.probe-timeout`; the runner injects `250ms`
   (`runner_exec.go`, next to the existing `ze_plugin_stage_timeout` precedent).
   The override only ever *shortens* a probe, so production keeps its 5s/3s
   defaults. `doctor-update-archive` 10.13s -> 0.62s; all 21 doctor tests
   5-14s -> 1.2-2.2s, still reporting destinations unreachable.

2. **Parallel runs get timeout headroom.** `ParallelTimeoutHeadroom` (x3,
   `parallel.go`) applied via `withParallelHeadroom` (`runner_exec_util.go`) to
   the resolved per-test budget whenever `concurrency > 1`. A 25s cap under
   `-p 8` becomes 75s. Serial runs (`-p 1`, single-test debug) keep the tight
   authored value so real slowdowns still surface.

Validation: `make ze-functional-test` green 3 consecutive runs, 0 timeouts.

## Reusable lesson

Load-induced timeout flakiness is not "raise this one timeout." It is two source
problems: (a) functional tests doing real multi-second network waits, and (b)
authored timeouts measured against an uncontended run with no parallel-load
headroom. Fix (a) by making the wait fast in tests (a cap that only shortens,
never lengthens, so production is untouched); fix (b) systemically by scaling the
budget with concurrency, not by hand-tuning each `.ci` cap (which rots and is
whack-a-mole). Per-test `timeout=` values bypass the runner's adaptive
`SuggestedTimeout`, so they are exactly the ones that need the headroom multiplier.
Related: [[1013-verify-gate-hardening]].

## Files

None recorded.
