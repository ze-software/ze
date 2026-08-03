# 1171 -- fixit-reject-fence-observability-deferred-external-plugin-signals

## Context

Two functional tests, `test/plugin/as112-external-refuses.ci` (an external plugin that
refuses to start) and `test/plugin/cos-external-warns.ci` (one that warns but keeps
running), held the daemon open with a blind `time.sleep(4.0)` so its relayed stderr could
carry the refuse/warn line the test asserts. The goal was to replace both blind holds with a
deterministic fence and ratchet `test/.ci-sleep-baseline` 132 -> 130, without losing the
byte-for-byte stderr proofs. The spec first pursued a "queryable state" design (a new
`Process.exited` marker polled by an in-daemon observer plugin via `system subsystem list`);
implementation proved that design impossible for the reject case and it was replaced.

## Decisions

- Chose a **test-runner `await=stderr:contains=<text>[:timeout=<dur>]`** primitive
  (`internal/test/runner/await_stderr.go`) over the queryable-state/observer design, because
  the observer approach is unbuildable for the reject case (see Gotchas) and await is uniform
  across reject and warn, adds no engine surface, and fences on the exact stderr line the test
  already asserts. Chosen by Thomas after the startup-barrier discovery.
- Reverted the `Process.exited` engine marker + its `system subsystem list` field + observer
  plugins as **unused** once await subsumed both tests -- no engine code ships.
- Reused the existing `syncWriter` (a mutex-guarded pattern-wait writer, originally for
  ze-peer's "listening on") via `newSyncWriterPattern`; tee the daemon's stderr through it and
  block on `WaitFor` before teardown. Chose this over a new daemon-side marker file or making
  startup-failure fatal (a production behavior change).
- Kept the primitive strictly additive: off unless `await=stderr` is present, so all ~1040
  existing plugin `.ci` tests are byte-for-byte unaffected (`teeDaemonStderr` returns the plain
  accumulator when the fence is nil).

## Consequences

- `await=stderr` is a general `.ci` fence: the rest of the reject-fence bucket
  (`trafficusage-external-refuses.ci`, `flowexport-external-refuses.ci`, still on blind sleeps)
  can adopt it to convert without any engine work.
- The determinism win is real but invisible to the `.ci`-sleep ratchet in isolation: the
  ratchet only counts `time.sleep(` in `test/**/*.ci`; `dispatch_until`/`WaitFor` internal
  sleeps are exempt. Converting these two is a genuine flake-reduction, not a ratchet trick.
- No operator-facing change: the plugins behave identically; only the test harness changed.

## Gotchas

- **An observer plugin cannot watch a plugin that fails startup.** as112 refuses at the top of
  `RunEngine` (`internal/plugins/as112/register.go`), which is a plugin-startup
  failure; `StartupCoordinator.PluginFailed` (`internal/component/plugin/startup_coordinator.go`)
  "aborts the ENTIRE startup process", so a co-located observer plugin dies at the barrier
  ("plugin 0 failed: startup incomplete"). The daemon does not exit on the failure either
  (`startup.go` logs and returns, keeps running; no later plugin phase runs). The whole
  reject-fence bucket shares this -- do not try to fence a refuse via an in-daemon observer.
  The warn case (cos) is different: it completes startup, so an observer WOULD work there --
  but await is simpler and uniform.
- **`system subsystem list` has two parallel handlers.** The dispatchable command
  `system subsystem list` reaches `handleSystemSubsystemList`
  (`internal/component/plugin/server/system.go`); `show system subsystem list` reaches a
  near-identical `handleShowSystemSubsystemList` (`internal/component/cmd/show/system.go`). The
  first (server) is the one a plugin/observer dispatch actually hits, proven by
  `test/plugin/subsystem-list.ci`.
- **The ci-sleep ratchet regex matches comments.** `time\.sleep\(` is counted anywhere in a
  `.ci`, including a comment that says "replaced the time.sleep(4.0)". Reword conversion
  comments (e.g. "4.0s sleep") or the count never drops.
- **A bare `.ci` foreground process cannot dispatch engine commands.** `API()` needs
  `ZE_PLUGIN_HUB_TOKEN`/engine FDs (`test/scripts/ze_api.py`), set only for
  daemon-spawned plugins (`process.go`); a `cmd=foreground:exec=python3 ...` has none.
  This is why the queryable-state design needed an observer plugin at all (and why await,
  which reads the daemon's own captured stderr, sidesteps it).
- **When adding an early-teardown wait, guard BOTH daemon-ready waits.** The foreground and
  background `waitReady(daemon.ready)` sites (`runner_exec.go`) are twins; skipping only one
  leaves a latent 5s stall on the other path (found in review).
- **Mutation-verify this fence by inverting the producer's guard, never by swapping
  plugins.** The line the test awaits is emitted behind `if !p.IsInternal()`
  (`pkg/plugin/sdk/sdk.go`, called at `internal/plugins/as112/register.go`).
  Inverting that condition is the authoritative mutation: it removes the refusal the
  fence exists to observe. Substituting a different plugin changes the fixture instead
  of the producer, so the test can stay green while proving nothing.
- **An await needle must be plugin-SPECIFIC and colon-free.** The bare phrase
  "refusing to start as an external plugin process" is shared by as112, traffic-usage
  and flow-export, so it matches the wrong daemon. The `.ci` key-value splitter also
  truncates a needle at the first `:`.

## Files

- `internal/test/runner/await_stderr.go` (new) -- `parseAwait`, `awaitStderrTimeout`,
  `teeDaemonStderr`, `awaitDaemonStderr`, default-timeout const
- `internal/test/runner/await_stderr_test.go` (new) -- parse + timeout-resolver unit tests
- `internal/test/runner/record.go` -- `AwaitStderr`/`AwaitStderrTimeout` fields
- `internal/test/runner/record_parse.go` -- `await=` dispatch
- `internal/test/runner/runner_exec.go` -- fence syncWriter, stderr tee, wait-before-teardown,
  guarded daemon.ready skip (both foreground and background)
- `internal/test/runner/runner_exec_util.go` -- `newSyncWriterPattern`
- `test/plugin/as112-external-refuses.ci`, `test/plugin/cos-external-warns.ci` -- converted to
  `await=stderr`, blind sleeps dropped
- `test/.ci-sleep-baseline` -- 132 -> 130
- `docs/architecture/testing/ci-format.md` -- `await=stderr` directive documented
- `internal/component/cmd/show/system.go` -- one-line stale cross-reference comment fix (hook-required)
