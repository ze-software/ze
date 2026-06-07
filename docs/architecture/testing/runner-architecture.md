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
<!-- source: cmd/ze/ze_test_register.go -- subcommands registry, zeTestAdd() -->
Each subcommand routes to one of three engines that independently re-implement
the same goroutine-pool + semaphore + live-display orchestration.

| Engine | Source | Scheduling | Concurrency | Drives Display / timing / failure-groups |
|--------|--------|------------|-------------|------------------------------------------|
| `Runner.Run` | `internal/test/runner/runner.go` | own goroutine pool + semaphore | `RunOptions.Parallel` (0 → all selected) | yes |
| `ParallelRunner[T]` | `internal/test/runner/parallel.go` | own goroutine pool + semaphore (generic over test type `T`) | fixed `DefaultParallelConcurrent = 20` | yes |
| web loop | `cmd/ze/ze_test_web.go` | sequential `for` loop | 1 (sequential) | no (bespoke pass/fail counting) |

<!-- source: internal/test/runner/runner.go -- Runner.Run, the .ci scheduling + process-orchestration engine -->
<!-- source: internal/test/runner/parallel.go -- ParallelRunner[T] generic scheduler, DefaultParallelConcurrent -->
<!-- source: cmd/ze/ze_test_web.go -- bespoke sequential web test loop -->

### Which suites use which engine

| Engine | Suites | Per-test execution |
|--------|--------|--------------------|
| `Runner.Run` | encode, plugin, reload, chaos (via `runEncodingOrAPI`); and the `.ci` suites ui, managed, policy, firewall, l2tp, l2tp-wire, install, static, traffic, flow-export, vpp (via `runCISubcommand`) | `Runner.runTest` — spawns `ze`/`ze-peer`, applies expectations |
| `ParallelRunner[T]` | bgp decode, bgp parse, editor (`.et`) | per-suite `Run` func: `DecodingTest`, `ParsingTest`, `EditorTest` |
| web loop | web (`.wb`) | `RunWBFile` — drives a headless browser |

<!-- source: cmd/ze/ze_test_bgp.go -- runEncodingOrAPI selects Runner for encode/plugin/reload/chaos -->
<!-- source: cmd/ze/ze_test_ci_runner.go -- runCISubcommand wires the non-bgp .ci suites to Runner -->
<!-- source: internal/test/runner/decoding.go -- DecodingTest scheduled via NewParallelRunner -->
<!-- source: internal/test/runner/parsing.go -- ParsingTest scheduled via NewParallelRunner -->
<!-- source: cmd/ze/ze_test_editor.go -- EditorTest scheduled via NewParallelRunner -->
<!-- source: cmd/ze/ze_test_web.go -- RunWBFile invoked in a sequential loop -->

### Shared reporting

All three engines render through the same components:

- **Live status and per-test result lines** — `Display` (header, in-place status, `TestFinished`, `Summary`). <!-- source: internal/test/runner/display.go -- Display, TestFinished, Summary -->
- **Color/glyph formatting** — `Colors`, TTY-aware via `slogutil.UseColor`. <!-- source: internal/test/runner/color.go -- Colors, NewColors -->
- **Timing baseline** — rolling per-test durations persisted under the suite label. <!-- source: internal/test/runner/timing.go -- Timings, Record, Save -->
- **Failure routing** — under `ZE_VERIFY_MODE=1`, failed suites emit a compact `VERIFY FAILURE INDEX` with one `VERIFY FAILURE GROUP: {json}` token per group. <!-- source: internal/test/runner/failure_group.go -- GroupFunctionalFailures, PrintFailureGroups --> <!-- source: internal/test/runner/parallel.go -- verifyModeEnabled, ZE_VERIFY_MODE -->

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
<!-- source: cmd/ze/ze_test_web.go -- exec.LookPath("agent-browser") skip when absent -->
<!-- source: internal/component/web/testing/runner.go -- Browser methods invoke agent-browser via runAgent -->

Today the web suite runs against **one shared `ze` web server** (a single
`baseURL`) and **one global browser session** (`agent-browser close --all`
between tests), which is why it executes sequentially: `.wb` scenarios that mutate
and `commit` config would otherwise corrupt each other on a shared daemon, and a
single browser shares cookies/refs across tests.
<!-- source: cmd/ze/ze_test_web.go -- startTestWebServer, single shared listenAddr/baseURL -->
<!-- source: internal/component/web/testing/runner.go -- closeBrowser / runAgentWithHTTPSIgnore("close","--all") -->

`agent-browser` itself supports isolated concurrent sessions via `--session <name>`
(or the `AGENT_BROWSER_SESSION` env var) — each is a separate browser with its own
cookies, tabs, and refs. The current integration does not use this; it relies on
the default session and global close. <!-- source: .claude/rules/agent-browser.md -- agent-browser CLI usage -->

## `.ci` and `.et` formats

These formats are documented in [`ci-format.md`](ci-format.md):

- **`.ci`** — config/process tests: tmpfs blocks, `cmd=` process orchestration,
  and `expect=` assertions (exit code, stdout/stderr, JSON, syslog, file, HTTP).
- **`.et`** — editor TUI tests: `input=` actions, `expect=` editor-state
  assertions, plus `session=`/`restart=`/`wait=` steps, executed in source order.

Both record an ordered `Steps` slice like `.wb`, and both currently report only a
single PASS/FAIL per file. <!-- source: internal/component/cli/testing/parser.go -- TestCase.Steps, InputAction, Expectation --> <!-- source: internal/component/cli/testing/runner.go -- runTestCase iterates tc.Steps -->

## Direction (planned)

The three engines and the sequential web loop are slated to consolidate onto one
central scheduler, with parallel web execution and per-assertion trace output.
This is **not yet implemented**; it is tracked as a three-spec sequence:

1. `plan/spec-test-runner-unify.md` — one central `ParallelRunner`; `.ci` delegates scheduling; configurable per-suite concurrency cap.
2. `plan/spec-test-web-parallel.md` — per-test `ze` daemon + `agent-browser --session`; web runs through the unified runner.
3. `plan/spec-test-trace-mode.md` — per-assertion `✓`/`✗` trace plus a `VERIFY STEP` machine token, implemented once in the unified runner.

Implementation is sequenced after `plan/spec-verify-debugging-protocol.md`, which
is concurrently editing `runner.go`, `parallel.go`, `display.go`, and `report.go`.
