# 495 -- ETXTBSY flaky test in parallel runner

## Context

The managed test `init-meta-keys` (and potentially any test using tmpfs shell scripts) failed intermittently with `fork/exec ./test-init-meta-keys.sh: text file busy`. The test runner executes up to 20 tests in parallel, each writing scripts to tmpfs and immediately executing them. The failure appeared random and was not reproducible on retry.

## Decisions

- Chose retry loop (3 attempts, 10ms sleep) at the single `proc.Start()` call site that executes tmpfs-written scripts, over alternatives like fsync after write or serializing script execution.
- Followed Go stdlib precedent: `os/exec/lp_linux_test.go`, `cmd/go/internal/base/base.go`, and `cmd/go/internal/test/test.go` all retry on ETXTBSY for the same reason.
- Did not add retry to the other two `proc.Start()` calls in `runner_exec.go` (lines 184, 256) because those execute compiled Go binaries (`r.testPath`, `r.zePath`), not tmpfs-written scripts.

## Consequences

- Eliminates the ETXTBSY flaky test failure in parallel test runs.
- The retry is invisible: 10ms delay only triggers during the rare race, zero overhead otherwise.
- Any future tmpfs script execution through the same `runOrchestrated()` path is automatically protected.

## Gotchas

- The root cause is Go issue #22315: `fork()` in one goroutine inherits write-open fds from another goroutine's `os.WriteFile`, causing `ETXTBSY` on the subsequent `execve`. This is a kernel-level race that Go cannot fully prevent with `O_CLOEXEC` because the fd exists between fork and exec.
- The failing test name is misleading. Any of the 22+ managed tests using tmpfs scripts can hit this. The test name in the failure report is whichever test lost the timing lottery.
- Go's `cmd/go` uses 100ms with exponential backoff (conservative). 10ms is sufficient because the race resolves as soon as the forked child reaches its own `execve`, which is microseconds away.

## Files

- `internal/test/runner/runner_exec.go` -- ETXTBSY retry at `proc.Start()`
