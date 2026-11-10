# Test Runner Architecture

How the functional runner (`le test <suite>`) discovers, schedules, executes, and reports functional tests.

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

Every harness command is the le command `le test <name>`. Each has its own
package at `internal/le/test/<name without hyphens>`, whose `register.go` calls
`leroot.Register`, `leroot.RegisterShape` and `leroot.RegisterForwarding` with
the answer `harnesstool.Answer` builds for a tool, or `harnesstool.SuiteAnswer`
for a `.ci` suite from its `CIRunnerConfig`. `RegisterForwarding` names the
program the words go to, `harnesstool.Helper` for a tool and
`harnesstool.Runner` for a suite, and that pass-through declaration is what
`le help test <name>` and `le | json` (`passes-words`) show. The command
forwards: every word after
`test <name>` reaches the handler in `internal/test/cli` or its packages, a
trailing help word included, and the handler's exit code is le's. No package
under `internal/test` registers a ze root.
<!-- source: internal/le/test/harnesstool/harnesstool.go -- Answer, SuiteAnswer -->
<!-- source: internal/le/test/peer/register.go -- one harness command's registration -->
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
| `.ci` `Runner` | encode, plugin, reload, chaos, ui, managed, policy, firewall, l2tp, l2tp-wire, install, static, traffic, flow-export, vpp | `Runner.runTest` spawns `ze` or `le test peer` and applies expectations |
| Direct `parallelRunner[T]` use | bgp decode, bgp parse, editor (`.et`), web (`.wb`) | Suite-specific functions run `DecodingTest`, `ParsingTest`, `EditorTest`, or `zeTestWebTest` |
| Bespoke scheduler | ExaBGP predecessor suite | `runExaBGPSelected` schedules parallel and serial batches |

<!-- source: internal/test/cli/cmd_bgp.go -- runEncodingOrAPI selects Runner for encode/plugin/reload/chaos -->
<!-- source: internal/test/cli/ci_runner.go -- runCISubcommand wires the non-bgp .ci suites to Runner -->
<!-- source: internal/test/runner/decoding.go -- DecodingTest scheduled via NewParallelRunner -->
<!-- source: internal/test/runner/parsing.go -- ParsingTest scheduled via NewParallelRunner -->
<!-- source: internal/test/cli/cmd_editor.go -- EditorTest scheduled via NewParallelRunner -->
<!-- source: internal/test/cli/cmd_web.go -- zeTestWebTest scheduled via NewParallelRunner, per-test daemon + session -->
<!-- source: internal/test/cli/cmd_exabgp.go -- runExaBGPSelected -->

### How a suite's child processes end

The `.ci` runner gives each successfully started child one owner and one Wait
worker. Every exit consumer reads the owner's cached result. Deferred cleanup
stops the process group and joins the worker before temporary files and port
leases are released, including early errors and panic unwinding. Named stop
removes an active handle, but retains that lifetime owner until cleanup.
The owner caches lifecycle observation errors separately from the process's
exit result. Each retained owner's deferred `finish` joins cleanup and fails the
case on a lifecycle error, preserving any earlier diagnostic. Expected exit
codes, intentional named stops and self-validation cannot waive that failure.
<!-- source: internal/test/runner/process_owner.go -- reap, Wait, lifecycleError, finish -->
On Linux and Darwin, the owner observes exit with `waitid(WNOWAIT)`, attempts to
kill remaining group members while the leader still reserves its PID, and retires
numeric signaling before the sole `Cmd.Wait` reaps it. A final group-signal error
other than `ESRCH` is cached as a lifecycle failure; numeric signaling is still
retired and the child is still reaped. Darwin is the one exception: XNU's
`killpg1` skips zombie members and answers `EPERM` when it signalled none, so a
group left holding only the exited leader answers `EPERM` where Linux answers
success. On that `EPERM` the owner lists the group through `kern.proc.pgrp` and
accepts it only when every member is a zombie or already exiting (`P_WEXIT`).
The exiting case is real: `proc_prepareexit` hides a dying process from
`proc_find` before it becomes a zombie, so `killpg1` skips it while the listing
still shows it running. A live member, or a listing that fails, keeps the error.
<!-- source: internal/test/runner/process_group_darwin.go -- exitedGroupSignalErr, darwinExitedGroupSignalErr, darwinMemberGone --> Deadline cancellation, lifetime expiry and
deferred cleanup share the retirement lock. A late cleanup cannot signal a
recycled group number. Darwin uses a local binding because x/sys/unix
does not wrap its waitid syscall. FreeBSD uses `wait4(WNOWAIT)`. OpenBSD uses
kqueue `NOTE_EXIT`; a process-table lookup distinguishes an already-exiting child
from a lost child when registration returns `ESRCH`. Neither adapter reaps the
child before final group cleanup. These cover the runner's existing Linux,
Darwin, FreeBSD and OpenBSD build contexts. The fallback for other operating
systems remains unsafe; those contexts also lack a loopback implementation in
this package. No build context is removed by the ownership repair.
<!-- source: internal/test/runner/process_owner.go -- ownedProcess, Wait, cleanup -->
<!-- source: internal/test/runner/process_exit_freebsd.go -- beforeReap -->
<!-- source: internal/test/runner/process_exit_openbsd.go -- beforeReap, waitOpenBSDExit, openBSDChild -->
<!-- source: internal/test/runner/runner_exec.go -- runTest, runOrchestrated -->

Both peer startup paths wait for the peer's `listening on` token within the
existing case context. The peer prints it only after its listener binds. Startup
consumes the case's remaining budget; it has no separate five-second limit and
does not restart the case deadline. A peer that exits before readiness fails
immediately after its owner joins final output copying, preserving its exit
status and both streams. Case cancellation and expiration retain their context
cause, and the consumer never starts without readiness. The existing empty
expectation diagnostic still explains which directives reach the peer.
<!-- source: internal/test/runner/peer_startup.go -- awaitPeerListening -->

Web servers use that same sole-reaper ownership through `StartServerProcess`.
They start on `plugin.StartChild`'s persistent spawner thread. Server stop joins
the owner before removing its directory, releasing ports, or returning the
scheduler lease. Cleanup failures fail the case while retaining browser steps
and earlier errors. The owner remains attached if the kernel cannot finish
reaping; a timeout alone never establishes cleanup.
`ServerProcess.Wait` and `Done` observe that same reaper; they never call a
second `Cmd.Wait`. `Close` immediately kills the owned group. `Stop` instead
sends the caller's signal to the leader under the retirement lock, waits for its
explicit grace, then kills the group and joins if needed. `StopGroup` uses the
same ownership and cleanup, sending the initial signal to the whole group.
The QEMU suite deadline uses it for TERM and a fifteen-second KILL grace.
Browser cleanup retains TERM and the
canonical ten-second `TeardownGraceTimeout`; normal hugepage VM shutdown retains
INT and its existing ten-second grace. Deliberate native PPP peer departure
retains TERM and three seconds, accepting xl2tpd's documented exit 1 only after
a nonreaping kernel probe excluded an already-exited parent before the stop.
Cancellation uses immediate `Close` instead of a graceful budget. Both cleanup
routes cache one verdict, retaining signal and lifecycle errors. A signal death
is intentional only after a nonreaping probe excludes a preexisting parent exit
and the owned signal request succeeds. `ESRCH` is benign cleanup, not evidence
that a signal was delivered. In particular, only a successful, pre-exit owned
KILL can normalize SIGKILL; the before-reap cleanup of remaining descendants
cannot normalize the parent's earlier death. Unknown or unsupported stop
observations remain lifecycle failures after owned cleanup; they do
not establish a live parent. An unexpected command exit, including self-SIGKILL,
remains the original exit error when there is no independent cleanup failure.
A stale compiler output therefore cannot turn a killed build into success.
Command callers await `Done` and consume `Close` once rather than joining the
same cached exit twice. A caller's
positive `Cmd.WaitDelay` supplies its pipe-drain bound; otherwise the existing
runner grace applies.
<!-- source: internal/test/runner/server_process.go -- StartServerProcess, ServerProcess.Close, ServerProcess.Stop, ServerProcess.Wait, ServerProcess.Done -->
<!-- source: internal/test/runner/runner_exec_util.go -- TeardownGraceTimeout -->
<!-- source: internal/test/cli/cmd_web.go -- zeTestRunWebTest, zeTestWebServer.stop -->

Web credential seeding checks the case context before and after synchronous
bcrypt and store operations. These checks refuse later startup after expiration;
they cannot interrupt a blocked storage lock or filesystem call. The case owner
stays attached during that work. A hard elapsed startup guarantee across those
calls is not yet established.
<!-- source: internal/test/cli/cmd_web.go -- zeTestSeedWebUsers -->

Editor discovery validates the complete `.et` snapshot and its timeout with
parallel headroom before execution. Resource options use `recordOptionArms` and
the same capability registry as `.ci`. The scheduler receives the declared
Linux requirement, exclusive group, skip reason, and parse error. A capability
reaches it only as that skip reason, which names the absent capability: the
host probe runs at parse time, so the scheduler has no capability list to read.
Execution uses that snapshot, so editing the file after discovery cannot bypass
its admission decision. No editor family privilege or exclusive group is implicit.
Failures while waiting for a group report the scheduler's error even though the
editor child has not started.
An unreadable or invalid capability source fails admission with its cause;
it does not become a capability skip or permission to start the editor.
<!-- source: internal/test/cli/cmd_editor.go -- discoverEditorTests, runEditorTests -->
<!-- source: internal/test/runner/record_parse_option.go -- ParseResourceOption -->

The web parser also accepts shared resource declarations through
`ParseResourceOption`. It retains admission metadata in the parsed case and
continues parsing after a skip, so a later malformed declaration is an error.
Web discovery reads each file once, before dependency checks or builds, and
execution uses that parsed snapshot. Selected parse failures take precedence over
a missing browser dependency; skipped cases need no browser or server build.
The scheduler receives the resource, skip and parse-failure fields. Its exclusive
lease remains held until the callback returns, including callback cleanup.
Admission failures are retained from the scheduler records even in quiet mode,
where the display's failure callback does not run.
<!-- source: internal/component/web/testing/parser.go -- WBTestCase, parseWBOption -->
<!-- source: internal/test/cli/cmd_web_discovery.go -- discoverWebCase, webDiscoveryErrors, webAdmissionRecord -->
<!-- source: internal/test/cli/cmd_web.go -- cmdWebMain, zeTestRunWebTest -->

The editor child executes authored content with `RunETContentIn` on its owning
goroutine. Caller cancellation reaches authored waits and validation waits;
model timers are joined and the store is closed before it returns, and close
errors fail the result. This cooperative API cannot interrupt a blocked file
read or storage lock: its process owner must enforce the hard deadline.
Process-wait failures include the owned child's PID in the reported error, so
the diagnostic identifies the process whose cleanup completed.
<!-- source: internal/component/cli/testing/runner.go -- RunETContentIn, runTestCaseInContext -->
<!-- source: internal/component/cli/testing/headless_wait.go -- waitEvent -->
<!-- source: internal/test/editorcase/parent.go -- Run -->

`le mock editor-case` is a hidden helper with no arguments. Its parent supplies
regular files at descriptor 3 (one JSON request), 4 (result), and 5 (goroutine
diagnostics), plus a control pipe at 6. The request contains `content`, an
absolute `deadline`, and an absolute parent-owned `directory`. Content is the
authored snapshot, not a path to reread. Request input is limited to 64 MiB;
missing fields, unknown fields and trailing JSON are refused. A byte or EOF on
the control pipe requests cancellation. The helper joins its diagnostic reader
after model/store closure and before writing its single result. A result alone
does not establish process reaping or directory cleanup. Native suite adoption
of the parent process owner is wired through `editorcase.Run`: one Wait worker
owns the process, cooperative EOF gets two seconds before the independent hard
group cancellation, and the two-second pipe wait is a separate limit. If reaping
still cannot finish, the foreground owner reports cleanup-incomplete and remains
attached, retaining resources and its scheduler slot. It never returns a false
reap. The parent rejects incomplete, contradictory or trailing results and failed
process exits, removes work and closes descriptors before publication, and
retains failure logs/diagnostics outside work.
Cleanup errors fail the overall verdict and preserve the failed-cleanup path;
they do not erase the child's assertion trace.
Result keys and step keys use their declared kebab-case JSON tags. Missing,
null-required, duplicate, unknown and contradictory fields are failures; a
successful exit alone cannot make an absent result pass.
<!-- source: internal/test/editorcase/worker.go -- Main, readRequest, run, watchControl -->
<!-- source: internal/le/mock/editorcase/register.go -- init -->
<!-- source: internal/test/editorcase/parent.go -- Run, wait, finish -->
<!-- source: internal/test/editorcase/result.go -- readResult -->

The hidden-mock guard reads both daemon config `run` statements in `.ci` files
and non-test Go launchers under `internal/test`. For a native launcher, it ties
an `os/exec` mock invocation, inherited `ExtraFiles`, and the process start to
the same constructor instance in source order. A later reassignment cannot
transfer an earlier process's launch evidence. A Go test fixture or command name in text
cannot supply that evidence. Both directions remain checked: every discovered
private worker is hidden, and every hidden mock has a discovered launcher.
Ordinary public mock servers do not become hidden merely because a fixture
starts them without inherited descriptors.
<!-- source: internal/le/le/root/hidden_spawned_test.go -- TestSpawnedMocksAreExactlyTheHiddenOnes, spawnedMocksAt, descriptorSpawnedMocks -->
<!-- source: internal/le/le/root/hidden.go -- RegisterHidden, Hidden -->

A suite starts a daemon and a peer, and each of those starts more: ze forks its
plugins, the ExaBGP mock forks the bridge's scripts. So a stop aimed at the pid
the runner holds reaps a fraction of the tree. Every fork therefore takes its
attributes and its cancellation from its process owner (the `.ci` owner above,
or `plugin.KillGroupOnCancel` for the other runners): the
child heads a process group, and a canceled context signals that whole group.

Three things end a process, and the suite owes all three:

| Route | What it covers |
|-------|----------------|
| A `defer` beside the start | Every return from the case, including a panic and a branch added later |
| Context cancellation | The per-case timeout, and the suite context when the operator interrupts the runner |
| `Pdeathsig` on Linux | The runner dying outright, which no code in the runner can cover |

An explicit stop inside a branch is an ORDERING barrier, not a lifetime: output
must not be read while its writer runs, and an exit status does not exist until
the process has one. A branch that reads it as the lifetime leaves the process
running when it returns by another route.

Cancellation is the route that failed silently. `exec.CommandContext` installs
its own cancel, which kills the direct child alone, so every grandchild was
reparented to init and the direct child stayed a zombie behind it: its pipes
never reached EOF, so the runner's `Wait` never ran. Taking the fork from
`plugin.KillGroupOnCancel` is what closes it.

A runner with no signal handler is the gap that hides best. Go ends a process
on an unhandled SIGINT where it stands, so no `defer` runs, and a child in a
process group of its own never saw the Ctrl-C that stopped its parent.
<!-- source: internal/test/cli/cmd_exabgp.go -- runOneExaBGPTest, cancelExaBGPOnSignal -->
<!-- source: internal/test/cli/cmd_exabgp_process.go -- startExaProcess, stopExaProcess -->
<!-- source: internal/component/plugin/sysproc.go -- KillGroupOnCancel, KillProcessGroup -->
<!-- test: internal/test/cli/cmd_exabgp_process_test.go TestCanceledContextReapsTheWholeGroup, TestStopExaProcessReapsTheWholeGroup -->

The release stage dispatcher uses `StartServerProcess` for each owned stage
group, sharing the stable spawner, identity-locked cancellation and before-reap
retirement. Root cancellation sends immediate SIGKILL to the still-owned group,
including TERM-ignoring members. Dispatch immediately defers checked `Close`,
not the graceful `Stop`. The sole direct-child Wait and output-reader join finish
before dispatch returns and its caller can release the admission lease.
Inherited output held by an escaped process has a separate two-second drain
bound; closing the pipe does not establish that the escaped process was killed
or reaped. Fixture-owned escaped sentinels are cleaned up separately.
<!-- source: internal/le/verify/dispatch/dispatch.go -- dispatch, runStage -->

A Docker lab container is the child `Pdeathsig` cannot reach: the daemon, not
the runner, is its parent, so killing the runner kills nothing it started. The
labs therefore cover the three routes differently. The `defer` beside the start
is `Suite.Run`'s teardown. Cancellation is a `signal.NotifyContext` root on
SIGINT and SIGTERM at each lab entry point, so a Ctrl-C runs that teardown. The
stand-in for `Pdeathsig` is the owner labels every lab resource carries.
The next lab start reports leftovers without removing them. The explicit
`./le test integration clean` command removes resources whose owner is provably
gone (`interop.md`, "Crash-safe teardown"). A launcher can automatically remove
only its attributed children's leftovers through `Docker.CleanupOwnedOrphans`.
<!-- source: internal/le/interoplab/lab.go -- Suite.Run -->
<!-- source: internal/le/interoplab/orphans.go -- Docker.CleanupOrphans, Docker.CleanupOwnedOrphans -->
<!-- source: internal/le/interoplab/run_identity.go -- Docker.ChildEnvironment -->

The docker client is the opposite case to a runner with no handler: le holds
the signal root, so the client must not see the Ctrl-C itself. Every docker
child heads a process group of its own (`systemProcessRunner.Run`), which takes
it out of the terminal's foreground group, so a Ctrl-C reaches le alone and the
teardown's `docker rm -f` is not killed by the same signal, or by a second one.
A canceled context kills that whole group, and a descendant that left the
group (a `setsid`) and still holds the child's output keeps `Run` waiting no
longer than `processWaitDelay`. A child handed a terminal as one of its streams
stays in le's group, because a background group that reads its terminal is
stopped by SIGTTIN.

The signal root holds SIGINT and SIGTERM only. A signal le does not root
(SIGHUP, SIGQUIT) ends le by Go's default action and leaves its docker clients
and Go builds running in their own groups. The containers they hold carry le's
pid in their owner labels, so `./le test integration clean` removes them once
le is gone.

So the context is the only stop such a child gets, and every docker command
and lab cross-compile a `./le` verb runs MUST take its context from that verb's
signal root. A step bounded from `context.Background()` (a kernel check, an
image build, a scenario) ignores the Ctrl-C and runs to its bound. The kernel
checks of `./le verify evidence release-candidate`, `./le setup docker kernel`,
the deployment proofs and the terminal demos, and the scenarios `./le feature
run` drives, each run under that root, and so does the daemon
cross-compile of the signal-rooted deployment proofs (`l2tp`, `vpp`,
`vpp-iface`). `gokrazy-l2tp-ppp` runs its whole appliance path under
the root: the host driver's compile, the runtime-kernel query, the child le
that runs gok, and the QEMU machine. The child le heads a process group of its
own, and the cancel kills the whole group, so a cancel during the build ends
the build, the compilers gok started included, and boots nothing; a cancel
after the boot kills the machine, so every console wait ends at once and the
deferred stops tear the lab and the peer down, with no context of their own.
A run the root stopped, in the build, the lab setup or the boot, answers the
stop as an error and prints no OK or FAIL verdict. `./le test qemu docker-lab`
takes its root before its first child: the docker-host image build runs under
it, and the linux ze compile runs through `Admission.RunContext`, whose wait for a slot ends with the root: a
root that captured SIGTERM must not leave a queue that never asks it holding
the verb. `./le setup docker kernel install` takes its root before its cache
query, so the cancel kills the ze-host compile behind that query. Its `sudo`
steps stay in le's group, where a Ctrl-C at the terminal reaches sudo itself.
The cancel sends a running step SIGTERM, which sudo relays to its command, and
starts no later step. An unfinished install reports only the successful command
prefix as completed and lists every remaining step, including reading the GRUB
menu and selecting its saved default. Cancellation before the first step says
that no step started, rather than claiming a partial change. Once a command could
have changed files, the refusal warns not to reboot until installation finishes:
module removal followed by a failed or interrupted copy is still unfinished.
It states that no automatic reboot was requested and prints the exact retry,
preserving the selected builder and confirmed release.
Before canceling the installer context, its signal worker unregisters SIGINT and
SIGTERM. A second interrupt therefore takes the default action even while a step
ignores TERM and the first stop is still waiting. Normal return unregisters and
joins the signal worker too. The terminal stays attached; this is not a bounded
privileged-process-tree reap, and sudo exiting does not prove its command ended.
<!-- source: internal/le/setup/docker/kernel/dockerkernel.go -- installSignalContext, cancelInstallOnSignal -->
<!-- test: internal/le/setup/docker/kernel/one_off_proof_test.go TestInstallSecondSignalEndsWaitingEntry -->
<!-- source: internal/le/test/integration/actions.go -- nativeRunner, signalContext -->
<!-- source: internal/le/verify/evidence/evidence.go -- dockerKernel, Runner.Run -->
<!-- source: internal/le/setup/docker/kernel/dockerkernel.go -- runDockerKernelCheck, runDockerKernelInstall, installCache, runInstallStep, installCommand -->
<!-- test: internal/le/setup/docker/kernel/dockerkernel_root_test.go TestDockerKernelInstallCacheBuildsUnderTheVerbRoot, TestDockerKernelInstallStepsStopWithTheRoot, TestDockerKernelCheckBuildsUnderTheVerbRoot -->
<!-- test: internal/le/setup/docker/kernel/install_progress_proof_test.go TestInstallUnstartedStepDoesNotClaimChanges -->
<!-- test: internal/le/setup/docker/kernel/install_plan_progress_proof_test.go TestInstallPlanProgress -->
<!-- source: internal/le/test/deployment/daemonkernel.go -- buildCheckedDaemon -->
<!-- source: internal/le/test/deployment/daemonbuild.go -- runDaemonBuild -->
<!-- source: internal/le/test/deployment/gokrazyimage.go -- gokrazyImage, RuntimeKernelCacheDir, hostTool, gokrazyBuildCommand -->
<!-- source: internal/le/test/deployment/gokrazyl2tp.go -- gokrazyL2TP.Run, gokrazyL2TP.observe, proveAppliance -->
<!-- source: internal/le/test/deployment/actions.go -- runGokrazyL2TP -->
<!-- source: internal/le/build/compile/build.go -- BuildHost -->
<!-- test: internal/le/test/deployment/gokrazyroot_test.go TestGokrazyImageBuildsUnderTheVerbRoot, TestGokrazyBuildCancelEndsTheGrandchildren, TestGokrazyStoppedRunPrintsNoVerdict -->
<!-- source: internal/le/test/qemu/dockerlab.go -- runDockerLab -->
<!-- source: internal/le/job/job.go -- Admission.RunContext -->
<!-- test: internal/le/test/qemu/dockerlab_root_test.go TestDockerLabBuildsUnderTheVerbRoot -->
<!-- test: internal/le/job/runcontext_test.go TestRunContextEndsTheWaitForASlot, TestRunContextStartsNoChildUnderADoneRoot -->
<!-- source: internal/le/site/terminaldemo/render.go -- Engine.checkKernel, Engine.demoGrants -->
<!-- source: internal/le/feature/recordrun.go -- runCatalogScenario -->
<!-- source: internal/le/interoplab/process.go -- systemProcessRunner.Run, sharesTerminal -->
<!-- source: internal/le/interoplab/orphans.go -- Docker.CleanupOrphans -->
<!-- test: internal/le/interoplab/orphans_docker_test.go TestKilledLabRunIsCleanedByNextStart -->
<!-- test: internal/le/interoplab/process_cancel_test.go TestCanceledCommandKillsItsGrandchildren, TestCanceledCommandDoesNotWaitOnAnEscapedPipeHolder -->

### Shared reporting

The scheduler and suite wrappers render through the same components:

- **Live status and per-test result lines** — `Display` (header, in-place status, `TestFinished`, `Summary`). <!-- source: internal/test/runner/display.go -- Display, TestFinished, Summary -->
- **Color/glyph formatting** — `Colors`, TTY-aware via `slogutil.UseColor`. <!-- source: internal/test/runner/color.go -- Colors, NewColors -->
- **Timing baseline** — rolling per-test durations persisted under the suite label. Baseline updates are suppressed when the run is contended (host load > CPUs with concurrent test processes) to prevent slow-run pollution. <!-- source: internal/test/runner/timing.go -- Timings, Record, Save --> <!-- source: internal/test/runner/hostload.go -- HostLoad, snapshotHostLoad --> <!-- source: internal/core/hostload/hostload.go -- Load.Contended -->
- **Per-test timeout resolution** — the effective wall-clock budget is `explicit cmd timeout=` (or `option=timeout`) ▸ else baseline-derived `SuggestedTimeout` (`min(suite-default, max(5s, 5×avg))`) ▸ else the suite default. The resolved value is then widened by `ParallelTimeoutHeadroom` (×3) whenever the run executes tests concurrently (`concurrency > 1`): authored timeouts are measured against an uncontended run, so parallel execution — where tests share CPU and run slower — needs proportional headroom or budgets set near the uncontended runtime flake. Serial runs (`-p 1`, single-test debug) keep the tight authored value so real slowdowns surface quickly. <!-- source: internal/test/runner/runner_exec.go -- runTest/runOrchestrated timeout resolution, withParallelHeadroom --> <!-- source: internal/test/runner/parallel.go -- ParallelTimeoutHeadroom -->
- **Inner gates derive from that budget, never from a constant** — a fixed inner deadline is sized against one machine and lies at any other speed, so the gates a daemon enforces *inside* a test derive from the resolved per-test budget and then take the same parallel headroom (applied once, on top). Three do this today. The `await=stderr` fence (`0.8 ×` budget, floored, clamped back down to the budget) and the plugin-startup stall window handed over as `ze_plugin_stage_timeout` (same shape, floor `10s`) are enforced by the runner. The third is enforced by a compiled fixture in its own process: the runner publishes the resolved budget as `ze.test.budget`, `ChildTestBudget` reads it back, and `WaitBudget` / `WaitAttempts` hand each fixture wait a percentage of it. That route exists because a fixture is a child racing its own clock, so the headroom the runner applies to the budget it measures the child against never reached the deadlines inside the child: `storageCommand` held 25s while the sixteen `.ci` files driving it declared `option=timeout:value=90s`, and `startFixtureDaemon` polled a hand-written 450 attempts kept in step with a 60s budget by nothing but a comment. A fixture percentage is below 100 for the same ordering reason the other two shares are below 1: the fixture's own message, which names what it was waiting for, must beat the runner's bare timeout kill. <!-- source: internal/test/runner/parallel.go -- TestBudgetEnv, ChildTestBudget --> <!-- source: internal/test/fixture/budget.go -- WaitBudget, WaitAttempts --> The stall window is **not** a budget for how long a startup stage may take — `StartupCoordinator.WaitForStageProgress` already waits on the *condition*, restarting its window every time any plugin completes a stage — it bounds only how long the whole tier may go with zero progress before it is declared wedged. Machine speed therefore enters through the test's own declared budget, which is where an author already expresses it: the ospfv3 netns tests declare `timeout=15s` for work that costs 1.6s natively, and `./le test qemu netns` runs `-p 1`, so the parallel headroom is a no-op there and could not compensate for emulation. The share is below 1 for ordering, not for speed: the watchdog must expire *before* the outer budget so the failure names the wedged plugin instead of reporting an opaque test timeout. <!-- source: internal/test/runner/plugin_stage_stall.go -- pluginStageStall, pluginStageStallEnv --> <!-- source: internal/test/runner/await_stderr.go -- defaultAwaitStderrTimeout --> <!-- source: internal/component/plugin/startup_coordinator.go -- WaitForStageProgress, noteProgressLocked --> <!-- source: internal/component/plugin/server/server.go -- defaultStageTimeout, ze.plugin.stage.timeout -->
- **External-probe test knob** — `ze.doctor`-style reachability checks probe real network destinations with multi-second timeouts; functional tests set `ze.test.doctor.probe-timeout` (the runner injects `250ms`) so probes to deliberately-unreachable fixtures fail fast instead of dominating wall-clock. The override only shortens a probe, never lengthens it, so production keeps its per-check defaults. <!-- source: internal/core/diagnostic/doctor_probe.go -- DoctorProbeTimeout, DoctorProbeTimeoutEnv --> <!-- source: internal/test/runner/runner_exec.go -- proc.Env probe-timeout injection -->
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

### Port leases

`.ci` execution leases `$PORT`, `$PORT2` and `$PORT3` through `LeaseTestPorts` before
expanding commands. `ReservePorts` uses the same per-port advisory lock mechanism
for web cases. The stable directory is `/tmp/le-port-locks-<effective-uid>`, not
a directory under the caller's `TMPDIR`, checkout, or session. A flat per-user
directory avoids requiring access through another user's private lock directory.
Participating same-user runners share lock inodes across those boundaries.
Directory and file creation modes are 0700 and 0600, respectively; cross-user
coordination is unsupported. An existing directory is checked before use: one
that is a link, is owned by another user, or that the group or others can
write is refused with an error naming its owner and mode, because a writer
could unlink a held lock file and a replacement inode would admit a second
holder.
<!-- source: internal/test/runner/ports.go -- checkPortLockDir --> Lock files MUST NOT be unlinked, even after
release, because a replacement inode would admit a second holder.

All participating runners MUST be rebuilt after this namespace cutover.
Runners using the old `le-port-locks` directory under their temporary directory
do not share these locks. There is no fallback or hybrid locking mode.

While locked, a candidate range is probed on both `127.0.0.1` (`tcp4`) and `::1`
(`tcp6`). All probe listeners remain open until the complete range is checked,
then close so children can bind. Only an unavailable IPv6 family, protocol, or
loopback address permits omitting IPv6; an occupied IPv6 port rejects the range.
`LeaseTestPorts` keeps its 25000-32759 fallback band. The lease owner MUST release
its locks after its use of the ports ends.

The plugin alias/shape, command-catalog plugin-shape, config-archive-prune,
SSH local-command and pipe-alias completion daemon fixtures receive the
runner-owned `$PORT` as their fixture argument.
`startFixtureDaemon` parses that actual port with `parsePort` before launching
and passes it through `envTestBGPPort`; it does not derive a port from the PID.
The runner MUST retain the lease until the fixture process exits, and these
fixture callers MUST stop and reap their daemon before returning, so ownership
outlasts the daemon's socket use.
<!-- source: internal/test/fixture/plugin_fixture_11_alias.go -- startFixtureDaemon, aliasDriver, fixtureDaemon.stop -->
<!-- source: internal/test/fixture/plugin_fixture_command_catalog.go -- commandCatalogPluginShape -->
<!-- source: internal/test/fixture/config_archive_prune_fixture.go -- configArchivePrune -->
<!-- source: internal/test/fixture/ui_fixture_local_command_ssh.go -- localCommandDaemonSSH -->
<!-- source: internal/test/fixture/ui_fixture_pipe_alias_completion.go -- pipeAliasCompletion -->

A fixture whose daemon serves a second listener takes that port from the
same lease. The rbac-web-config-deny fixture receives `$PORT` for BGP and
`$PORT2` for the web server it configures, and it waits for the daemon's
ready file, then requires the daemon's `web server listening on` line for
that port, instead of polling a fixed address. The as112-doh test gives its
DoH listener `$PORT2` in its config and passes the same port to its observer
plugin. A fixed listener port is shared by every test that names it, so two
such tests running at once collide however the lease spans fall.
<!-- source: internal/test/fixture/plugin_fixture_12_rbac.go -- p12WebRBAC, p12WaitWebListener, p12WebRBACEnvironment -->
<!-- source: internal/test/fixture/plugin_fixture_02.go -- as112DoH02, as112DoHAddress02 -->
<!-- test: internal/test/fixture/plugin_fixture_12_rbac_test.go -- TestRBACWebFixtureTakesTheLeasedWebPort, TestRBACWebFixtureWaitsForTheReportedListener, TestAS112DoHFixtureTakesTheLeasedPort -->

These are same-host, same-user advisory leases for processes sharing `/tmp`,
not universal socket reservations. They cannot exclude a nonparticipating
process that binds after probing, or coordinate isolated lock filesystems.
Job admission and stress concurrency controls do not replace port leases.
<!-- source: internal/test/runner/ports.go -- LeaseTestPorts, ReservePorts, tryReservePortRange, reservePortLocks -->
<!-- source: internal/test/runner/runner_exec.go -- runTest -->

### Exclusive-group leases

`option=exclusive:group=<name>` uses a same-host, same-effective-user advisory
lease, not a per-runner channel. Separate native stress-repro children and
independent runner invocations therefore coordinate even with different
`TMPDIR`, checkout, or session paths. Unset groups acquire no lease; different
groups do not block each other. Group names remain declarations in the tests,
not a central registry.

The lease uses `group-<sha256-of-full-name>.lock` in the protected port-lock
directory above. Hashing keeps arbitrary group names inside that directory and
within filesystem name limits. The existing directory ownership and symlink
checks apply. Group files are created with mode 0600, opened without following
symlinks, and must be regular files owned by the effective user without group or
other write permission.
<!-- source: internal/test/runner/exclusive.go -- leaseExclusiveGroupIn, checkExclusiveGroupFile -->
<!-- source: internal/test/runner/ports.go -- portLockDirectory, checkPortLockDir -->

The scheduler rejects parse failures before skips, and handles both before any
group or semaphore wait. The scheduler MUST acquire the group before its concurrency semaphore; otherwise
waiting members could occupy every slot and starve unrelated tests. Both waits
honor the runner context. The group wait polls nonblocking `flock` every 20 ms,
without introducing a test deadline. Acquisition and release errors fail the
record visibly rather than dropping exclusion. The scheduler MUST retain the
lease through the test callback and its cleanup, then close it on every return
path. Kernel descriptor cleanup also releases it when the runner exits.
Lock files MUST NOT be unlinked, even after release: existing waiters still
reference the old inode, and a replacement would allow two holders.
<!-- source: internal/test/runner/parallel.go -- parallelRunner.Run -->
<!-- source: internal/test/runner/exclusive.go -- leaseExclusiveGroup, waitExclusiveGroup -->

Every participating runner MUST be rebuilt after this cutover. Old in-memory
gates do not coordinate with these leases, and there is no fallback to them.
Different users, isolated `/tmp` filesystems, and nonparticipating processes
remain outside the contract. The regression runs actual separate processes and
checks same-group cancellation, independent groups, and stable lock inodes
across release and reacquisition.
<!-- test: internal/test/runner/exclusive_process_test.go -- TestParallelRunnerExclusiveAcrossProcesses -->
<!-- test: internal/test/runner/exclusive_lease_test.go -- TestExclusiveGroupLeaseStableInodeAndCancellation, TestExclusiveGroupLeaseRefusesUnsafePaths, TestParallelRunnerExclusiveAcquisitionFailureIsVisible -->


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
| `option=timeout:value=<dur>` | The authored budget, default 30s. `CaseContext` applies headroom once from scheduler concurrency: x3 for concurrent cases, x1 for serial cases. Verify-mode environment does not change this policy. Browser commands, waits, and retries inherit the absolute deadline. Invalid authored durations fail parsing; scaling overflow is refused before execution |
| `option=skip:reason=<text>` | Skip the test; `reason` is surfaced in runner output |
| `option=viewport:width=<n>:height=<n>` | Resize the viewport before the first navigation |
| `option=locale:lang=<tag>` | Set `Accept-Language` for the session |
| `option=auth:user=<u>:password=<p>:role=<r>` | Repeatable. Seeds the user and starts the server with authentication instead of `--insecure-web` |
| `option=server:kind=web\|lg\|lg-no-engine\|chaos\|daemon` | Which server the harness starts. Default `web`. An unknown kind fails parsing, because starting the default one and asserting against it is a pass that proves nothing |
| `option=env:var=<name>:value=<v>` | Repeatable. Sets one environment variable on the SERVER process |
| `option=plugin:name=<internal plugin>` | Repeatable. Adds `plugin { internal <name> { use <name> } }` to the daemon kind's config, so a test drives a command only that plugin serves. Any other kind loads no config, so the harness refuses the test rather than run it without the plugin |

<!-- source: internal/component/web/testing/parser.go -- parseWBOption: timeout, skip, viewport, locale, auth, server, env, plugin -->
<!-- source: internal/component/web/testing/deadline.go -- CaseContext, Bound -->
<!-- source: internal/test/cli/cmd_web.go -- daemonWebConfig -->
The scheduler starts the case deadline before server startup. It passes the
parsed case and that context to `RunWBCase`; browser execution does not re-read
the authored file or restart its budget. Parse errors refuse execution before
server startup, including when the malformed file also declares a skip.
<!-- source: internal/test/cli/cmd_web.go -- zeTestRunWebTest -->
<!-- source: internal/component/web/testing/runner.go -- RunWBCase -->

Ze serves three htmx interfaces from three programs, and a browser proof of one
of them has to start that one.

| `kind` | What the harness starts | Scheme | Ports |
|--------|-------------------------|--------|-------|
| `web` | `ze start --web <port> --web-only` | `https` | 1 |
| `lg` | `le test peer --mode sink`, then `ze -` with a looking-glass listener and one peer dialling that sink | `http` | 2 |
| `lg-no-engine` | `le mock lg`, the real looking glass with a dispatcher that always fails | `http` | 1 |
| `chaos` | `le chaos run --in-process --web :<port>` | `http` | 1 |
| `daemon` | `ze start <config> --web <port> --insecure-web`, the web UI on a running daemon with one BGP peer that never connects | `https` | 1 |

The looking glass gets a peer because its pages read `show bgp`: without
one, every assertion would run against an empty table. `le chaos run` is a second
compile of `cmd/ze` under different tags, so it is built only when a selected
test asks for it, beside the `ze` binary the run is using.

`daemon` exists because only a running daemon owns the confirmed-commit
window: under `--web-only` a web `commit confirmed` is refused, so a test of
the window drives the web UI the daemon serves.
<!-- source: internal/test/cli/cmd_web.go -- zeTestStartDaemonWebServer -->

`lg-no-engine` exists because no configuration reaches the engine-unavailable
state: the looking glass dispatches in process, so a daemon with no BGP still
answers an empty peer list. `le mock lg` builds the REAL server through
`lg.NewLGServer` and injects one failing dispatcher, which is the only part that
is not production code. The two ends of the `lg` daemon peer carry different
loopback addresses (127.0.0.1 and 127.0.0.2), so a rule comparing a route
against the peer address has two values to compare.
<!-- source: internal/test/cli/cmd_web.go -- zeTestStartLGServer, zeTestStartLGNoEngineServer, zeTestStartChaosServer, zeTestResolveWebBinaries -->
<!-- source: internal/test/cli/cmd_lg.go -- CmdLG -->
<!-- source: internal/le/test/functional/binaries.go -- alternate chaos build -->

### Actions

| Action | Keys | Effect |
|--------|------|--------|
| `open` | `path` | Navigate to `baseURL + path` |
| `click` | `id` or `text` | Click element by id or visible text. A click by `id` first waits, inside the same retry budget, until no finite CSS transition or animation runs on the element or an element containing it: `agent-browser` measures the box and then presses at that point, so a control still sliding (the error drawer's tab) took the click somewhere else and reported success. Still moving at the deadline, the click fails and says so |
| `fill` | (`id` or `text`) + `value` | Clear and type `value` |
| `hover` | `id` or `text` | Hover element |
| `press` | `key` (+ optional `id`/`text`) | Press a key, optionally focused on an element |
| `wait` | `ms` (or none) | Wait `ms` milliseconds, or for in-flight network to settle. A wait longer than 30s touches the browser between sleeps: the `agent-browser` daemon reaps itself after 60s with no command and takes the page with it, so a pure sleep of a minute returned to a browser holding nothing |
| `wait-until` | `path` + `contains` | Re-open `path` until the page it serves contains the text. Stops polling at `expectDeadline`. **Leaves the browser on `path`** |
| `wait-html` | exactly one of `contains` or `not-contains` | Re-read the body of the page the browser already holds until it contains, or no longer contains, the text. Never opens, reloads or navigates, so it judges what an htmx answer did to the current DOM (an out-of-band swap, a class removed). Stops polling at `expectDeadline` under the case deadline, and fails naming the text with an excerpt of the last page it read. Any other key, an empty value or both keys together is refused |
| `login` | `user` + `password` | Submit the real login form once and wait for a loaded authenticated page before continuing |
| `back` | none | The browser's own back button. The only way to prove what a pushed URL does when the operator returns to it: htmx 2 restores its own history cache, htmx 4 keeps none and the browser navigates for real |
| `forward` | none | The browser's own forward button. htmx 4 traverses the Navigation API, which holds entries on both sides of the current one, and the entry AHEAD is reached by a second traversal that `back` cannot make |
| `screenshot` | `file` | Save a screenshot only after two render frames, loaded fonts, finite animations and fetch/XHR work settle under the case deadline |

<!-- source: internal/component/web/testing/runner.go -- executeAction: open/click/fill/hover/press/wait/wait-until/wait-html/login/back/forward/screenshot -->
<!-- source: internal/component/web/testing/runner.go -- waitHTML, waitHTMLState -->

`wait:ms=` is a blind wait and asserts on elapsed time, which is what
`ai/rules/completion.md` bans: it passes on a quiet host and fails on a busy one.
Reach for the wait that names the state instead.

| The step waits for | Use |
|--------------------|-----|
| A render the browser drives (an htmx swap, a JS update) | Nothing. A positive `expect=` polls the DOM until it lands |
| A mutation to reach the SERVER | `action=wait-until:path=<readback>:contains=<text>` |
| A text to leave the page in hand (an out-of-band swap clearing a bar) | `action=wait-html:not-contains=<text>` |
| In-flight requests to drain, with no assertion after | `action=wait` |

A positive `expect=` cannot stand in for `wait-until`: it re-reads the DOM the page
already holds, so a readback the browser never fetched again reports the state that
was true before the action. `action=wait` cannot either, because its idle predicate
is true both before a request begins and after it ends.

`wait-until` cannot judge an in-page swap either: it re-opens the page, and the
fresh render answers from the server, so a swap the browser never applied is
invisible to it. `expect=html:not-contains=` states an absence and gets only the
short `expectRetryDeadline` retry, so an answer slower than that window fails the
step without naming a state it waited for. `wait-html` names the state the step waits for
and polls the page in hand for it. `test/web/recorded-config-save.wb` uses it
after Confirm Commit, where the commit answer's out-of-band `#commit-bar` must
drop its `visible` class.

Login waits for a complete authenticated document, not fetch/XHR idleness.
The form submits a native POST, which those counters do not observe. An
immediate following navigation can replace that request before its session
cookie arrives. The login action polls for the authenticated page's chrome and
the absence of the login form under `expectDeadline`; it neither re-submits nor
re-opens the page. Failure retains the final snapshot.
<!-- source: internal/component/web/testing/runner.go -- Login, waitLogin, loginReadyExpr -->

The browser configuration demonstration and its validator parse the same
`test/web/recorded-config-save.wb` journey with `ParseWBFile`, then execute its
ordered steps with the same runner. It submits the real login form, edits and
saves the hostname, reviews the diff, confirms the commit, re-fetches the clean
pending diff and navigates to read the committed value. Its paced waits are
presentation holds, not evidence of readiness. `RunWBCaseWithCapture` adds only
an optional final capture: the same deadline bounds render/font/finite-animation
and request settling, plus the capture callback. Installed request observation
and finite counter/timestamp values are required: absent observation is not idle,
and capture refuses within its deadline before taking a screenshot. The
callback borrows the owned browser's CDP endpoint before normal owned cleanup,
including capture failure.
Failure diagnostics preserve the original command/deadline cause alongside the
last completed structured readiness observation: request instrumentation and
types/finite values, quiet time, document/font/visibility state, finite animation
targets/timing, and frame progress. Samples are labeled before or after the frame
wait; a stale sample cannot establish what a later timed-out eval was doing.
Diagnostic queries consume the existing capture retry/case budget.
<!-- source: internal/component/web/testing/runner.go -- RunWBCase, RunWBCaseWithCapture, runWBCase -->
<!-- source: internal/component/web/testing/capture.go -- WBCapture, settleCapture, capture -->
<!-- source: internal/le/site/terminaldemo/web_config.go -- runWebConfigJourney -->

Request observation is registered on the owned native daemon before the first
application navigation. The pinned agent-browser 0.26.0 does not consume
`AGENT_BROWSER_INIT_SCRIPTS`; the runner uses its newline-JSON `addinitscript`
action, backed by `Page.addScriptToEvaluateOnNewDocument` on the persistent
active-page CDP session. Before registration, it evaluates that same source with
an explicit `about:blank` guard and requires a positive installation result,
preserving browser exception details. Registration or evaluation failure prevents
application navigation and retains normal checked browser cleanup. It never
injects observation after an application page has already started its requests.
The endpoint follows the pinned native directory priority: explicit
`AGENT_BROWSER_SOCKET_DIR`, `XDG_RUNTIME_DIR/agent-browser`, the home directory's
`.agent-browser`, then platform temp. Unix uses the owned session's `.sock`;
Windows reads its actual `.port` sidecar. Dial/read/write share the case budget;
the cancellation callback is joined and each connection closed before return.
Responses must be complete newline-JSON, at most 1 MiB, and match the request ID
with an explicit success verdict. There is no version fallback or guessed idle.
<!-- source: internal/component/web/testing/render_observer.go -- Browser.renderObserverScript, renderObserver.command, renderObserverSocketDir -->
The pinned native contract is implemented by its
[script actions](https://github.com/vercel-labs/agent-browser/blob/v0.26.0/cli/src/native/actions.rs),
[CDP registration and evaluation](https://github.com/vercel-labs/agent-browser/blob/v0.26.0/cli/src/native/browser.rs),
and [session transport](https://github.com/vercel-labs/agent-browser/blob/v0.26.0/cli/src/connection.rs).

Two properties of `wait-until` bite if you assume otherwise.

**It leaves the browser on the path it polled.** Every expectation after it judges
the readback, not the page the test was driving. That is what makes
`commit-flow.wb` work: its closing `not-contains` must read `/config/diff`, because
the page discard redirects to never held the pending change anyway. When you want
the previous page back, `action=open` there yourself.

**The case deadline also bounds the command in progress.** Inner retry limits
cannot extend that deadline. A `waitLoad` that does not settle fails with the
last observed cause. Cleanup has a separate context. A failed owned browser
close makes the case fail and preserves its completed steps. Server cleanup
also propagates errors and joins its sole process owner. Native subprocess tests
exercise the production scheduler and callback with owned server/browser fixtures:
same-group admission stays blocked during browser cleanup, different groups finish,
and the group is reusable after success, failure or cancellation. A lock-only port
probe verifies retention independently of the server listener; a non-reaping wait
observation verifies that the server owner has joined before callback return.
These tests compile current Go source. `TestWebNativeCommand` separately drives
a rebuilt named `le test web` through malformed, skipped, successful,
browser-failure and startup-deadline cases. Its server and browser executables
are owned fixtures; neither proof establishes detached-browser descendant cleanup.
<!-- source: internal/test/cli/cmd_web_resource_integration_linux_test.go -- TestWebNativeExclusiveCleanup, TestWebAdmissionWorker -->
<!-- source: internal/test/cli/cmd_web_command_integration_linux_test.go -- TestWebNativeCommand -->

The browser integration packages have explicit development-host routes rather
than the default QEMU package pass:

- `internal/component/web/testing` requires host `agent-browser` and Chromium:
  `./le job run label browser-capture command env CGO_ENABLED=0 go test -tags integration ./internal/component/web/testing`.
- `internal/chaos/web` requires the same native browser dependencies and an
  active owned development-session root for retained capture evidence:
  `./le job run label browser-clock command env CGO_ENABLED=0 go test -tags integration ./internal/chaos/web`.
  Keep that session identity and checkout root when the compiled test is run
  inside the pinned private renderer; bind only the invocation’s owned scratch
  directory and the read-only real ownership marker, not shared session data.
- The Linux integration proofs in `internal/test/cli` require a current-source
  named `le` for their actual command child, not a package run that skips the
  required `-web-command-binary` input:
  `./le job run label browser-command command sh -c 'CGO_ENABLED=0 go test -tags integration ./internal/test/cli -web-command-binary="$LE_BIN"'`.

The QEMU coverage declarations name these host routes explicitly. A missing
browser dependency or omitted native-binary argument is not execution evidence.
<!-- source: internal/le/test/qemu/alltests.go -- excludedIntegrationPackages -->


Immediate browser commands use a separate two-second `WaitDelay` to bound output
drain after exit or cancellation. A retained pipe becomes a cleanup error rather
than an unbounded wait. This does not establish that detached Chrome or browser
daemon descendants have exited.
<!-- source: internal/component/web/testing/runner.go -- agentPipeWaitDelay, runAgentCore, runAgentOutput -->
<!-- source: internal/component/web/testing/runner.go -- waitUntil, waitLoad, inflightIdleExpr -->
<!-- source: internal/component/web/testing/expect.go -- retryPositive, retryCommand, expectDeadline -->

### Expectations

| Expectation | Keys | Passes when |
|-------------|------|-------------|
| `element` | `id` / `not-id` | element with that id is present / absent in the DOM |
| `element` | `text` / `not-text` | text is present / absent in the accessibility snapshot (case-insensitive) |
| `html` | `contains` / `not-contains` | the page BODY contains / does not contain the substring |
| `head` | `contains` / `not-contains` | the page HEAD contains / does not contain the substring. `html` cannot answer for it: `get html <selector>` returns one element's inner HTML, and it reads the body. The head is where a page states which assets it loads, and each page loads what its own markup needs (`internal/le/web/assets.Write`), so what a head does NOT carry is a property of the page |
| `breadcrumb` | `contains` / `not-contains` (CSV) | each segment is present / absent |
| `url` | `contains` / `not-contains` | the ADDRESS BAR contains / does not contain the substring. It read the accessibility snapshot until 2026-08-15, which never carries the address, so the assertion could only ever fail |
| `title` | `contains` | the page text contains the substring (case-insensitive) |

Each positive retry and transient-query retry has its own `expectDeadline`:
15 seconds serially, or 45 seconds with concurrent scheduler headroom. Sequential
fetch and positive stages retain separate allowances. The outer expectation
poller starts further attempts only within five seconds (15 concurrently); this
elapsed-time cutoff does not cancel an already-running check or become an
ancestor deadline. Checks without an internal retry still receive outer polling.
All work inherits the earlier absolute case deadline, so no retry resets or
extends the case budget. A negative check judges absence on one successful query,
although the outer poller also rechecks failed expectations. An absence therefore
proves something only when the preceding step made the state current.
<!-- source: internal/component/web/testing/expect.go -- checkExpectation: element/breadcrumb/html/head/url/title; retryPositive vs retryFetch -->
<!-- source: internal/component/web/testing/runner.go -- checkExpectationRetry, getHTML reads body, getHeadHTML reads head -->

### Browser integration (agent-browser)

The web runner shells out to the external `agent-browser` CLI for every browser
operation; if it is not on `PATH`, the suite skips.
<!-- source: internal/test/cli/cmd_web.go -- exec.LookPath("agent-browser") skip when absent -->
<!-- source: internal/component/web/testing/runner.go -- Browser methods invoke agent-browser via runAgent -->

The web suite runs each `.wb` test in parallel (capped at 4). Each test gets
its own `ze` daemon (own port via `ReservePorts` + own tmpdir config store).
It also gets its own browser session,
so scenarios that mutate and `commit` config cannot corrupt each other.
The session name carries the runner's process id and the test nick
(`le-web-<pid>-<nick>`). The `agent-browser` daemon is shared by every run
on the host, so a name from the nick alone made two concurrent runs drive one
browser for the same test number. Each test closes only its own session, and
the daemon's idle timeout reaps a session whose run died. No run closes all
sessions, because that kills the pages of every other run on the host.

The owned session overrides inherited `AGENT_BROWSER_SESSION` settings,
including case and dot/underscore aliases. Standalone browser instances also
allocate a process-scoped identity. Close refuses a browser without an owned
identity and never sends `close --all`.
<!-- source: internal/component/web/testing/runner.go -- agentEnv -->
<!-- source: internal/test/cli/cmd_web.go -- zeTestRunWebTest, per-test ReservePorts + MkdirTemp + session -->
<!-- source: internal/test/cli/cmd_web.go -- zeTestBrowserSession -->
<!-- source: internal/component/web/testing/runner.go -- newBrowserWithSession, agentEnv sets AGENT_BROWSER_SESSION -->

## `.ci` and `.et` formats

These formats are documented in [`ci-format.md`](ci-format.md):

- **`.ci`** — config/process tests: tmpfs blocks, `cmd=` process orchestration,
  and `expect=` assertions (exit code, stdout/stderr, JSON, syslog, file, HTTP).
- **`.et`** — editor TUI tests: `input=` actions, `expect=` editor-state
  assertions, plus `session=`/`restart=`/`wait=` steps, executed in source order.

Both record an ordered `Steps` slice like `.wb`, and both emit per-step trace
output on failure (and on all tests under `-v`).
<!-- source: internal/component/cli/testing/parser.go -- testCase.Steps, InputAction, Expectation -->
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
`internal/le/test/qemu/netns_linux.go` selects all four of its subsets by name for
that reason, and its `assert_named` guard refuses a subset still carrying a
numeric selector: a nick had already drifted there, with firewall `"17"`
resolving to `command-owner-firewall-root.ci` rather than to any `017-*.ci`.

## Which binaries a fixture reaches

`le test bgp` refuses an unknown suite verb with a non-zero exit and names the
verb. Help and an empty argument list still print usage successfully.
An orchestrated `.ci` with an unconsumed `stdin=peer` block fails discovery;
the runner reports that parse failure rather than executing a passing fixture.

A fixture runs `ze` by bare name. The runner builds `ze`, and only `ze`, into
a throwaway directory (`LE_TEST_NO_BUILD=1` skips that build and uses a
pre-built `ze`). No harness binary exists: every harness command is
`le test <name>`, and the runner runs inside that `le`, so the runner's own
executable (`os.Executable`) answers every harness exec.

| `.ci` exec head | Runs |
|-----------------|------|
| `ze` | the `ze` this run built |
| `le` | the runner's own executable, with the authored words |
| anything else | a PATH lookup |

A retired standalone harness name falls in the last row.
No program answers to it, so the step fails at the lookup.

So the `le` row runs whatever code the runner's own executable holds. A direct
`./le test <suite>` run executes the launcher's build of HEAD
(`bin/builds/<time>-<sha>/le`), never the working tree, so an uncommitted
fixture edit does not run. The runner says so. `Runner.Build`, on the route
that builds `ze` (not under `LE_TEST_NO_BUILD`), reads the build commit from
its own directory name (`gotoolchain.ParseBuildName`). It then lists the non-test Go files under
`internal/test/` that differ between that commit and the working tree
(`git diff --name-only <sha>` plus untracked files). When one of them does, it
keeps those in a package le links, from `go list -deps` of `./cmd/ze` with
le's tags. `Runner.Run` prints the result before the first test and again after
a failing summary. The notice names the commit, up to ten paths and their
count, and the two fixes: `./le test functional <suite>`, and
`./le --name session` followed by the words of the run. It changes no verdict
and no exit code. A `--name` build (`bin/le-<name>/le`) and the isolated set of
`./le test functional` carry the tree's code, and their directory names are not
build names, so they print nothing. So does a launcher build when no linked
file differs. A check that cannot answer (git or go failed) prints
`le may be stale:` with the cause, never silence.
<!-- source: internal/test/runner/stale.go -- findStaleBuild, staleNotice, checkoutStaleSource -->
<!-- source: internal/test/runner/runner.go -- Runner.Build, Runner.Run, printStaleNotice -->

The parse steps resolve the same heads the same way (`resolveParseExec`).
A step whose words are `le test peer` takes the
peer role (`launchesPeer`, `internal/test/runner/harness_exec.go`): its stdin
block goes to the peer as an expect file, it receives the leased BGP port, the
peer contract (`peer_contract.go`) validates its block, and a check-mode peer's
`successful` line governs the result.
`le test <name>` runs in the test's work directory when `test <name>` is a
harness command, which the runner reads off registration: under `test`, the
forwarding commands are the harness commands. `le mock <name>` runs there too
(`leMockArea`): a stand-in server's file flags (`le mock rtr --sync-release`,
`--outage-start`, `--outage-release`) name files the test's observer writes by
bare name in that directory. Every other `le` head keeps the repository root,
as `./le` and `go` do.

`TestRTRSyncReleaseNativeRunnerEntry` exercises this mock path with a current
named `le` binary supplied through `-rtr-native-le`. Its foreground observer
writes the release in the private directory and requires the complete cache
response over TCP, including the expected VRP and End of Data. Without the
binary argument the test skips; that skip is not native execution evidence.
This harness proof does not replace the functional BGP route assertions.

The runner prepends a shim directory to every child's PATH. It holds `ze` and
`le` as symlinks to the two binaries above, and nothing else. So
`exec.LookPath`, `exec.Command("ze", ...)`, and a plugin `run` string that
starts `le mock engine-steps` name the binaries this run uses. The shim exists
because a cross-compiled binary carries its target in the file name. One
directory holding two architectures once gave the QEMU guest the host's `ze`.

No child environment carries `ZE_LE_BUILD_NAME`: a child reached through the
`le` link is not the named build file, and `refuseWrongBuildName` would refuse
every one of them under `./le --name x`.

No child environment inherits the developer's `ze.config.dir`, under any
spelling, so no child seeds, starts or edits the operator's store. A DUT, and
every process of an orchestrated test, gets its test's root from `dutEnv`
instead (`docs/functional-tests.md`, "The DUT's store"). Any other child gets
none.
<!-- source: internal/test/runner/harness_exec.go -- droppedFromChild -->
<!-- source: internal/test/runner/record_dut_store.go -- dutEnv -->

Every child environment carries `ze_le_store_trim=off`, and an inherited
spelling of that key is dropped. Every `le` call otherwise starts le's hourly
store trim in the checkout it is pointed at, as a detached child that writes
`tmp/store-trim/` and removes store entries, inside trees that fixtures compare
byte for byte. The one fixture that tests the trim, `ui/le-store-trim-answers`,
sets the key back to `on` over its own throwaway checkout.
<!-- source: internal/test/runner/harness_exec.go -- storeTrimVariable, storeTrimOffEntry, replacedInChild -->
<!-- source: internal/test/runner/runner_exec_util.go -- childEnv -->

`$ZE_REPO_ROOT/bin/ze` is not that binary and MUST NOT be used to find it.
`.gitignore` excludes `bin/` and no verification job writes `ze` there. A
fixture that reads it therefore fails on the CI runner. It passes on a
developer machine that happens to hold a build, and then reports on a binary
the run never made. `ZE_REPO_ROOT` names the source tree, and it is for reading
TRACKED repository content: a golden config, a fixture input.

The native `le` binary follows that rule too, and `nativeLEBinary` is the one
helper that answers it. `./le test functional` builds `ze`, `ze-stripped` and
`le` from the tree under test into every isolated set, runs each suite as
`<set>/le test <suite>` with `LE_TEST_NO_BUILD=1` and `ZE_BIN`, and the fixture
finds that `le` by bare name on the same PATH. Reading
`$ZE_REPO_ROOT/bin/le` instead is what made 20 `ui` cases fail in every fresh
worktree and pass in a checkout where a developer had built one
(`plan/journal/gate-verdict-depends-on-the-machine.md`).

<!-- source: internal/test/runner/runner.go -- setupBinShims, childPathEnv -->
<!-- source: internal/test/runner/harness_exec.go -- leHarnessArea, leMockArea, droppedFromChild -->
<!-- source: internal/test/fixture/ui_fixture_common.go -- uiZEBinary -->
<!-- source: internal/test/fixture/fixture.go -- nativeLEBinary -->
<!-- source: internal/le/test/functional/binaries.go -- buildCommands, Prepare -->
<!-- source: internal/test/runner/runner_exec_util.go -- zeRepoRootEnv -->

## Scratch roots

The functional runner writes its per-run and per-test working directories
(configs, sockets, daemon pid and ready files) under
`sessionpath.DefaultScratchRoot()` / `EnsureScratchRoot(baseDir)` when a session
is active, rather than an unowned `$TMPDIR/ze-functional-*`. Off-session it
still uses the system temp directory.

<!-- source: internal/test/sessionpath -- DefaultScratchRoot, EnsureScratchRoot -->
<!-- source: internal/test/runner/runner.go -- run and per-test working directories -->

## Timing baseline and auto-timeout

`le test` saves per-test timing to `tmp/test-timings.json` as a rolling EMA with
alpha 0.3. After three samples the baseline drives two things:

- **Auto-timeout.** The per-test timeout is `min(global, max(5s, 5 x baseline avg))`.
  A test that normally takes 500ms gets a 5s timeout instead of the default 15s,
  so a hang is caught in seconds rather than minutes. An explicit `.ci`
  `timeout=` overrides it.
- **Slow detection.** A test exceeding 2x its baseline is flagged in the summary.

Baseline updates are suppressed during a contended run, so a slow run does not
pollute the EMA.

The baseline has a second reader. `./le test cost report suite <suite>` ranks a
suite's tests by their share of the suite's recorded time, and lists those
that are long against their suite rather than against their own history, which
slow detection never flags. It reads the file through `runner.ReadTimings`,
which refuses a missing or corrupt file where the runner starts afresh, and
takes the same three-sample minimum, `runner.BaselineSamplesMin`. Because the
baseline never holds a contended run's times, neither does the report.
`docs/contributing/running-commands.md`, "Which tests cost the most of their
run", documents it.
<!-- source: internal/test/runner/timing.go -- ReadTimings, BaselineSamplesMin, TimingsPath -->
<!-- source: internal/le/test/cost/cost.go -- rankBaseline, rankGroup -->

## Contended run verdicts

<!-- source: internal/test/runner/hostload.go -- HostLoad, isNearTimeout -->
<!-- source: internal/core/hostload/hostload.go -- Load.Contended -->

On a loaded machine the failure index is headed
`VERIFY FAILURE INDEX (CONTENDED RUN)` with host load details. That means load
exceeded the CPU count with concurrent `le test <name>` or `go test` processes. The harness count reads each process's argument list (`le` with `test` as its first argument), because the harness has no binary name of its own.

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

`./le test stress-repro run suite <suite>` recreates that pressure cheaply: CPU and GC
burner processes oversubscribe every core while many concurrent copies of one
suite loop, and it captures the FIRST failure's complete, untruncated output. It
sets `GOTRACEBACK=all` so a panic dumps every goroutine, reuses the isolated
binary set `internal/le/test/functional` prepared during the loaded window, and writes
the capture to `tmp/stress-repro/<slug>-<ts>.log`. Exit 0 means reproduced, 1 not
reproduced, 2 a setup error.

```
./le test stress-repro run suite rsvpte iterations 80
./le test stress-repro run suite rsvpte race
./le test stress-repro run suite encode burners 32 parallel 8
./le test stress-repro run suite plugin test 97 any-failure
./le test stress-repro run suite "plugin --draft" test 97 any-failure
```

The suite is a NAME from the functional suite table
(`internal/le/test/functional/suites.go`), and the child runs the `le test` argv
that table gives it, so `suite plugin` runs `le test bgp plugin` and `suite ui`
runs `le test ui`. A name the table does not hold is refused, with the valid
names, before the build or any burner starts. Words after the name (`--draft`)
and the `test` selector are split on whitespace and follow that argv, so a
multi-token selector reaches `le test` exactly as typed by hand.
Each concurrent invocation is a separate native process. Declared exclusive
groups still exclude their same-group siblings through the shared group leases;
stress parallelism does not bypass that test contract.
<!-- source: internal/le/test/stressrepro/run.go -- run -->
<!-- source: internal/le/test/stressrepro/process.go -- realProcessRunner.Invoke, runCommand -->
<!-- source: internal/le/test/stressrepro/run.go -- harnessArgs -->

By default only a CRASH signature (a panic, a `DATA RACE`, or a runtime error)
counts as a reproduction, and everything else is discarded down to the last 500
bytes. An assertion flake exits non-zero with no crash signature, so
`any-failure` is what keeps its evidence.

A no-build reproduction tests the isolated binary set it was given. After
changing daemon source, run the owning `./le test functional <suite>` action once
(`internal/le/test/functional.Prepare` rebuilds the pair) before trusting a verdict:
otherwise a fixed bug still reproduces against the stale binary.

## The must-fail suite

`internal/test/runner/testdata/mustfail/` holds one `.ci` per refusal or
judgement the runner owes, and every one of them MUST fail.
`TestCIMustFailFixturesAllFail` parses each fixture, runs it when it parses, and
fails the build when one PASSES, naming it.

Each fixture states the failure it expects on a `# must-fail: <text>` line, and
the gate compares that text against the parse error or against `rec.Error`. A
fixture that goes red for a NEW reason therefore fails the gate as loudly as one
that goes green: a must-fail suite that stops discriminating is the same defect
one level up.

It exists because the runner has produced the same class four times in one week:
a `.ci` reading green while asserting nothing, because the runner answered a
directive it did not understand instead of refusing it. No static check finds
that in general. The file is well formed and the assertion is real, and only the
STIMULUS is wrong. The one general detector is a forced red, and the only way to
keep a forced red is to store it.

Four fixtures are parse-time refusals, which is where three of the four known
instances lived. Two run a real child from `/bin`, so they judge the run path:
that a named `stdin=` block reaches the child's standard input, and that a
declared `expect=exit:code=` is still compared. Neither needs a built `ze`. The
`ze`-specific stdin routing is proven by `TestCIStdinPipesForNonDaemonZeCommand`
over `routeStdinBlock` instead, because reaching it from a `.ci` needs the
binary.
<!-- source: internal/test/runner/must_fail_test.go -- TestCIMustFailFixturesAllFail, mustFailMarker -->

## Per-step trace output

The `.ci`, editor, and web execution paths record `trace.StepResult` slices.
They emit two trace formats through `internal/test/trace`:

- **Human:** colored `checkmark`/`cross` glyphs, one line per step, with kind, assert, and failure detail.
- **Machine:** `VERIFY STEP: {json}` token per step, matching the `VERIFY FAILURE GROUP` convention from `failure_group.go`.

Trace is emitted automatically on failure (default tier). Every suite on the
parallel runner (`.ci`, parse, decode, editor, web) prints each failed test's
reason in a normal run: the error, and for editor and web the failed step
only (`trace.PrintFailedSteps`). Under `-v` a failed test prints its whole
step trace, and passing tests also show theirs. Until 2026-09-24 the parallel
runner called the failure callback only under `-v` or verify mode, so a normal
`./le test functional web` run named a failed test and never said why. The `.ci` runner emits trace in failure
reports when `rec.StepTrace` is non-empty, and `Report.printStepTraces` prints
every selected test's trace under `-v`. That last half was untrue for the `.ci`
runner until 2026-09-07: `RunOptions.Verbose` was carried from the command line
and read by nothing, so `le test <suite> -v` over a green suite printed no more
than a bare run.

The first step of each command is what the runner RAN: the argv it built, and
where the named `stdin=` block went (`piped`, or `written to a file, named in
the argv above`). The `.ci` says what to run and the runner can rewrite it, a
`ze -` daemon launch becoming `ze [flags] start <file>`, and neither rewrite
appeared anywhere an author reads. So a test whose stimulus was replaced looked
exactly like one whose stimulus was honored, which is how
`test/ui/bgp-decode-pcap-stdin.ci` asserted a decoded capture while the capture
reached the child as a file path it never wrote.
<!-- source: internal/test/runner/runner_exec_trace.go -- recordExecStep -->

That step is written before the process starts, so on its own it does not say
the process started. When the run ends, each command that started gains a
clock mark: `[clock start <wall time>, started +<s> pid <n>, first stderr
+<s>]`, or `no stderr before the run ended`. The offsets are measured from the
moment the test's deadline was set, and the wall time uses the daemon's slog
layout, so a daemon log line can be placed on the same clock. A timeout then
says where its budget went: before exec, in the exec, or in the process's own
startup. The base `bgp-delay-open` TIME30 is the case that needed it: its
daemon logged its first config line 21.7s into a 30s budget, and nothing could
say which of the three took the time.
<!-- source: internal/test/runner/runner_exec_trace.go -- execClock, firstOutput -->
<!-- source: internal/test/runner/report.go -- printStepTraces -->

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
