# Learned: functional test flakiness under host load

Spec: `plan/spec-test-flake-under-load.md`

## Context

Four consecutive `make ze-verify` cycles each failed on something different;
the last failed on three plugin tests that passed in isolation. The box was
running overlapping verify cycles, six subagent `go test` sessions, and an
interactive session.

## Decisions

1. **Root cause is wall-clock timeouts under CPU starvation.** All test
   timeouts use `context.WithTimeout` (pure wall-clock). Port collisions,
   build cache races, and tmp/ workspace collisions were ruled out.

2. **"unknown" failure classification was a near-miss timeout.** ze-peer
   gave up waiting for messages from a CPU-starved ze-daemon, but the
   context deadline hadn't fired. Added `near_timeout` classification
   (elapsed > 80% of timeout + non-specific failure type).

3. **Loaded runs pollute the timing baseline.** Passing-but-slow tests
   inflate the EMA (alpha=0.3), loosening future `SuggestedTimeout` and
   suppressing `IsSlow` detection. Solution: suppress `timings.Record`
   and `timings.Save` when `HostLoad.Contended()` is true.

4. **Host load context recorded in failure groups.** `FailureGroup.HostLoad`
   carries load average, CPU count, and concurrent process counts. The
   verify summary labels contended runs explicitly.

## Consequences

- On contended machines, failures now carry load context and the verify
  index header says "CONTENDED RUN" with process counts.
- Timing baselines are protected from slow-run pollution.
- The `near_timeout` failure kind replaces ambiguous `unknown` for
  CPU-starvation near-misses.

## Gotchas

- The verify lock only serializes `make ze-verify*` invocations. Subagent
  `go test` sessions run concurrently and are the dominant load source.
  The lock gap is by design (blocking manual test runs would break
  debugging workflows).
- `HostLoad.Contended()` requires both high load AND concurrent processes.
  High load alone (e.g., compilation) without concurrent test processes
  does not trigger contended classification.
- `pgrep` may not be available on all platforms; the code returns 0 on
  failure, so contended detection is best-effort.

## Files

None recorded.
