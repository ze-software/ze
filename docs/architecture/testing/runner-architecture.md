# Test Runner Architecture

How `ze-test` discovers, schedules, executes, and reports functional tests.

This document covers the **execution architecture** (the engines that run tests
concurrently and render results) and the **web `.wb` test format**. For the
`.ci` and `.et` file formats see [`ci-format.md`](ci-format.md); for the suite
inventory and how to invoke suites see [`../../functional-tests.md`](../../functional-tests.md).

> **Status:** describes the system as it is **today**. A planned consolidation
> is tracked separately — see [Direction](#direction-planned).

## Layers

Every functional suite moves through the same five stages, regardless of which
engine schedules it:

| Stage | What happens |
|-------|--------------|
| Discover | Walk a `test/<suite>/` directory, parse each test file into a record |
| Select | Filter by `-a`/`--all`, ids, `--pattern`, or `--start` |
| Schedule | Run selected tests, bounded by a concurrency limit |
| Execute | Per-test work: spawn `ze`, drive a browser, or drive the editor model |
| Report | Live status, per-test PASS/FAIL line, summary, timing baseline, failure index |

Discovery, selection, and reporting are shared. **Scheduling and execution are
where the system currently forks into three separate engines.**

## The three execution engines

`ze-test` subcommands register themselves in a dispatch table.
<!-- source: internal/test/cli/register.go -- subcommands registry, Register() -->
Each subcommand routes to one of three engines that independently re-implement
the same goroutine-pool + semaphore + live-display orchestration.

| Engine | Source | Scheduling | Concurrency | Drives Display / timing / failure-groups |
|--------|--------|------------|-------------|------------------------------------------|
| `Runner.Run` | `internal/test/runner/runner.go` | own goroutine pool + semaphore | `RunOptions.Parallel` (0 → all selected) | yes |
| `ParallelRunner[T]` | `internal/test/runner/parallel.go` | own goroutine pool + semaphore (generic over test type `T`) | fixed `DefaultParallelConcurrent = 20` | yes |
<!-- source: internal/test/runner/runner.go -- Runner.Run, the .ci scheduling + process-orchestration engine -->
<!-- source: internal/test/runner/parallel.go -- ParallelRunner[T] generic scheduler, DefaultParallelConcurrent -->

### Which suites use which engine

| Engine | Suites | Per-test execution |
|--------|--------|--------------------|
| `Runner.Run` | encode, plugin, reload, chaos (via `runEncodingOrAPI`); and the `.ci` suites ui, managed, policy, firewall, l2tp, l2tp-wire, install, static, traffic, flow-export, vpp (via `runCISubcommand`) | `Runner.runTest` — spawns `ze`/`ze-peer`, applies expectations |
| `ParallelRunner[T]` | bgp decode, bgp parse, editor (`.et`), web (`.wb`) | per-suite `Run` func: `DecodingTest`, `ParsingTest`, `EditorTest`, `zeTestWebTest` |

<!-- source: internal/test/cli/cmd_bgp.go -- runEncodingOrAPI selects Runner for encode/plugin/reload/chaos -->
<!-- source: internal/test/cli/ci_runner.go -- runCISubcommand wires the non-bgp .ci suites to Runner -->
<!-- source: internal/test/runner/decoding.go -- DecodingTest scheduled via NewParallelRunner -->
<!-- source: internal/test/runner/parsing.go -- ParsingTest scheduled via NewParallelRunner -->
<!-- source: internal/test/cli/cmd_editor.go -- EditorTest scheduled via NewParallelRunner -->
<!-- source: internal/test/cli/cmd_web.go -- zeTestWebTest scheduled via NewParallelRunner, per-test daemon + session -->

### Shared reporting

Both engines render through the same components:

- **Live status and per-test result lines** — `Display` (header, in-place status, `TestFinished`, `Summary`). <!-- source: internal/test/runner/display.go -- Display, TestFinished, Summary -->
- **Color/glyph formatting** — `Colors`, TTY-aware via `slogutil.UseColor`. <!-- source: internal/test/runner/color.go -- Colors, NewColors -->
- **Timing baseline** — rolling per-test durations persisted under the suite label. Baseline updates are suppressed when the run is contended (host load > CPUs with concurrent test processes) to prevent slow-run pollution. <!-- source: internal/test/runner/timing.go -- Timings, Record, Save --> <!-- source: internal/test/runner/hostload.go -- HostLoad, Contended -->
- **Per-test timeout resolution** — the effective wall-clock budget is `explicit cmd timeout=` (or `option=timeout`) ▸ else baseline-derived `SuggestedTimeout` (`min(suite-default, max(5s, 5×avg))`) ▸ else the suite default. The resolved value is then widened by `ParallelTimeoutHeadroom` (×3) whenever the run executes tests concurrently (`concurrency > 1`): authored timeouts are measured against an uncontended run, so parallel execution — where tests share CPU and run slower — needs proportional headroom or budgets set near the uncontended runtime flake. Serial runs (`-p 1`, single-test debug) keep the tight authored value so real slowdowns surface quickly. <!-- source: internal/test/runner/runner_exec.go -- runTest/runOrchestrated timeout resolution, withParallelHeadroom --> <!-- source: internal/test/runner/parallel.go -- ParallelTimeoutHeadroom -->
- **Inner gates derive from that budget, never from a constant** — a fixed inner deadline is sized against one machine and lies at any other speed, so the gates a daemon enforces *inside* a test derive from the resolved per-test budget and then take the same parallel headroom (applied once, on top). Two do this today: the `await=stderr` fence (`0.8 ×` budget, floored, clamped back down to the budget) and the plugin-startup stall window handed over as `ze_plugin_stage_timeout` (same shape, floor `10s`). The stall window is **not** a budget for how long a startup stage may take — `StartupCoordinator.WaitForStageProgress` already waits on the *condition*, restarting its window every time any plugin completes a stage — it bounds only how long the whole tier may go with zero progress before it is declared wedged. Machine speed therefore enters through the test's own declared budget, which is where an author already expresses it: the ospfv3 netns tests declare `timeout=15s` for work that costs 1.6s natively, and `make ze-netns-qemu-test` runs `-p 1`, so the parallel headroom is a no-op there and could not compensate for emulation. The share is below 1 for ordering, not for speed: the watchdog must expire *before* the outer budget so the failure names the wedged plugin instead of reporting an opaque test timeout. <!-- source: internal/test/runner/plugin_stage_stall.go -- pluginStageStall, pluginStageStallEnv --> <!-- source: internal/test/runner/await_stderr.go -- defaultAwaitStderrTimeout --> <!-- source: internal/component/plugin/startup_coordinator.go -- WaitForStageProgress, noteProgressLocked --> <!-- source: internal/component/plugin/server/server.go -- defaultStageTimeout, ze.plugin.stage.timeout -->
- **External-probe test knob** — `ze.doctor`-style reachability checks probe real network destinations with multi-second timeouts; functional tests set `ze.test.doctor.probe-timeout` (the runner injects `250ms`) so probes to deliberately-unreachable fixtures fail fast instead of dominating wall-clock. The override only shortens a probe, never lengthens it, so production keeps its per-check defaults. <!-- source: internal/component/doctor/checks_reach.go -- reachProbeTimeout, doctorProbeTimeoutEnv --> <!-- source: internal/test/runner/runner_exec.go -- proc.Env probe-timeout injection -->
- **Failure routing** — under `ZE_VERIFY_MODE=1`, failed suites emit a compact `VERIFY FAILURE INDEX` with one `VERIFY FAILURE GROUP: {json}` token per group. Contended runs label the index header and attach host load context to each group. A `near_timeout` failure kind classifies tests that consumed >80% of their timeout without the context deadline firing. <!-- source: internal/test/runner/failure_group.go -- GroupFunctionalFailures, PrintFailureGroups --> <!-- source: internal/test/runner/parallel.go -- verifyModeEnabled, ZE_VERIFY_MODE --> <!-- source: internal/test/runner/hostload.go -- IsNearTimeout, FailTypeNearTimeout -->

### Engine unification

`Runner.Run` delegates scheduling to `ParallelRunner[*Record]`. The `.ci`-specific
concerns (Build, process orchestration, `PrintAllFailures`) stay in `Runner`;
scheduling, timing, failure-group output, and summary display are the single
`ParallelRunner` engine. <!-- source: internal/test/runner/runner.go -- Run -->

- **Status-update cadence:** configurable via `SetStatusInterval`. `.ci` sets 500ms;
  other consumers default to `StatusUpdateInterval` (200ms).
  <!-- source: internal/test/runner/parallel.go -- SetStatusInterval, StatusUpdateInterval -->
- **Failure detail:** `ParallelRunner` prints verify-mode groups; `.ci` adds
  `PrintAllFailures` via the `onReport` hook.
  <!-- source: internal/test/runner/parallel.go -- Run failure reporting -->
  <!-- source: internal/test/runner/runner.go -- Run onReport -->
- **Stress mode:** `Runner.RunWithCount` resets record state between iterations and
  suppresses per-iteration summary via `SetNoSummary`.
  <!-- source: internal/test/runner/runner.go -- RunWithCount -->

## Web test format (`.wb`)

Web tests live in `test/web/*.wb` and drive the HTMX web UI through a headless
browser. A `.wb` file is a line-oriented sequence of directives; blank lines and
`#` comments are ignored. Each directive is `directive=kind:key=value:key=value`.
<!-- source: internal/component/web/testing/parser.go -- ParseWBFile, parseWBKV, extractWBKind -->

Actions and expectations are recorded in source order and executed
interleaved, so assertions observe the state produced by the actions before them.
<!-- source: internal/component/web/testing/parser.go -- WBStep ordering preserves interleaving -->
<!-- source: internal/component/web/testing/runner.go -- runWBTestCase iterates tc.Steps -->

### Directives

| Directive | Purpose |
|-----------|---------|
| `option=` | Test-level settings (see below) |
| `action=` | A browser action |
| `expect=` | A browser-state assertion |

Unknown directives or kinds fail parsing immediately.
<!-- source: internal/component/web/testing/parser.go -- ParseWBFile default case errors on unknown directive -->

### Options

| Option | Effect |
|--------|--------|
| `option=timeout:value=<dur>` | Per-test timeout (default `30s`) |
| `option=skip:reason=<text>` | Skip the test; `reason` is surfaced in runner output |

<!-- source: internal/component/web/testing/parser.go -- parseWBOption: timeout, skip -->

### Actions

| Action | Keys | Effect |
|--------|------|--------|
| `open` | `path` | Navigate to `baseURL + path` |
| `click` | `id` or `text` | Click element by id or visible text |
| `fill` | (`id` or `text`) + `value` | Clear and type `value` |
| `hover` | `id` or `text` | Hover element |
| `press` | `key` (+ optional `id`/`text`) | Press a key, optionally focused on an element |
| `wait` | `ms` (or none) | Wait `ms` milliseconds, or for in-flight network to settle |
| `screenshot` | `file` | Save a screenshot |

<!-- source: internal/component/web/testing/runner.go -- executeAction: open/click/fill/hover/press/wait/screenshot -->

### Expectations

| Expectation | Keys | Passes when |
|-------------|------|-------------|
| `element` | `id` / `not-id` | element with that id is present / absent in the DOM |
| `element` | `text` / `not-text` | text is present / absent in the accessibility snapshot (case-insensitive) |
| `breadcrumb` | `contains` / `not-contains` (CSV) | each segment is present / absent |
| `url` | `contains` | the page snapshot contains the substring |
| `title` | `contains` | the page text contains the substring (case-insensitive) |

<!-- source: internal/component/web/testing/expect.go -- checkExpectation: element/breadcrumb/url/title; id/not-id/text/not-text/contains/not-contains -->

### Browser integration (agent-browser)

The web runner shells out to the external `agent-browser` CLI for every browser
operation; if it is not on `PATH`, the suite skips.
<!-- source: internal/test/cli/cmd_web.go -- exec.LookPath("agent-browser") skip when absent -->
<!-- source: internal/component/web/testing/runner.go -- Browser methods invoke agent-browser via runAgent -->

The web suite runs each `.wb` test in parallel (capped at 4) with full per-test
isolation: each test gets its own `ze` daemon (own port via `ReservePorts` + own
tmpdir config store) and its own `agent-browser` session (via `AGENT_BROWSER_SESSION`
env var), so `.wb` scenarios that mutate and `commit` config cannot corrupt each other.
<!-- source: internal/test/cli/cmd_web.go -- zeTestRunWebTest, per-test ReservePorts + MkdirTemp + session -->
<!-- source: internal/component/web/testing/runner.go -- NewBrowserWithSession, agentEnv sets AGENT_BROWSER_SESSION -->

## `.ci` and `.et` formats

These formats are documented in [`ci-format.md`](ci-format.md):

- **`.ci`** — config/process tests: tmpfs blocks, `cmd=` process orchestration,
  and `expect=` assertions (exit code, stdout/stderr, JSON, syslog, file, HTTP).
- **`.et`** — editor TUI tests: `input=` actions, `expect=` editor-state
  assertions, plus `session=`/`restart=`/`wait=` steps, executed in source order.

Both record an ordered `Steps` slice like `.wb`, and both emit per-step trace
output on failure (and on all tests under `-v`).
<!-- source: internal/component/cli/testing/parser.go -- TestCase.Steps, InputAction, Expectation -->
<!-- source: internal/component/cli/testing/runner.go -- runTestCase iterates tc.Steps -->

## Per-step trace output

All three runners record `trace.StepResult` slices during execution and emit
dual-format trace output via `internal/test/trace`:

- **Human:** colored `checkmark`/`cross` glyphs, one line per step, with kind, assert, and failure detail.
- **Machine:** `VERIFY STEP: {json}` token per step, matching the `VERIFY FAILURE GROUP` convention from `failure_group.go`.

Trace is emitted automatically on failure (default tier). Under `-v`, passing tests
also show their step trace. The `.ci` runner emits trace in failure reports when
`rec.StepTrace` is non-empty.

The trace package (`internal/test/trace`) is a leaf with no runner imports, avoiding
cycles between the three runner packages that all import it.
<!-- source: internal/test/trace/trace.go -- StepResult, PrintTrace -->
<!-- source: internal/test/runner/report.go -- step trace in failure reports -->
<!-- source: internal/test/cli/cmd_web.go -- web trace on fail + verbose -->
<!-- source: internal/test/cli/cmd_editor.go -- editor trace on fail + verbose -->

### History

This was a three-spec sequence, now complete:

1. `plan/learned/868-test-web-parallel.md` -- web migrated from bespoke loop to `ParallelRunner`, per-test daemon + session isolation.
2. `plan/learned/908-test-trace-mode.md` -- per-step trace output across all three runners.

The `verify-debugging-protocol` dependency (`plan/learned/843-verify-debugging-protocol.md`)
landed the failure-group and verify-mode foundation that trace mode builds on.
