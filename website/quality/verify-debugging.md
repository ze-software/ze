# Verify and Debugging Workflow

When a test fails, its failure report should identify the next command to run. This page explains how to read those reports and enable logging for the part of Ze under investigation.

<!-- source: ../main/internal/le/verify/engine/run.go -- verify entry point -->
<!-- source: ../main/internal/le/verify/lock/register.go -- verify lock -->
<!-- source: ../main/internal/test/runner/failure_group.go -- failure grouping -->
<!-- source: ../main/internal/test/trace/trace.go -- per-step trace output -->
<!-- source: ../main/docs/guide/debugging-tools.md -- debugging tools -->

`./le verify current` is a release check, also run by GitHub CI on pushed commits. It runs checks in stages, prevents overlapping verification runs, and reports commands for reproducing failures. During development, a change is checked with the tests for the affected packages and behavior; a full verification run is scheduled by the maintainer.

```bash
./le verify current fast
./le verify current
```

## What verify does

<table>
<thead><tr><th>Stage</th><th>Purpose</th><th>Typical rerun</th></tr></thead>
<tbody>
<tr><td>Lint and architecture checks</td><td>Formatting, code analysis, generated docs, component registration, and project rules.</td><td><code>./le go lint run</code> or the printed check command.</td></tr>
<tr><td>Unit and race checks</td><td>Package behavior, affected test groups, and concurrent memory access.</td><td><code>go test -race -run TestName ./path/...</code></td></tr>
<tr><td>Functional suites</td><td><code>.ci</code>, <code>.wb</code>, and <code>.et</code> behavior that an operator or browser can observe.</td><td><code>./le test &lt;suite&gt; NAME -v</code></td></tr>
<tr><td>Compatibility checks</td><td>ExaBGP and related protocol compatibility checks included in this verification run.</td><td>The command printed by the failure group.</td></tr>
</tbody>
</table>

The verify runner writes logs under `tmp/`, keeps an index of failures, and groups related failures in its report. The job scheduler in `internal/le/job/job.go` (`./le job run`) prevents resource-intensive verification runs from overlapping and interfering with shared temporary files or their reports.

## Reading the failure

Start with the first failing group. It tells you the stage, summary, related files, and rerun command. If the failure came from a functional transcript, rerun exactly that test with `-v`. If it came from a Go package, rerun one test or one package before rerunning a group target.

```bash
./le test bgp plugin 42 -v
go test -race -run TestName ./internal/component/bgp/...
```

If the rerun prints a temporary directory, keep it only when you need the artifacts. If the test is Linux-only, go straight to QEMU rather than trying to make Darwin behave like Linux.

```bash
ZE_TEST_KEEP_TMP=1 ./le test bgp plugin 42 -v
./le test qemu run command 'le test bgp plugin 79 -v' keep-alive
```

## Trace output

Functional runners emit per-step trace records. Human output shows the line and status. Machine output uses `VERIFY STEP` JSON so failure grouping can stay stable even when prose changes.

```text
  12 ✓ action open
  13 ✗ expect element -> expected element with text "Routes" not found
VERIFY STEP: {"file":"test/web/foo.wb","step":13,"kind":"expect","status":"fail"}
```

Trace output is available for `.ci`, `.wb`, and `.et`. The first broken transition usually explains later assertion failures, so it is the useful place to begin.

## Debug logging

Ze logging is controlled per subsystem through environment variables. Enable the log for the subsystem involved in the failure.

<table>
<thead><tr><th>Area</th><th>Example</th></tr></thead>
<tbody>
<tr><td>BGP peer behavior</td><td><code>ze.log.bgp.reactor.peer=debug ./le test bgp plugin NAME -v</code></td></tr>
<tr><td>Plugin server behavior</td><td><code>ze.log.plugin.server=debug ./le test bgp plugin NAME -v</code></td></tr>
<tr><td>Config parsing</td><td><code>ze.log.config=debug ./le test bgp parse NAME -v</code></td></tr>
<tr><td>Linux diagnosis</td><td><code>./le test qemu run command '...' keep-alive</code>, then inspect <code>ip</code>, <code>nft</code>, <code>dmesg</code>, and temp files.</td></tr>
</tbody>
</table>

Do not turn on every log by default. Broad logs can hide the one line that matters and can change timing in concurrent tests.

## Skipped and intermittent tests

A skipped test supplies no result. `option=needs-linux` marks functional behavior that requires Linux. `.wb` has an explicit skip option for external browser fixtures. Go tests should skip only when an optional capability is absent and another check covers the required behavior.

Intermittent failures need their cause fixed. Replace timing guesses with readiness checks, process waits, observable browser state, or protocol synchronization. If increasing a delay makes the test pass, the runner needs to check when the operation has finished.

## Debugging order

The failure index gives you a command for reproducing the problem. If its output is insufficient, enable logging for the affected subsystem. A kernel-dependent failure needs Linux, while a disagreement with another daemon needs the interoperability lab. After the fix, rerun the failing test and the other tests reached by the change; CI supplies the full verification.
