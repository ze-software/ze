# Test Runner Architecture

How `ze-test` discovers, schedules, executes, and reports functional tests.

This document covers the **execution architecture** (the scheduler and suite
wrappers) and the **web `.wb` test format**. For the
`.ci` and `.et` file formats see [`ci-format.md`](ci-format.md); for the suite
inventory and how to invoke suites see [`../../functional-tests.md`](../../functional-tests.md).


## Layers

Every functional suite moves through the same five stages:

| Stage | What happens |
|-------|--------------|
| Discover | Walk a `test/<suite>/` directory, parse each test file into a record |
| Select | Filter by `-a`/`--all`, ids, `--pattern`, or `--start` |
| Schedule | Run selected tests, bounded by a concurrency limit |
| Execute | Per-test work: spawn `ze`, drive a browser, or drive the editor model |
| Report | Live status, per-test PASS/FAIL line, summary, timing baseline, failure index |

Discovery and execution vary by suite. Most suites use the shared scheduler.
The `.ci` `Runner` wrapper adds build and process orchestration.

## Scheduler and suite wrappers

`ze-test` subcommands register themselves in a dispatch table.
<!-- source: internal/test/cli/register.go -- init, registerCIRoot -->
<!-- source: internal/test/cli/dispatch.go -- registerRoot, registerCIRoot -->
`NewParallelRunner` constructs `parallelRunner` for direct use or use through
the `.ci` `Runner` wrapper. The predecessor ExaBGP suite uses the bespoke
scheduler in `runExaBGPSelected`.

| Layer | Source | Role | Concurrency |
|-------|--------|------|-------------|
| `parallelRunner[T]` | `internal/test/runner/parallel.go` | Schedules tests, updates status, records timing, and reports failures | `SetConcurrency`, with `DefaultParallelConcurrent = 20` when zero |
| `.ci` `Runner` | `internal/test/runner/runner.go` | Builds binaries and uses `runTest` for process orchestration, then delegates scheduling | `RunOptions.Parallel`, with zero expanded to all selected tests |
<!-- source: internal/test/runner/parallel.go -- parallelRunner.Run, DefaultParallelConcurrent -->
<!-- source: internal/test/runner/runner.go -- Runner.Run -->

### Suite scheduler examples

| Wrapper or scheduler | Example suites | Per-test execution |
|---------|--------|--------------------|
| `.ci` `Runner` | encode, plugin, reload, chaos, ui, managed, policy, firewall, l2tp, l2tp-wire, install, static, traffic, flow-export, vpp | `Runner.runTest` spawns `ze` or `ze-peer` and applies expectations |
| Direct `parallelRunner[T]` use | bgp decode, bgp parse, editor (`.et`), web (`.wb`) | Suite-specific functions run `DecodingTest`, `ParsingTest`, `EditorTest`, or `zeTestWebTest` |
| Bespoke scheduler | ExaBGP predecessor suite | `runExaBGPSelected` schedules parallel and serial batches |

<!-- source: internal/test/cli/cmd_bgp.go -- runEncodingOrAPI selects Runner for encode/plugin/reload/chaos -->
<!-- source: internal/test/cli/ci_runner.go -- runCISubcommand wires the non-bgp .ci suites to Runner -->
<!-- source: internal/test/runner/decoding.go -- DecodingTest scheduled via NewParallelRunner -->
<!-- source: internal/test/runner/parsing.go -- ParsingTest scheduled via NewParallelRunner -->
<!-- source: internal/test/cli/cmd_editor.go -- EditorTest scheduled via NewParallelRunner -->
<!-- source: internal/test/cli/cmd_web.go -- zeTestWebTest scheduled via NewParallelRunner, per-test daemon + session -->
<!-- source: internal/test/cli/cmd_exabgp.go -- runExaBGPSelected -->

### Shared reporting

The scheduler and suite wrappers render through the same components:

- **Live status and per-test result lines** — `Display` (header, in-place status, `TestFinished`, `Summary`). <!-- source: internal/test/runner/display.go -- Display, TestFinished, Summary -->
- **Color/glyph formatting** — `Colors`, TTY-aware via `slogutil.UseColor`. <!-- source: internal/test/runner/color.go -- Colors, NewColors -->
- **Timing baseline** — rolling per-test durations persisted under the suite label. Baseline updates are suppressed when the run is contended (host load > CPUs with concurrent test processes) to prevent slow-run pollution. <!-- source: internal/test/runner/timing.go -- Timings, Record, Save --> <!-- source: internal/test/runner/hostload.go -- HostLoad, snapshotHostLoad --> <!-- source: internal/core/hostload/hostload.go -- Load.Contended -->
- **Per-test timeout resolution** — the effective wall-clock budget is `explicit cmd timeout=` (or `option=timeout`) ▸ else baseline-derived `SuggestedTimeout` (`min(suite-default, max(5s, 5×avg))`) ▸ else the suite default. The resolved value is then widened by `ParallelTimeoutHeadroom` (×3) whenever the run executes tests concurrently (`concurrency > 1`): authored timeouts are measured against an uncontended run, so parallel execution — where tests share CPU and run slower — needs proportional headroom or budgets set near the uncontended runtime flake. Serial runs (`-p 1`, single-test debug) keep the tight authored value so real slowdowns surface quickly. <!-- source: internal/test/runner/runner_exec.go -- runTest/runOrchestrated timeout resolution, withParallelHeadroom --> <!-- source: internal/test/runner/parallel.go -- ParallelTimeoutHeadroom -->
- **Inner gates derive from that budget, never from a constant** — a fixed inner deadline is sized against one machine and lies at any other speed, so the gates a daemon enforces *inside* a test derive from the resolved per-test budget and then take the same parallel headroom (applied once, on top). Two do this today: the `await=stderr` fence (`0.8 ×` budget, floored, clamped back down to the budget) and the plugin-startup stall window handed over as `ze_plugin_stage_timeout` (same shape, floor `10s`). The stall window is **not** a budget for how long a startup stage may take — `StartupCoordinator.WaitForStageProgress` already waits on the *condition*, restarting its window every time any plugin completes a stage — it bounds only how long the whole tier may go with zero progress before it is declared wedged. Machine speed therefore enters through the test's own declared budget, which is where an author already expresses it: the ospfv3 netns tests declare `timeout=15s` for work that costs 1.6s natively, and `./le qemu netns-test` runs `-p 1`, so the parallel headroom is a no-op there and could not compensate for emulation. The share is below 1 for ordering, not for speed: the watchdog must expire *before* the outer budget so the failure names the wedged plugin instead of reporting an opaque test timeout. <!-- source: internal/test/runner/plugin_stage_stall.go -- pluginStageStall, pluginStageStallEnv --> <!-- source: internal/test/runner/await_stderr.go -- defaultAwaitStderrTimeout --> <!-- source: internal/component/plugin/startup_coordinator.go -- WaitForStageProgress, noteProgressLocked --> <!-- source: internal/component/plugin/server/server.go -- defaultStageTimeout, ze.plugin.stage.timeout -->
- **External-probe test knob** — `ze.doctor`-style reachability checks probe real network destinations with multi-second timeouts; functional tests set `ze.test.doctor.probe-timeout` (the runner injects `250ms`) so probes to deliberately-unreachable fixtures fail fast instead of dominating wall-clock. The override only shortens a probe, never lengthens it, so production keeps its per-check defaults. <!-- source: internal/component/doctor/checks_reach.go -- reachProbeTimeout, doctorProbeTimeoutEnv --> <!-- source: internal/test/runner/runner_exec.go -- proc.Env probe-timeout injection -->
- **Failure routing** — under `ZE_VERIFY_MODE=1`, failed suites emit a compact `VERIFY FAILURE INDEX` with one `VERIFY FAILURE GROUP: {json}` token per group. Contended runs label the index header and attach host load context to each group. A `near_timeout` failure kind classifies tests that consumed >80% of their timeout without the context deadline firing. <!-- source: internal/test/runner/failure_group.go -- groupFunctionalFailures, printFailureGroups --> <!-- source: internal/test/runner/parallel.go -- verifyModeEnabled, ZE_VERIFY_MODE --> <!-- source: internal/test/runner/hostload.go -- isNearTimeout, FailTypeNearTimeout -->

### `.ci` Runner wrapper

`Runner.Run` delegates scheduling to `parallelRunner[*Record]`. The `.ci`
wrapper keeps builds, process orchestration, and `PrintAllFailures`.
`parallelRunner` owns scheduling, timing, failure-group output, and summary
display.
<!-- source: internal/test/runner/runner.go -- Runner.Run -->

- **Status-update cadence:** `.ci` calls `setStatusInterval` with 500 ms.
  Other consumers use `StatusUpdateInterval` (200 ms).
  <!-- source: internal/test/runner/parallel.go -- setStatusInterval, StatusUpdateInterval -->
- **Failure detail:** `parallelRunner` prints verify-mode groups. `.ci` adds
  `PrintAllFailures` through the `onReport` hook.
  <!-- source: internal/test/runner/parallel.go -- parallelRunner.Run -->
  <!-- source: internal/test/runner/runner.go -- Runner.Run -->
- **Stress mode:** `Runner.RunWithCount` resets record state between iterations
  and calls `setNoSummary` to suppress each iteration summary.
  <!-- source: internal/test/runner/runner.go -- Runner.RunWithCount -->

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
| `option=timeout:value=<dur>` | The test's wall-clock budget, default 30s. `runWBTestCase` checks it before each step, so a test is bounded at its budget plus one step. Each step is bounded by `agentTimeout` (30s per `agent-browser` command) or `expectDeadline` (15s per retried assertion or `wait-until`). A value the runner cannot read is a parse error, never a fall back to the default. A budget nothing can enforce must not leave the file looking bounded |
| `option=skip:reason=<text>` | Skip the test; `reason` is surfaced in runner output |
| `option=viewport:width=<n>:height=<n>` | Resize the viewport before the first navigation |
| `option=locale:lang=<tag>` | Set `Accept-Language` for the session |
| `option=auth:user=<u>:password=<p>:role=<r>` | Repeatable. Seeds the user and starts the server with authentication instead of `--insecure-web` |
| `option=server:kind=web\|lg\|lg-no-engine\|chaos` | Which server the harness starts. Default `web`. An unknown kind fails parsing, because starting the default one and asserting against it is a pass that proves nothing |
| `option=env:var=<name>:value=<v>` | Repeatable. Sets one environment variable on the SERVER process |

<!-- source: internal/component/web/testing/parser.go -- parseWBOption: timeout, skip, viewport, locale, auth, server, env -->
<!-- source: internal/test/cli/cmd_web.go -- zeTestRunWebTest runs RunWBFileWithSession with no per-test deadline -->

Ze serves three htmx interfaces from three programs, and a browser proof of one
of them has to start that one.

| `kind` | What the harness starts | Scheme | Ports |
|--------|-------------------------|--------|-------|
| `web` | `ze start --web <port> --web-only` | `https` | 1 |
| `lg` | `ze-test peer --mode sink`, then `ze -` with a looking-glass listener and one peer dialling that sink | `http` | 2 |
| `lg-no-engine` | `ze-test lg`, the real looking glass with a dispatcher that always fails | `http` | 1 |
| `chaos` | `ze-chaos --in-process --web :<port>` | `http` | 1 |

The looking glass gets a peer because its pages read `show bgp`: without
one, every assertion would run against an empty table. `ze-chaos` is a second
compile of `cmd/ze` under different tags, so it is built only when a selected
test asks for it, beside the `ze` binary the run is using.

`lg-no-engine` exists because no configuration reaches the engine-unavailable
state: the looking glass dispatches in process, so a daemon with no BGP still
answers an empty peer list. `ze-test lg` builds the REAL server through
`lg.NewLGServer` and injects one failing dispatcher, which is the only part that
is not production code. The two ends of the `lg` daemon peer carry different
loopback addresses (127.0.0.1 and 127.0.0.2), so a rule comparing a route
against the peer address has two values to compare.
<!-- source: internal/test/cli/cmd_web.go -- zeTestStartLGServer, zeTestStartLGNoEngineServer, zeTestStartChaosServer, zeTestResolveWebBinaries -->
<!-- source: internal/test/cli/cmd_lg.go -- cmdLG -->
<!-- source: internal/le/functional/binaries.go -- alternate chaos build -->

### Actions

| Action | Keys | Effect |
|--------|------|--------|
| `open` | `path` | Navigate to `baseURL + path` |
| `click` | `id` or `text` | Click element by id or visible text |
| `fill` | (`id` or `text`) + `value` | Clear and type `value` |
| `hover` | `id` or `text` | Hover element |
| `press` | `key` (+ optional `id`/`text`) | Press a key, optionally focused on an element |
| `wait` | `ms` (or none) | Wait `ms` milliseconds, or for in-flight network to settle. A wait longer than 30s touches the browser between sleeps: the `agent-browser` daemon reaps itself after 60s with no command and takes the page with it, so a pure sleep of a minute returned to a browser holding nothing |
| `wait-until` | `path` + `contains` | Re-open `path` until the page it serves contains the text. Stops polling at `expectDeadline`. **Leaves the browser on `path`** |
| `login` | `user` + `password` | Drive the login form and submit |
| `back` | none | The browser's own back button. The only way to prove what a pushed URL does when the operator returns to it: htmx 2 restores its own history cache, htmx 4 keeps none and the browser navigates for real |
| `forward` | none | The browser's own forward button. htmx 4 traverses the Navigation API, which holds entries on both sides of the current one, and the entry AHEAD is reached by a second traversal that `back` cannot make |
| `screenshot` | `file` | Save a screenshot |

<!-- source: internal/component/web/testing/runner.go -- executeAction: open/click/fill/hover/press/wait/wait-until/login/back/forward/screenshot -->

`wait:ms=` is a blind wait and asserts on elapsed time, which is what
`ai/rules/completion.md` bans: it passes on a quiet host and fails on a busy one.
Reach for the wait that names the state instead.

| The step waits for | Use |
|--------------------|-----|
| A render the browser drives (an htmx swap, a JS update) | Nothing. A positive `expect=` polls the DOM until it lands |
| A mutation to reach the SERVER | `action=wait-until:path=<readback>:contains=<text>` |
| In-flight requests to drain, with no assertion after | `action=wait` |

A positive `expect=` cannot stand in for `wait-until`: it re-reads the DOM the page
already holds, so a readback the browser never fetched again reports the state that
was true before the action. `action=wait` cannot either, because its idle predicate
is true both before a request begins and after it ends.

Two properties of `wait-until` bite if you assume otherwise.

**It leaves the browser on the path it polled.** Every expectation after it judges
the readback, not the page the test was driving. That is what makes
`commit-flow.wb` work: its closing `not-contains` must read `/config/diff`, because
the page discard redirects to never held the pending change anyway. When you want
the previous page back, `action=open` there yourself.

**`expectDeadline` stops the POLLING, and is not a wall-clock cap.** The deadline is
checked between rounds, so the round in flight when it expires runs to completion,
and each round issues browser commands capped only by `agentTimeout`. What bounds a
`.wb` test above that is its own `option=timeout`, checked between steps (see
Options). It was inert until 2026-08-23, and a web test that stopped making progress
hung the suite rather than failing one test.
<!-- source: internal/component/web/testing/runner.go -- waitUntil, waitLoad, inflightIdleExpr -->
<!-- source: internal/component/web/testing/expect.go -- retryPositive, retryCommand, expectDeadline -->

### Expectations

| Expectation | Keys | Passes when |
|-------------|------|-------------|
| `element` | `id` / `not-id` | element with that id is present / absent in the DOM |
| `element` | `text` / `not-text` | text is present / absent in the accessibility snapshot (case-insensitive) |
| `html` | `contains` / `not-contains` | the page BODY contains / does not contain the substring |
| `head` | `contains` / `not-contains` | the page HEAD contains / does not contain the substring. `html` cannot answer for it: `get html <selector>` returns one element's inner HTML, and it reads the body. The head is where a page states which assets it loads, and each page loads what its own markup needs (`internal/le/webassets.Write`), so what a head does NOT carry is a property of the page |
| `breadcrumb` | `contains` / `not-contains` (CSV) | each segment is present / absent |
| `url` | `contains` / `not-contains` | the ADDRESS BAR contains / does not contain the substring. It read the accessibility snapshot until 2026-08-15, which never carries the address, so the assertion could only ever fail |
| `title` | `contains` | the page text contains the substring (case-insensitive) |

A POSITIVE expectation polls until it holds or `expectDeadline` expires, so it is a
state wait as well as an assertion. A NEGATIVE one judges absence on one answer,
because retrying an absence can only ever turn a real failure into a pass. So an
absence proves something only when the step before it made the state current.
<!-- source: internal/component/web/testing/expect.go -- checkExpectation: element/breadcrumb/html/head/url/title; retryPositive vs retryFetch -->
<!-- source: internal/component/web/testing/runner.go -- getHTML reads body, getHeadHTML reads head -->

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
<!-- source: internal/component/web/testing/runner.go -- newBrowserWithSession, agentEnv sets AGENT_BROWSER_SESSION -->

## `.ci` and `.et` formats

These formats are documented in [`ci-format.md`](ci-format.md):

- **`.ci`** — config/process tests: tmpfs blocks, `cmd=` process orchestration,
  and `expect=` assertions (exit code, stdout/stderr, JSON, syslog, file, HTTP).
- **`.et`** — editor TUI tests: `input=` actions, `expect=` editor-state
  assertions, plus `session=`/`restart=`/`wait=` steps, executed in source order.

Both record an ordered `Steps` slice like `.wb`, and both emit per-step trace
output on failure (and on all tests under `-v`).
<!-- source: internal/component/cli/testing/parser.go -- TestCase.Steps, InputAction, Expectation -->
<!-- source: internal/component/cli/testing/runner.go -- runTestCaseIn iterates tc.Steps; runTestCase owns the temp directory and delegates -->

## Selecting one test

<!-- source: internal/test/runner/selection.go -- indexRecordSelector -->

The runner's one-based ordinal is an internal DISPLAY position over a sorted
fixture population. Adding or renaming an earlier fixture silently renumbers
every later row, so a numeric id kept past the turn names a different test than
it did when it was written. On one occasion a concurrent session added `.ci`
files and moved id 373 from `resolve-ping` to `remove-private-as-replace-peer`
while an id-driven script reported green for tests it never ran.

A positional selector matches a record's Nick, Name, or CIFile EXACTLY
(`indexRecordSelector`), so passing a NAME positionally is as stable as
`--pattern` and, unlike a substring pattern, cannot widen.
`internal/le/qemu/netns_linux.go` selects all four of its subsets by name for
that reason, and its `assert_named` guard refuses a subset still carrying a
numeric selector: a nick had already drifted there, with firewall `"17"`
resolving to `command-owner-firewall-root.ci` rather than to any `017-*.ci`.

## Scratch roots

The functional runner writes its per-run and per-test working directories
(configs, sockets, daemon pid and ready files) under
`sessionpath.DefaultScratchRoot()` / `EnsureScratchRoot(baseDir)` when a session
is active, rather than an unowned `$TMPDIR/ze-functional-*`. Off-session it
still uses the system temp directory.

<!-- source: internal/test/sessionpath -- DefaultScratchRoot, EnsureScratchRoot -->
<!-- source: internal/test/runner/runner.go -- run and per-test working directories -->

## Timing baseline and auto-timeout

`ze-test` saves per-test timing to `tmp/test-timings.json` as a rolling EMA with
alpha 0.3. After three samples the baseline drives two things:

- **Auto-timeout.** The per-test timeout is `min(global, max(5s, 5 x baseline avg))`.
  A test that normally takes 500ms gets a 5s timeout instead of the default 15s,
  so a hang is caught in seconds rather than minutes. An explicit `.ci`
  `timeout=` overrides it.
- **Slow detection.** A test exceeding 2x its baseline is flagged in the summary.

Baseline updates are suppressed during a contended run, so a slow run does not
pollute the EMA.

## Contended run verdicts

<!-- source: internal/test/runner/hostload.go -- HostLoad, isNearTimeout -->
<!-- source: internal/core/hostload/hostload.go -- Load.Contended -->

On a loaded machine the failure index is headed
`VERIFY FAILURE INDEX (CONTENDED RUN)` with host load details. That means load
exceeded the CPU count with concurrent `ze-test` or `go test` processes.

- A `near_timeout` kind says the test consumed over 80% of its timeout without
  the context deadline firing. That is CPU starvation, not a bug. Rerun it on a
  quiet machine.
- The `host-load` field in the failure group JSON carries the load average, the
  CPU count, and the concurrent process counts at run start.

The project rejects retry-on-failure masking: a contended verdict is a
classification, never an automatic retry.

## Reproducing a load-dependent flake

Some failures surface only under the scheduling and GC pressure of the whole
functional run, with many concurrent `ze` daemons on every core. Rerunning the
single suite never triggers them, looping the whole run is impractical, and the
verify aggregator truncates the crashing daemon's goroutine stack to about two
lines, so the crash site is usually lost.

`./le stress-repro run suite <suite>` recreates that pressure cheaply: CPU and GC
burner processes oversubscribe every core while many concurrent copies of one
suite loop, and it captures the FIRST failure's complete, untruncated output. It
sets `GOTRACEBACK=all` so a panic dumps every goroutine, reuses the isolated
binary set `internal/le/functional` prepared during the loaded window, and writes
the capture to `tmp/stress-repro/<slug>-<ts>.log`. Exit 0 means reproduced, 1 not
reproduced, 2 a setup error.

```
./le stress-repro run suite rsvpte iterations 80
./le stress-repro run suite rsvpte race
./le stress-repro run suite bgp burners 32 parallel 8
./le stress-repro run suite "bgp plugin" test 97 any-failure
```

The suite selector and `test` selector are both split on whitespace, so a
sub-suite and a multi-token selector reach `ze-test` exactly as typed by hand.

By default only a CRASH signature (a panic, a `DATA RACE`, or a runtime error)
counts as a reproduction, and everything else is discarded down to the last 500
bytes. An assertion flake exits non-zero with no crash signature, so
`any-failure` is what keeps its evidence.

A no-build reproduction tests the isolated binary set it was given. After
changing daemon source, run the owning `./le functional <suite>` action once
(`internal/le/functional.Prepare` rebuilds the pair) before trusting a verdict:
otherwise a fixed bug still reproduces against the stale binary.

## Per-step trace output

The `.ci`, editor, and web execution paths record `trace.StepResult` slices.
They emit two trace formats through `internal/test/trace`:

- **Human:** colored `checkmark`/`cross` glyphs, one line per step, with kind, assert, and failure detail.
- **Machine:** `VERIFY STEP: {json}` token per step, matching the `VERIFY FAILURE GROUP` convention from `failure_group.go`.

Trace is emitted automatically on failure (default tier). Under `-v`, passing tests
also show their step trace. The `.ci` runner emits trace in failure reports when
`rec.StepTrace` is non-empty.

The trace package (`internal/test/trace`) is a leaf with no runner imports.
This structure prevents import cycles between trace producers.
<!-- source: internal/test/trace/trace.go -- StepResult, PrintTrace -->
<!-- source: internal/test/runner/report.go -- step trace in failure reports -->
<!-- source: internal/test/cli/cmd_web.go -- web trace on fail + verbose -->
<!-- source: internal/test/cli/cmd_editor.go -- editor trace on fail + verbose -->

### History

This sequence is complete:

1. Web migrated from a bespoke loop to construction with `NewParallelRunner`, per-test daemon + session isolation.
2. Per-step trace output across all execution paths.

The `verify-debugging-protocol` dependency landed the failure-group and
verify-mode foundation that trace mode builds on.
