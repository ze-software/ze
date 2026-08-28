# Verify and Debugging Workflow

Use this page when `./le verify current mode full` fails, when a test needs to be rerun narrowly, or when debug logging should be enabled without turning the whole run into noise.

<!-- source: ../main/internal/le/verify/run.go -- verify entry point -->
<!-- source: ../main/internal/le/verifylock/register.go -- verify lock -->
<!-- source: ../main/internal/test/runner/failure_group.go -- failure grouping -->
<!-- source: ../main/internal/test/trace/trace.go -- per-step trace output -->
<!-- source: ../main/docs/guide/debugging-tools.md -- debugging tools -->

`./le verify current mode full` is the normal pre-handoff gate. It is staged, locked, and designed to tell a developer where to rerun. The changed-only command is useful during development, but a finished change should pass the shared gate that would catch cross-package and functional regressions.

```bash
./le verify current mode changed
./le verify current mode full
```

## What verify does

<table>
<thead><tr><th>Stage</th><th>Purpose</th><th>Typical rerun</th></tr></thead>
<tbody>
<tr><td>Lint and architecture checks</td><td>Formatting, static analysis, generated docs, wiring, and project rules.</td><td><code>./le verify-lint run</code> or the printed validation target.</td></tr>
<tr><td>Unit and race checks</td><td>Package contracts, changed groups, and race-sensitive paths.</td><td><code>go test -race -run TestName ./path/...</code></td></tr>
<tr><td>Functional suites</td><td><code>.ci</code>, <code>.wb</code>, and <code>.et</code> behavior that an operator or browser can observe.</td><td><code>bin/ze-test &lt;suite&gt; NAME -v</code></td></tr>
<tr><td>Compatibility checks</td><td>ExaBGP and related protocol compatibility gates that belong in the local pass.</td><td>The command printed by the failure group.</td></tr>
</tbody>
</table>

The verify runner writes logs under `tmp/`, keeps a compact failure index, and prints grouped failures. The lock in `internal/le/verifylock/register.go` prevents two verify-class runs from corrupting shared temp state or making failures unreadable.

## Reading the failure

Start with the first failing group. It tells you the stage, summary, related files, and rerun command. If the failure came from a functional transcript, rerun exactly that test with `-v`. If it came from a Go package, rerun one test or one package before rerunning a group target.

```bash
bin/ze-test bgp plugin 42 -v
go test -race -run TestName ./internal/component/bgp/...
```

If the rerun prints a temporary directory, keep it only when you need the artifacts. If the test is Linux-only, go straight to QEMU rather than trying to make Darwin behave like Linux.

```bash
ZE_TEST_KEEP_TMP=1 bin/ze-test bgp plugin 42 -v
./le qemu run command 'bin/ze-test-linux-arm64 bgp plugin 79 -v' keep-alive
```

## Trace output

Functional runners emit per-step trace records. Human output shows the line and status. Machine output uses `VERIFY STEP` JSON so failure grouping can stay stable even when prose changes.

```text
  12 ✓ action open
  13 ✗ expect element -> expected element with text "Routes" not found
VERIFY STEP: {"file":"test/web/foo.wb","step":13,"kind":"expect","status":"fail"}
```

Trace output is available for `.ci`, `.wb`, and `.et`. Use it to locate the first broken transition, not the last assertion that happened to fail.

## Debug logging

Ze logging is controlled per subsystem through environment variables. Enable the narrow log that matches the failing surface.

<table>
<thead><tr><th>Surface</th><th>Example</th></tr></thead>
<tbody>
<tr><td>BGP peer behavior</td><td><code>ze.log.bgp.reactor.peer=debug bin/ze-test bgp plugin NAME -v</code></td></tr>
<tr><td>Plugin server behavior</td><td><code>ze.log.plugin.server=debug bin/ze-test bgp plugin NAME -v</code></td></tr>
<tr><td>Config parsing</td><td><code>ze.log.config=debug bin/ze-test bgp parse NAME -v</code></td></tr>
<tr><td>Linux diagnosis</td><td><code>./le qemu run command '...' keep-alive</code>, then inspect <code>ip</code>, <code>nft</code>, <code>dmesg</code>, and temp files.</td></tr>
</tbody>
</table>

Do not turn on every log by default. Broad logs can hide the one line that matters and can change timing in concurrent tests.

## Skips and flakes

A skip is not proof. `option=needs-linux` is the correct marker for functional behavior that requires Linux. `.wb` has an explicit skip option for external browser fixtures. Go tests should skip only when an optional capability is genuinely absent and another gate covers the required behavior.

Flakes should be fixed at the source. Replace timing guesses with readiness checks, process waits, observable browser state, or protocol synchronization. If the test only passes after a sleep grows, the runner is missing a signal.

## Debugging order

Use the failure index first, rerun the narrow command second, enable one debug log third, and escalate to QEMU or interop only when the contract needs that environment. After the fix, rerun the narrow command and then the gate that would have caught the regression.
