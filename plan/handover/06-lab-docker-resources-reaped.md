# Handover: lab Docker resources reaped, phases 1-2

Spec: `plan/spec-lab-docker-cleanup-and-go-test-containers.md`.
Written 2026-10-18 by the agent that landed phases 1-2.

## Landed

| Commit | Subject |
|--------|---------|
| `ffd98a4193` | le: move the process scanner into the leaf internal/le/procscan |
| `c2b69d6519` | interoplab: label lab Docker resources and reap a dead run's leftovers |

The spec commit after these carries the pasted `cleanup dry` and `cleanup`
runs (Functional Tests table) and the `TestKilledLabRunIsCleanedByNextStart`
result.

## What remains of phases 1-2

| Item | State | Next step |
|------|-------|-----------|
| `docs/architecture/testing/interop.md` | This change's hunks (How It Works steps 1 and 5, "Crash-safe teardown", the Prerequisites row) are NOT committed. The file also holds another session's uncommitted dnsmasq row naming `test/interop/Dockerfile.dnsmasq` and two `dhcpv4-*-dnsmasq` scenarios that are not in HEAD, so it was held back (`ai/rules/git-safety.md`) | Commit it once the dhcpv4 work lands (`spec-dhcpv4-client-drops-lease-every-period`, `plan/handover/05-dhcpv4-lease-maintenance.md`), or carry both together when that session's files are in HEAD |
| `TestKilledLabRunIsCleanedByNextStart` | Proved the verdict on the real daemon in an earlier run; no clean pass under host load. The `*-dm8b7wc2wq89` leftovers of the ~23:20 failure are gone (the post-fix `cleanup dry` found none; spec, Functional Tests). Not rerun after `546f386a3c`: load average 72.8 at 23:31, 44.6 at 23:38 | Rerun when `uptime` is under ~16: `ZE_REQUIRE_DOCKER=1 ./le job run label kill quiet command go test -tags docker -count=1 -run TestKilledLabRunIsCleanedByNextStart ./internal/le/interoplab/`, then check `docker ps -a --filter label=ze.lab` holds no `ze-orphan-*` resource, which proves the new per-run `t.Cleanup` on the daemon. Paste both into the spec row |
| ~~`cleanup` reports a client-side timeout as a failure~~ | DONE in `546f386a3c`: `Docker.confirmRemoval` re-observes after any failed removal (removed / failed / unknown), in `RemoveContainer`, `removeNetwork`, `releaseImage`, so `Lab.cleanup` and `claimName` share it. Four fake-runner tests, three red before the fix | none |
| ~~`TestDockerTagNeverJoinsIntegration` red~~ | DONE in `546f386a3c`: an in-test fixture tree is the negative case; red shown by mutating the predicate, green restored | none |
| ~~The kill test leaves its resources when a child fails~~ | DONE in `546f386a3c` (code): each child prints its `ze.lab.run` first and the parent's `t.Cleanup` removes that run's labeled resources through the wrapper. Unproven on the daemon until the rerun above | the rerun above |
| Real `cleanup` through `./le --name` | The 23:38 run through `./le --name lab06` failed to REBUILD on another session's uncommitted `internal/component/bgp/reactor/peer_static_routes.go`; the same binary was run directly and exited 0 (`2 removed, 0 kept, 0 failed, 0 unknown`) | none for this spec; a build break belongs to that session |
| AC-8 signal roots for deployment, perf, verify evidence and the terminal demo | Not done. Only the integration verbs and ipsec/radius `Run` are rooted | Moves into phase 3, whose files they are |
| Owed gates | `./le test unit all`, `./le test functional gating`, full `./le verify worktree`. Recorded in `plan/verification-debt/3e9fd812.md` | Run each in a fresh agent; clear the debt rows with `./le commit debt-clear` |
| Lint coverage of the docker-tagged file | `.golangci.yml` `build-tags` does not list `docker`, so the default lint never reads `orphans_docker_test.go`. `go vet -tags docker` passes after `546f386a3c` | Lint `internal/le/interoplab` with the docker tag added, in a fresh agent |
| Gates owed by `546f386a3c` | Recorded in `plan/verification-debt/ee26b4ec.md` (3 rows). Scoped lint over `internal/le/interoplab` and `internal/le/test/qemu` not run (five-minute rule) | Run in a fresh agent, clear with `./le commit debt-clear` |

## Owed to `docs/architecture/testing/interop.md` (appended 2026-10-18)

Not written into the file: it still carries another session's uncommitted
dnsmasq hunk. When that session lands, add this paragraph to "Crash-safe
teardown", right after the paragraph that starts "`./le test integration
cleanup` runs the cleanup by hand", and commit it with this change's other
held hunks:

> A removal call that fails, or that outlasts the 30 s client bound while the
> daemon completes it, is not reported failed on its own say. The wrapper
> inspects that exact resource again under the same bound
> (`Docker.confirmRemoval`): gone is "removed", still present is "failed" with
> the removal's error, and an inspect that fails too is "unknown", naming both
> errors. The summary counts the unknown outcomes, and either a failure or an
> unknown makes the command exit 1. The scenario teardown and the same-name
> pre-clean go through the same removals, so a container the daemon removed is
> never listed as a cleanup error. A listing that times out names no resource
> and stays a setup error with the command to run.
> `<!-- source: internal/le/interoplab/docker.go -- Docker.confirmRemoval -->`

## Untracked file not this work's

`plan/spec-lab-docker-cleanup-and-go-test-containers.md` is untracked and reads
like an earlier copy of this spec. Nothing in phases 1-2 refers to it; the owner
decides whether it goes.

## Phase 3 (MUST adoption), first agent, 2026-10-19

### Landed

| Commit | Subject | Covers |
|--------|---------|--------|
| `5b0f6a3e7c` | interoplab: one-shots run named and removed when canceled | wrapper extension, M-2, M-8 (the RunOneShot half of AC-14; the retag is phase 4) |
| `4fa80099f6` | terminaldemo: demo containers run through interoplab.RunOneShot | M-6, AC-13 (terminal-demo half), AC-8 (terminal-demo root) |

What the wrapper now does (`internal/le/interoplab/docker.go`, `process.go`):

- `Docker.RunOneShot` adds `--name ze-oneshot-<random>` (`randomOneShotName`;
  the fixture client uses `fixtureOneShotName`). When the runner returns an
  error (caller context canceled, or the one-shot's own bound), it calls
  `RemoveContainer` on `context.WithoutCancel(ctx)` and joins both errors. A
  non-zero exit returns a `commandError` with `Cause == nil` and a result
  carrying `ExitCode`; nothing is removed, because `--rm` did it.
- `OneShotContainer` and `ProcessCommand` gain `Stdin`, `Stdout`, `Stderr`;
  `systemProcessRunner.Run` streams to a set writer and captures only a nil one.
- `ProcessRunner`, `ProcessCommand`, `ProcessResult` are exported (gopls rename,
  plus the `integration`-tagged `kernelcheck_host_integration_test.go` by hand);
  `NewFixtureDocker(runner)` is the cross-package test double (fixed owner).
- Every docker command runs under `Docker.environ` = `dockerhost.Inherited()`
  (the Colima socket on macOS), which the terminal demo's own exec used to set.
- M-2: `kernelProbeArgv` became `kernelProbeContainer` (an `OneShotContainer`);
  `DockerKernel` runs it through `RunOneShot`, and `probeFailed` tells "did not
  run" from "exited non-zero", so the AppArmor and unreadable-answer branches
  still read a non-zero probe's output.

Terminal demo (`internal/le/site/terminaldemo`): `containerCommand` answers an
`interoplab.OneShotContainer` (flags unchanged, in the same order after the
labels); `runDemoContainer` runs it through `Engine.docker.RunOneShot(Engine.ctx)`
with `demoContainerTimeout = time.Hour`. That is a NEW bound: the old exec had
none, and the parent should confirm the owner accepts it. `Options.Context` and
`Options.Docker` are new; the validate and render actions root on
`signalRooted` (SIGINT, SIGTERM). The image build and `docker image inspect`
still go through the package's own `Executor` (they start no container); the
AC-16 gate must decide whether those count (see below). The dead `insertArgs`
and `indexOf` were deleted. Page edit: `docs/contributing/gh-pages.md`, the
paragraph after the `containerCommand` anchor.

### Evidence

- `go test -race -count=1 ./internal/le/interoplab/` ok; `./internal/le/site/terminaldemo/` ok (3.8s).
- `go vet -tags integration` and `-tags docker` over `internal/le/interoplab` clean.
- `TestDockerBuildRetainsImageAcrossRetag` (integration tag, real daemon, load 22): PASS in 78.04s after the M-8 edit.
- Red then green: `TestRunOneShotRemovesACanceledContainerByName` (both subtests) FAILED "recorded 1 commands, want the run and its removal" with the `RemoveContainer` call replaced by `var removeErr error`; PASS once restored.
- Red then green: `TestDemoContainerRunsUnderTheEngineContext` FAILED "validation under a canceled root context succeeded" with `RunOneShot(context.Background(), ...)`; PASS once restored.
- `TestRendererActionCancelsOnSIGINT`: PASS, not discriminated (removing the root kills the test process through SIGINT's default action: red, but not a clean record).
- Both commits carry `structural-red-ok`: `./le arch iface-resolution` is red on `internal/plugins/iface/dhcp/dhcp_v{4,6}_exchange_linux.go` (another session's DHCP work); `./le arch tier check` exited 0 when run alone.

### What remains of phase 3

| Item | State | Next step |
|------|-------|-----------|
| M-5 `internal/le/verify/evidence/evidence.go` (AC-13, AC-8) | NOT started. The file and `evidence_test.go` hold another session's uncommitted hunk (`./le verify current mode full` becomes `./le verify current`), and that session has 13 dirty files under `internal/le/verify/` reworking `verify current`. Carrying the hunk ahead of them can break the release-candidate script against HEAD (`ai/rules/git-safety.md`) | Once their verify work lands: `startDocker` becomes `RunOneShot` with `Stdin: os.Stdin`, `Stdout: os.Stderr`, `Stderr: io.MultiWriter(os.Stderr, tail)`; `Runner.dockerArgs` becomes the `OneShotContainer` (drop `run --rm`; the three named cache volumes stay as `-v` Arguments, untouched). The run is "deliberately unbounded" today and RunOneShot needs a positive timeout, so the bound is an owner question ("which bound"). The action roots on `signalContext` in `internal/le/verify/actions.go`. The argv tests read through `NewFixtureDocker` |
| M-3 `internal/le/test/deployment` (AC-11, AC-8) | NOT started | Own docker in `deployment.go` (`ensureImage`: inspect and pull; `removeContainer`: error discarded), `l2tp.go` (the daemon, peer, start and install execs built by `L2TP.daemonArgs`, `L2TP.peerArgs`, `L2TP.containerArgs`, `L2TP.installArgs`), `vppiface.go` (`vppIface.containerArgs`, `vppIface.vppArgs`, the daemon exec, `docker logs`), `vppevidence.go` (its exec and the `pkill` exec), `vppevidencerun.go` (the peer exec). They need a detached labeled container with privileged caps, `--device /dev/net/tun`, `/dev/vhost-net`, `--rm -d ... sleep infinity`, plus `Exec`, `ExecDetached`, `Logs`, `Pull`, `RemoveContainer` (these exist). Extend the wrapper ONCE with a typed run spec (devices, capabilities, mounts, network mode), not a raw argv field; read `plan/pre-release/spec-lab-containers-least-privilege.md` first, it edits `runContainer`. Removal errors go into each verb's report. The deployment verbs need a signal root |
| M-4 `internal/test/perfrunner/run.go` (AC-12, AC-8) | NOT started | `Runner.command` and its docker callers, `Runner.startDUT` (`run -d`), the `sleep 3600` sender in `Runner.runPerf`, `network create`, image builds, `rm -f` in `Runner.stop` and a defer (errors discarded), the `docker exec ... le test http-get` run. Needs the same typed detached spec as M-3 plus an exported network create; `./le perf` gets a signal root |
| AC-16 gate `TestNoDockerExecOutsideInteroplab` | NOT started. It lands only after M-3, M-4 and M-5, or it lands red | Host it in an existing `./le arch` gate; exempt N-4 (`internal/appliance/kernelbuilder`, `deployment/gokrazyimage.go`) by reading the inventory decision, not a hand list. Decide whether the terminal demo's `docker build` and `image inspect` through its `Executor` (the program sits in a slice, through the `dockerProgram` constant) is a refused site: AC-16 names "literal or constant" programs |
| AC-8 tests for deployment, `./le perf`, `./le verify evidence` | NOT started | One test per root in the shape of `TestNativeRunnerCancelsOnSIGINT` (`internal/le/test/integration/cleanup_test.go`); the terminal-demo one is `TestRendererActionCancelsOnSIGINT` |

### Owed to `docs/architecture/testing/interop.md` (phase 3)

The file is still blocked by another session's hunks. When it is free:

1. The `kernelcheck.go` anchor that lists `dockerAppArmor, appArmorProfileMissing, appArmorProfileUnapplied, appArmorRefusal, kernelProbeArgv`: `kernelProbeArgv` is now `kernelProbeContainer`. An anchor gate may report the stale symbol until then.
2. In "Crash-safe teardown", after the `confirmRemoval` paragraph above:

> A one-shot container (`Docker.RunOneShot`: the kernel probe, a host
> preflight, a terminal-demo render) runs in the foreground with `--rm` and a
> `ze-oneshot-<random>` name. Killing the docker client stops nothing the
> daemon runs, so when the caller's context is canceled or the one-shot's bound
> ends, the wrapper removes the container by that name on a context the
> cancellation does not reach, and reports the removal's outcome with the
> run's error.
> `<!-- source: internal/le/interoplab/docker.go -- Docker.RunOneShot -->`

### Gates owed (not run here: the five-minute rule)

`./le go lint run` over `internal/le/interoplab` and `internal/le/site/terminaldemo`
(with the `docker` and `integration` tags too), `./le test unit all`,
`./le test functional gating`. The debt rows are in
`plan/verification-debt/d58beadc.md`.

## Phase 3, second agent, 2026-10-19

### Landed

| Commit | Subject | Covers |
|--------|---------|--------|
| `30ce614efd` | deployment: proof containers run through interoplab.Docker | M-3, AC-11, AC-8 (deployment roots) |
| the commit carrying this section | docs: interop teardown for removal, one-shots and detached runs | the interop.md text owed by phases 1-3 (now unblocked: the dnsmasq hunk landed in `cc69746db8`), and the stale `kernelProbeArgv` anchor |

The wrapper extension, made once (`internal/le/interoplab/container.go`):
`DetachedContainer` + `Docker.RunDetached` (name, image, platform, network and
a `netip.Addr` address, capabilities, devices, mounts, environment, working
directory, entrypoint, command; owner labels; `run -d` with no `--rm`, removal
is explicit), `ContainerExec` + `Docker.ExecAttached` (attached Stdin, Stdout,
Stderr, `Interactive`, environment, a positive own Timeout; a non-zero exit is
an error) and `Docker.ImagePresent`. No raw argv field. `grantcheck`
classifies `--label` as no grant.

Deployment (`internal/le/test/deployment`): no `exec.Command*("docker")` is
left in the package, tests included (`look("docker", "go")` is a PATH lookup).
`startWatchedExec` runs a watched daemon through `ExecAttached` in a named
goroutine (`running.execUntilDone`); stop cancels the client, and
`running.execErr` keeps the outcome. `removeContainer(ctx, docker, name)` runs
on `context.WithoutCancel` and answers the error text, which lands in
`L2TPReport`, `VPPIfaceReport` and `VPPReport.ContainerRemoval`
(`container-removal-error`), is printed by `writeContainerRemoval` with
`./le test integration cleanup`, and makes the verb exit 1. `signalRooted` in
`actions.go` roots l2tp-test, vpp-iface-test, vpp-test, docker-l2tp-ppp-test
and docker-pppoe-accel-test. `vppIface` keeps its root as `ctx` (the terminal
demo's `Engine.ctx` precedent), set by `Run(ctx)`. The pull bound is now the
wrapper's 10 min (it was 15 min).

### Evidence

- `go test -race -count=1 ./internal/le/test/deployment/ ./internal/le/interoplab/...`: deployment, interoplab, grantcheck and every lab package ok, except foreign reds in untouched packages: `interoplab/bgp` `TestCheckerPopulationMatchesProducer` and `TestAnnouncementPlanOrderAndTimingFixture`; `interoplab/pppoe` `TestPPPoELabPeersRunWithoutPrivilege` ("scenario ipv6cp-missing-option has no Go checker"; that scenario directory is not tracked).
- `go vet -tags integration ./internal/le/test/deployment/` clean. The SRv6 proofs now use `startWatchedExec`, and an in-container `cp` into the scratch mount replaces `docker cp`.
- Red then green: `TestDeploymentVerbsCancelOnSignal` FAILED "payload not canceled code 0" for SIGINT and SIGTERM with `signalRooted` listening on SIGUSR1; `TestContainerRemovalSurvivesCancelAndIsReported` FAILED "removal under a canceled root failed" with the removal on the canceled context; both PASS restored. The test holds its own `signal.Notify`, so a broken root is a clean red, not a killed binary.
- New wrapper tests `TestRunDetachedRendersALabeledContainer`, `TestRunDetachedRefusesAnIncompleteSpec`, `TestExecAttachedCarriesStreamsAndBound`, `TestImagePresentTellsAbsentFromUnknown` pass; not mutation-discriminated.
- NOT run: the real `l2tp-test`, `vpp-test` and `vpp-iface-test` against a daemon (privileged VPP devices, `/dev/ppp`). Owed before the spec closes: it is the only proof that the streamed daemon exec, the `--interactive` stdin and the pkill stop behave as the raw execs did.

### What remains of phase 3

| Item | State | Next step |
|------|-------|-----------|
| M-5 `internal/le/verify/evidence/evidence.go` (AC-13, AC-8) | UNBLOCKED: the other session's verify rework landed in `839819ea2d`, and `internal/le/verify/evidence` is clean. Not started (budget) | The first agent's design above stands (`startDocker` and `Runner.dockerArgs` become one `RunOneShot`). `RunOneShot` refuses a non-positive timeout, so "unbounded as today" is a named constant of a century, never `math.MaxInt64` (`time.Now().Add` would overflow). **Owner decision owed: evidence container time limit.** Root the action on `signalContext` in `internal/le/verify/actions.go` |
| M-4 `internal/test/perfrunner/run.go` (AC-12, AC-8) | Not started (budget) | FIRST check the tier rule: `internal/test/perfrunner` importing `internal/le/interoplab` may be refused by `./le arch tier check`. `Runner.startDUT` and the `sleep 3600` sender in `Runner.runPerf` become `RunDetached` (network and `Address`, NET_ADMIN plus SYS_ADMIN, NET_RAW or SYS_CHROOT per DUT, the `GODEBUG` variable, `Runner.volumeArgs` as typed read-only `Mount`s); the execs become `Exec` or `ExecAttached` (`perf send` with its computed timeout, the pprof `http-get` with its file as Stdout); `Runner.stop`, the defer and `Runner.cleanup` use `RemoveContainer` with errors reported. `network create --subnet` needs an exported labeled fixed-subnet create (the lab's `createNetwork` takes candidates) and its removal. Builds go through `Docker.Build` only if its retained-tag semantics keep `dut.Image` usable; otherwise an exported labeled build is the open design point. Fake-runner tests only: perf is never measured on this machine. `./le perf` needs a signal root and its test |
| AC-16 gate `TestNoDockerExecOutsideInteroplab` | Not started | Unchanged from the row above; it lands only after M-4 and M-5. Deployment is clean for it now. The terminal demo's `docker build` and `image inspect` through its `Executor` is still the open call |
| Real deployment runs | Owed | `./le test deployment vpp-iface-test`, `vpp-test`, `l2tp-test` on a host with Docker and the devices; then `docker ps -a --filter label=ze.lab` shows nothing of theirs, and a Ctrl-C mid-run leaves nothing either |

### Gates owed (not run here: the five-minute rule)

Scoped `./le go lint run` over `internal/le/interoplab`,
`internal/le/interoplab/grantcheck` and `internal/le/test/deployment` (with
the `integration` tag too: the post-write hook reported 23 issues in the
package and printed one, fixed as `srcMount`), `./le test unit all`,
`./le test functional gating`. Debt rows: `plan/verification-debt/85b2c17f.md`.

## Phase 3, third agent (M-4), 2026-10-19

### Landed

| Commit | Subject | Covers |
|--------|---------|--------|
| see `git log --grep "perfrunner: DUTs, sender"` | perfrunner: DUTs, sender, network and builds run through interoplab | M-4, AC-12, AC-8 (`./le perf run`, `./le perf evidence-record` roots) |

Tier boundary: `internal/test/` is in `NonFeaturePrefixes` and
`DisableableNonProdPrefixes` (`internal/le/arch/tier/gates.go`), and
`pluginNonProduction` treats it as non-production, so `perfrunner` importing
`interoplab` is allowed; `./le arch tier check` exited 0 after the change. No
import cycle (`interoplab` does not reach `perfrunner`).

Wrapper extension, made once: `ImageBuild.NamedContexts []NamedBuildContext`
(`--build-context name=path`, refused when either half is empty) for FreeRtr;
`Docker.ContainerRunning` (`container inspect {{.State.Running}}`, an
unreadable answer is an error); `createNetwork`, `removeNetwork`,
`releaseImage` exported as `CreateNetwork`, `RemoveNetwork`, `ReleaseImage`
(sed over the seven interoplab files naming them; tagged files vetted).

perfrunner (`internal/test/perfrunner/run.go`): no `docker` argv is left;
`CommandRunner` now runs only the `go build` of le and the reporter. DUTs and
the sender are `RunDetached` (`dutContainer`, `senderContainer`; `DUT.IP` and
the sender/receiver addresses are `netip.Addr`), execs are `Exec` (setup,
failure now a warning, it was discarded) and `ExecAttached` (`perf send`, the
pprof `http-get` into its file), images are `Build` then `ReleaseImage` (the
buildx detection and the `buildx build --load` path are deleted: every build
is the wrapper's `docker build`), FRR is `Pull`. `Runner.containers` records
each container before its run; `removeContainer` and `cleanup` remove on
`context.WithoutCancel`, print a failure with `./le test integration cleanup`,
and `Runner.removalFailed` makes `Execute` answer 1. `Execute(ctx, ...)`: an
ended root stops between DUTs, still cleans up, answers 1. An image inspect
that fails is a failed DUT, never "skipped". `internal/le/perf`: `Bench.Run`,
`EvidenceRecord` and the three seams take ctx; `signalRooted` in `actions.go`
roots `perf run` and `perf evidence-record`.

Pages: `docs/guide/benchmarking.md` (labels, Ctrl-C, removal failures),
`docs/architecture/testing/interop.md` (the benchmark as a RunDetached caller,
`ContainerRunning`, `NamedBuildContext`, `ReleaseImage`).

### Evidence

- `go test -race -count=1` over `internal/test/perfrunner`, `internal/le/perf`,
  `internal/le/interoplab`, `internal/le/interoplab/grantcheck`: all ok.
  `go vet -tags "integration docker"` over interoplab, perfrunner, perf: clean.
- Red then green (mutation script, files restored and `cmp`-checked):
  `TestPerfVerbsCancelOnSignal` FAILED "payload not canceled code 0" for
  SIGINT and SIGTERM with the root on SIGKILL+SIGUSR1;
  `TestPerfInterruptedRunRemovesWhatItStarted` FAILED "a removal failed under
  the canceled root" with the removal on `ctx`;
  `TestPerfRemovalFailureIsReportedAndFailsTheRun` FAILED "did not fail the
  run" and "stderr lacks error: remove container" with the error discarded.
  All PASS restored.
- New, not mutation-discriminated: `TestPerfContainersCarryTheirFlagsAndLabels`,
  `TestContainerRunningTellsStoppedFromUnknown`, `TestBuildRendersNamedContexts`.
  `TestPerfRunnerMountsLinuxLe` keeps every assertion, now read from the
  fixture docker's argv.
- NOT run (forbidden here): `./le perf run` against Docker. Owed on the owner's
  Mac: one `./le perf run dut ze`, a Ctrl-C mid-run, then
  `docker ps -a --filter label=ze.lab` shows nothing of the run, and the
  FreeRtr build with `--build-context` under plain `docker build`.
- Lint: scoped lint over interoplab, grantcheck, deployment, perfrunner and
  perf ends at 0 issues in the linux, linux+integration, darwin and
  capability flavors (13 goconst in deployment, one misspelling in
  `kernelcheck.go`, three `errors.AsType` in `docker.go`, all fixed).
- Landed by the main thread: `f171a5d47c` (M-4) and `462bd8a060` (the
  deployment goconst fix).

### Next (phase 3 remainder, owner decisions D-1..D-4 recorded in the spec)

- M-5 `verify/evidence` onto `RunOneShot` with a 4-hour bound (D-1); the
  other session's rework landed in `839819ea2d`.
- Terminal demo `docker build` / `image inspect` onto `Docker.Build` and
  `ImagePresent` (D-2).
- `Pull` takes a caller bound; deployment pulls back to 15 minutes (D-3).
- The AC-16 gate, once nothing outside `internal/le/interoplab` execs docker
  except the NOT rows.

## Phase 3, fourth agent (D-1..D-3, M-5, M-6 remainder), 2026-10-19

### Landed

| Commit | Subject | Covers |
|--------|---------|--------|
| `8a91aa8ebb` | interoplab: Pull takes a bound that only lengthens the default | D-3: `Docker.Pull(ctx, image, boundMin)` runs under `max(boundMin, dockerPullTimeout)`; deployment's `ensureImage` passes `deploymentPullTimeout` (15m); lab and perfrunner pass 0 |
| `5cb877cbb6` | terminaldemo: image build and check go through interoplab.Docker | D-2, M-6 remainder: `Engine.BuildImage` is `Docker.Build` then `ReleaseImage`; `Engine.requireRendererImage` is `Docker.ImagePresent` (an unanswered inspect is an error, not "not built"); the Executor runs ffmpeg and go only; `dockerCommandBuild`/`dockerCommandImage` deleted; `docs/contributing/gh-pages.md` |
| `a757147bb3` | verify evidence: the build container runs through RunOneShot | M-5, AC-13, AC-8, D-1: `Runner.Start`/`startDocker` deleted; `Runner.container` is the `OneShotContainer` (same caps, devices, mounts, three cache volumes); `Run(ctx)` under `evidenceContainerTimeout` (4h); interruption is an error, never a verdict; `signalRooted` in `actions.go`; `docs/architecture/testing/interop.md` |

### Evidence

- `go test -race -count=1` over `internal/le/interoplab`, `internal/le/test/deployment` (the new test), `internal/le/site/terminaldemo`, `internal/le/verify/evidence`, `internal/le/verify`: ok. `go vet` (plain and `integration docker`) over interoplab, perfrunner, deployment: clean.
- Red then green by mutation, files restored and `cmp`-checked: `TestPullBoundOnlyLengthensTheDefault` (bound ignored: "10m0s, want 15m0s"; `cmp.Or` for `max`: "1m0s, want 10m0s"), `TestEnsureImagePullsUnderTheDeploymentBound` (bound ignored), `TestBuildImageTagsTheImageTheManifestNames` (release skipped: "commands = 1"), `TestRenderReportsAnImageCheckTheDaemonDidNotAnswer` (error read as absent), `TestTheContainerRunsUnderTheFourHourBound` (1h), `TestAnInterruptedRunRemovesTheContainerAndKeepsTheCaches` (Canceled guard removed), `TestReleaseCandidateCancelsOnSignal` (root on SIGKILL+SIGUSR1).
- NOT run (forbidden here): a real `./le verify evidence release-candidate` with a Ctrl-C, then `docker ps -a --filter label=ze.lab` empty; a real `./le site terminal-demo image-build`.

### AC-16 gate: not built, blocked on spec D-5

Survivor census (grep of every `exec.Command*` naming `docker` by literal or by a constant valued `docker`, outside `internal/le/interoplab`, after the three commits): only M-7, `internal/component/bgp/plugins/rpki/rpki_live_test.go` (lines 48, 134, 141, 271, 320, 326, 346) and `rtr_downgrade_test.go` (26, 31, 49). M-7 converts in phase 4 (AC-15, `labtest`). No NOT row trips a call-site check: N-4 `kernelbuilder` passes `backendDocker` to `runCommand(ctx, req, name, ...)`; `deployment/gokrazyimage.go` execs `ze-host`; N-6 (`setup/tools.go`, `toolstub`, `fixture`) names `docker` as a stub file or a tool name; N-7 `setup/dockerkernel` execs `step.Argv[0]`. So the gate needs no exemption at all once M-7 lands. Host: `internal/le/arch/tier`, per the spec; a red there is charged on every session's commit, which is why it was not landed red.

### Gates owed (not run here: the five-minute rule)

Scoped `./le go lint run` over `internal/le/interoplab`, `internal/le/test/deployment`, `internal/test/perfrunner`, `internal/le/site/terminaldemo`, `internal/le/verify/evidence`; `./le test unit all`; `./le test functional gating`.

## Phase 4, first agent (written by the main thread from the agent's report)

Landed: `f7c4cd8d92` (labtest helper: `Start`, AC-18, AC-19, AC-26; Q-6 by
build tag, `pull_docker.go` fails an unpullable image, `pull_live.go` skips it
ad hoc; the wrapper gains loopback-only `DetachedContainer.PublishedPorts` and
`Docker.PublishedPort`), `783143daaf` (AC-17: arch tier refuses a shipped
importer of labtest), `fa4404fa13` (M-7, AC-15: rpki live tests start stayrtr
through labtest, no docker exec left, assertions unchanged, no RFC tag),
`a73d572b19` (AC-16: arch tier refuses a docker exec outside interoplab, no
exemptions, green over the tree). Every new test was shown red by mutation.

Real daemon: `TestLabtestStartsServerAndCleansUp` passed in 42s (two earlier
runs overran the 60s run and 30s rm bounds at load about 25, leaving nothing).
Live RPKI with `ZE_REQUIRE_DOCKER=1`: Validation passed, Downgrade passed
(750092 IPv4 + 249908 IPv6 VRPs), ASPA skipped by its own existing check.

Main-thread reading of AC-18: labtest offers a published port or the host
network and creates no network of its own, which the AC's "a network or
published port" allows; its cleanup removes the container only.

Uncommitted, untested in the working tree: the spec (Phase 4/5, D-5 DECIDED
(a)); `internal/le/test/integration` (`runLiveRPKI`, `runRequiringDocker`,
`liveRPKIArgv`, `requireDockerEnvironment`; the live-rpki row is Native so it
sets `ZE_REQUIRE_DOCKER=1`; `TestLiveRPKIRequiresDocker` unrun); and
`docs/functional-tests.md` rows for Live RPKI and Docker peers.

Next:
1. Run `TestLiveRPKIRequiresDocker` and show it red by mutation.
2. M-8: retag `image_retention_integration_test.go` to `docker` (rename to
   `_docker_test.go` through remove + file) with the `ZE_REQUIRE_DOCKER` fail
   arm; update the `internal/le/interoplab` reason in
   `excludedIntegrationPackages` (`internal/le/test/qemu/alltests.go`); its only
   remaining integration test is `TestDockerKernelCheckOnThisHost`.
3. Lint coverage: add `docker` to the `capability` flavor in `flavorMatrix`
   (`internal/le/go/lint/matrix.go`) and to the pinned row in `parity_test.go`.
4. `evidence-nightly.yml`: `docker-peers` and `live-rpki` jobs, and both in
   `want` of `TestEvidenceNightlyScheduleActionsAndPrivileges`.
5. Run `./le test integration docker-peers` once and paste its summary.
6. Docs: qemu-integration.md, ci-workflows.md, a labtest sentence in
   testing.md, ai/INDEX.md rows.
7. Gates owed: scoped lint over interoplab/..., arch/tier and rpki (tags docker
   and live); `./le test unit all`; `./le test functional gating`.

## Phase 4, second agent and main thread

Landed: `8929d9bd55` (live-rpki under `ZE_REQUIRE_DOCKER=1`;
`TestLiveRPKIRequiresDocker` red by mutation), `af1416e9b5` (M-8 retention test
on the `docker` tag; real daemon pass in 52s, fails without docker under
`ZE_REQUIRE_DOCKER=1`, skips without it), `31ae065d19` (the lint capability
flavor carries `docker`), `3ea96e151f` (evidence-nightly `docker-peers` and
`live-rpki` jobs, pinned by workflowcheck), `12eb2f1dee` (testing docs and the
spec's docker-peers result). `./le test integration docker-peers` on this host:
exit 0. Scoped lint with the docker tag: 0 issues in this spec's files.

Still open: AC-25 (first nightly `docker-peers` run id, needs an owner-ordered
push); phase 5 (SHOULD rows S-1 TACACS+, S-2 NTP, S-3 gNMI, S-4 flow-export,
per Q-5); gates `./le test unit all`, `./le test functional gating`, the full
`./le go lint run`; real runs of `l2tp-test`, `vpp-test`, `vpp-iface-test`,
`verify evidence release-candidate` with Ctrl-C, the terminal-demo image build;
on the owner's Mac a `./le perf run dut ze` with Ctrl-C and the FreeRtr build.

## Phase 5, S-1 (TACACS+), 2026-10-19

Test: `TestTacPlusNGInterop` in
`internal/component/tacacs/tacplusng_docker_test.go` (tag `docker`, no RFC
tag, so no discrimination record: the walk is recorded here). Server:
`docker.io/christianbecker/tac_plus-ng@sha256:f17972cf0471ec32d4947a4152c8bd6a383b205f0dd0b48602fa6fa243fa393c`
(tac_plus-ng from event-driven-servers, Alpine packaging, MIT, image built
2025-11-11). The image has no ENTRYPOINT, so `labtest.Spec.Command` runs
`/bin/sh -c` to write the test's config to `/tmp` and exec the daemon: no
mount, no Dockerfile, no labtest change. Readiness is a PAP answer through
Ze's client, not a TCP accept.

Subtests and what each asserts through Ze's own code
(`tacacsAuthenticator.Authenticate`, `tacacsAuthorizer.Authorize`,
`TacacsClient.SendAccounting`): login accepted (exact `aaa.AuthResult`, the
priv-lvl 15 profile), wrong password (`aaa.ErrAuthRejected`), wrong shared
secret (exact error `tacacs: all TACACS+ servers unreachable for
authentication`, zero result, client WARN log naming `ErrBadSecret`), command
permitted with a deny-all local fallback, command denied with a permit-all
local fallback, accounting START answered `AcctStatusSuccess`. Green in 16-38s.

Red walk (each mutation marked `// MUTATION-APPLIED`, verified by diff, run
with `-count=1`, restored from a saved copy and `cmp`-checked; no marker left):

| Mutation | Red subtest | Failure |
|----------|-------------|---------|
| `authenticator.go` FAIL branch returns `errServerFailure` | wrong password rejected | `Target error should be in err chain: expected "authentication rejected"` |
| `authorizer.go` AUTHOR FAIL returns `fallback()` | command denied | `Should be false` |
| `authenticator.go` `privLvlMap[privLvl-1]` | login accepted | `Received unexpected error` |
| `reply.go` length-sum check disabled | shared secret mismatch | log names `non-printable ASCII byte`, not `bad secret` |
| `client.go` accounting minor version `0xC1` | accounting start recorded | `Received unexpected error` |
| `authorizer.go` PASS_ADD returns `!allowed` | command permitted | `Should be true` |

Every other subtest stayed green under each mutation. Restored run: `ok` 16.2s.

Behavior found, conformant: a wrong shared secret is an ERROR (RFC 8907
Section 4.5, "the packet MUST be discarded and an ERROR signaled"), which
Section 4.4 makes "as if the server could not be connected to", so the chain
falls to the next server and to local accounts. Documented in
`docs/guide/tacacs.md`. Defect found, not fixed (does not block S-1): the
final `sendToServers` error drops the per-server cause, journal row in
`plan/journal/error-names-no-fix-command.md`.

Landed: `321ca49ce1` (test, guide, this section), `1c1410c010` (journal row).

`./le test integration docker-peers`, 2026-10-19 after `321ca49ce1`: code 0,
population `internal/component/tacacs` (36.8s), `internal/le/interoplab`
(153.7s), `internal/le/interoplab/labtest` (30.9s), `internal/plugins/ntp`
(46.2s, the S-2 session's uncommitted test).

Not committed: the spec's S-1 row (Implementation Phases audit table) is
written in the working tree, true against HEAD now. It was held back because
the same file carries the running S-2 session's row naming
`ntp_chrony_docker_test.go`, which is not in HEAD; whoever commits the S-2 row
carries the S-1 row with it.

Owed gates: scoped lint of `internal/component/tacacs` with the docker tag
(`go vet -tags docker` passed), `./le test unit all`.

## Phase 5, S-2 (NTP), 2026-10-19

Landed: `6480a1ad5c` (interoplab: `DetachedContainer.PublishedPorts` and
`Docker.PublishedPort` take a typed `ContainerPort` (`TCPPort`, `UDPPort`),
the zero transport refused before docker runs; `labtest.Spec.Port` is that
type and `labtest.Spec` gains `Entrypoint`; rpki live tests name
`interoplab.TCPPort`), `ead47ff0ce` (the NTP tests, the interop.md paragraph,
the journal row).

Server: `dockurr/chrony:4.9` (chrony 4.9, digest
`sha256:f584829f268b26b26b351000b60aa7c3507933873b6a6d77ffe9dcabcfdc7025`).
Its entry point writes chrony.conf from its environment and ignores the
command, so the test sets `Entrypoint: "sh"` and writes its own config.

Tests in `internal/plugins/ntp/ntp_chrony_docker_test.go` (tag `docker`, no
RFC tag) drive `syncWorker.doSync` with the real `ntp.Query`; only
`setClockFn` is a recorder. Each first queries chrony directly to confirm the
premise:

| Test | chrony config | Ze reports |
|------|---------------|------------|
| `TestNTPClientSyncsFromChrony` | `local stratum 7` | synced, source = server, stratum 7, reach 1, offset within 500ms, rtt in (0, 2s], root delay 0, one clock step |
| `TestNTPClientRejectsUnsynchronisedChrony` | `server 192.0.2.1` only | not synced, no source, reach 0, last error set, no clock step |

Red walk (`// MUTATION-APPLIED`, verified by grep, `-count=1`, restored from a
saved copy and `cmp`-checked, no marker left):

| Mutation in `ntp.go` `doSync` | Red test | Failure |
|-------------------------------|----------|---------|
| `resp.Validate()` result ignored | Rejects | "doSync synced from an unsynchronised server", one clock step, reach 00000001, empty last error |
| `ps.Stratum = resp.Stratum` dropped | Syncs | "published stratum = 0, want 7", "server stratum = 0, want 7" |

The other test stayed green under each. Restored: both PASS (12.9s, 7.7s).
Wrapper mutations (UDP rendered `/tcp`; unspecified rendered `/tcp`) turned
`TestRunDetachedPublishesOnLoopback` and `TestPublishedPortReadsTheFirstBinding` red.

Assumptions corrected: an unsynchronised chrony sends stratum 0, not 16, with
leap 3 and reference id 0. RFC 5905 Section 11.1: "Remember that MAXSTRAT is
mapped to zero in the transmitted packet." chrony serves no stratum 16, so
the reject test covers stratum 0 with leap 3. Ze does not store the reference
id or the leap indicator (`serverState`, `state.go`), so the tests cannot
assert them. The container reads the host kernel clock, so the offset can only
be bounded. A dropped offset copy would still pass the bound.

Defect found, not fixed (does not block S-2): Ze's last error for that reply
is "kiss of death received", because beevik/ntp `Response.Validate` tests
stratum 0 before leap 3. Journal row in
`plan/journal/error-answered-with-another-errors-code.md`.

`./le test integration docker-peers` on this host: exit 0, population
tacacs, interoplab, interoplab/labtest, plugins/ntp (ntp `ok` 50.1s).

Owed gates: scoped lint of `internal/plugins/ntp`, `internal/le/interoplab`,
`internal/le/interoplab/labtest` and rpki with the docker and live tags
(`go vet -tags docker` / `"docker live"` passed); `./le test unit all`.

## Phase 5, S-3 (gNMI), 2026-10-19

Client: `ghcr.io/openconfig/gnmic:0.42.1@sha256:3e135878f555772653a278742d2c7460b47c20e2907e07f611ecf4cc5a2261cf`
(gnmic 0.42.1, commit 6b35566f, Apache-2.0). Ze is the server, so the peer is
a client container: `labtest.Run` (new, `labtest.go`: `Client`, `run`) runs
gnmic to its exit through `Docker.RunOneShot` on the host network, with the
test's CA bind-mounted read-only, and answers its stdout, stderr and exit
code. `35ebe6a7a9` gave `OneShotContainer` typed `Network` and `Mounts` and
exported `OneShotDidNotExit` (was `probeFailed`); red walk for it: the
`--network` rendering removed reddens `TestRunOneShotRendersNetworkAndMounts`.

Test: `TestGnmicInterop` in `internal/component/gnmi/gnmic_docker_test.go`
(tag `docker`, no RFC tag). Ze's real server (`NewServer` + `Serve`, as the
package's own tests start it) listens on `127.0.0.1:0` with TLS (CA and
127.0.0.1 server certificate generated in the test, gnmic verifies with
`--tls-ca`) and a bearer token; Set goes through a real
`api.ConfigSessionManager` over the package's recording editor. gnmic prints
`--format protojson`, which the test decodes into the gNMI messages:

| Subtest | Asserts |
|---------|---------|
| capabilities | encodings exactly [JSON_IETF], gNMI version 0.10.0, model set equal to `yangloader.DefaultLoader` (13 modules) |
| get | one notification, path [bgp router-id], string value 192.0.2.1 |
| get of an absent path | gnmic stderr `code = NotFound desc = path not found: /bgp/no-such-leaf` |
| subscribe once | exactly two responses: the update with that value, then sync_response true |
| set | one result, op UPDATE, path [bgp router-id]; the commit holds the new value |
| no token / wrong token | gnmic fails with `code = Unauthenticated desc = missing authorization` / `invalid token` |

Green: all seven, 233s on a loaded host (each container start ~30s while S-4
ran beside it; 49s unloaded).

Red walk (scripted, each mutation marked `// MUTATION-APPLIED`, marker count
1 verified, `-count=1`, restored from a saved copy and `cmp`-checked, no
marker left in `internal/component/gnmi`):

| Mutation | Red subtest | Failure |
|----------|-------------|---------|
| `capabilities.go` encodings add JSON | capabilities | `supported encodings = [JSON_IETF JSON], want [JSON_IETF]` |
| `capabilities.go` `loadModels` keeps only the first module | capabilities | `gnmic decoded 1 models, the loader holds 13` |
| `get.go` `treeToTypedValue` leaf value suffixed | get, subscribe once | `update value = "192.0.2.1-mutated"` in both (shared producer) |
| `get.go` absent path answered InvalidArgument | get of an absent path | stderr does not name the NotFound status |
| `subscribe.go` final sync_response false | subscribe once | `second response sync_response:false is not sync_response true` |
| `set.go` update result op REPLACE | set | `Set result op = REPLACE, want UPDATE` |
| `server.go` `checkAuth` missing authorization returns nil | no token | `gnmic succeeded, want the refusal` |
| `server.go` token compare never mismatches | wrong token | `gnmic succeeded, want the refusal` |
| `server.go` `Serve` installs no TLS credentials | capabilities | `gnmic exited 1, want 0` (handshake) |

One equivalent mutation, recorded: blanking `ModelData.Organization` stayed
green, because no module the gnmi test binary loads declares an organization
(10 of 242 Ze YANG files do, none of the 13 loaded here); the model-set
assertion still compares organization when a module carries one.

Defect found, not fixed (does not block S-3): `Server.Get` never reads
`GetRequest.encoding`, so an unadvertised encoding is answered instead of
refused `Unimplemented` (gNMI specification Section 3.3.1). Journal row in
`plan/journal/silent-fall-through.md`.

Shared files with S-4: `labtest.go`, `labtest_test.go` and the labtest
section of `docs/architecture/testing/interop.md` carry both sessions' hunks.

## Phase 5, S-4 (flow-export), 2026-10-19

The `integration && linux` carrier
`internal/plugins/flowexport/interop_integration_linux_test.go` is replaced
(deleted, no-layering) by `internal/plugins/flowexport/collectors_docker_test.go`,
tag `docker` alone. The split: nothing in it needs the QEMU guest. It drives the
public encoders through `Sender` over UDP, with no namespace, privilege or
kernel feature, so the whole file moved and none of it stays `integration`. The
package stays in the guest population (`internal/le/test/qemu/alltests.go`
unchanged) through `producer_integration_linux_test.go`
(`TestFlowExportDaemonCounterGeneration`, which needs a namespace and a dummy
interface). `-flowexport-peer-dir`, the host-process collectors, the
`/proc` socket-ownership probe and the unprovisioned skip are gone.

Image: `test/interop-flowexport/Dockerfile.collectors` (nfdump v1.7.10 at
`9e47b47d`, sflowtool v6.11 at `c953cfe8`, unchanged pins), built by labtest as
`ze-flowexport-collectors:test`. Its old `ENTRYPOINT ["/flowexport.test"]` and
`CMD` (the test binary mounted into the image) are removed: the test runs on
the host now.

labtest gained (landed in `16051e7098`, carried by S-3): `Spec.Dockerfile`
(build through `Docker.Build` instead of pull, run-owned tag released after the
container), `Spec.ReadyInContainer` (a command whose exit 0 is ready, for a
UDP server a host probe cannot observe; exactly one of it and `Ready`),
`StartContainer` answering `Container{Address}` with `Container.Exec`; `Start`
is `StartContainer(...).Address`. Unit tests: `TestLabtestBuildsADockerfileImage`,
`TestLabtestFailsWhenTheBuildFails`, `TestLabtestReadyInContainer`, the
"two ready" validate case.

How the test runs a collector: entry point `sh`, which runs the collector as a
child, records its pid and on exit its status, then `sleep infinity`, so the
files survive the collector's exit. Ready = pid recorded and the port bound in
the container's `/proc/net/udp` (only the collector is in that namespace). Ze
sends to the published loopback port. nfdump runs per completed rotation file
by `docker exec`; sflowtool's `-j` output is read with `cat`. Stop = SIGTERM to
the recorded pid, exit within 3 s with status 0 or 143, then the final read.
Every assertion function (`assertExportNFDumpRecords`,
`assertExportSFlowDatagrams` and helpers) moved byte-identical.

Green: `TestFlowExportNFDumpInterop` (netflow9, ipfix) and
`TestFlowExportSFlowToolInterop`, 70.8 s with `ZE_REQUIRE_DOCKER=1`.
No `RFC requirement:` tag in the file, so no discrimination record; the walk
(`// MUTATION-APPLIED`, grep-verified, `-count=1`, restored from a pristine
copy and `cmp`-checked, no marker left):

| Mutation | Red | Failure | Stayed green |
|----------|-----|---------|--------------|
| `netflow9/flow_data.go` SrcAS +1 | `NFDump/netflow9` | `src_as=65552, want 65551` (and IPv6) | `NFDump/ipfix` |
| `ipfix/data.go` packet total drops OutPackets | `NFDump/ipfix` | `in_packets=4294967307, want 12884901912` | `NFDump/netflow9` |
| `sflow/counter.go` ifInOctets truncated to 32 bits | `SFlowTool` | `ifInOctets="5", want "4294967301"` | - |
| `sflow/flow_adapter.go` output port masked to 30 bits | `SFlowTool` | `outputPort="573855352", want "3795080824"` | - |

Restored: all three PASS (36.8 s).

Found: the first run hit the 900 s test timeout inside `docker build`, with no
output, because the daemon's pull of `debian:bookworm-slim` stalled (every
`docker pull`, alpine included, hung until the stalled build client was
stopped). The timed-out test binary left its `docker build` client running:
the wrapper's process runner does not end a child when the test binary dies.
Environmental, not reproduced since; not journaled as a code defect.

Owed gates (not run here): scoped lint of `internal/plugins/flowexport` and
`internal/le/interoplab/labtest` with `-tags docker` (`go vet -tags docker`
passed), `./le test unit all`. The `docker-peers` journal row in
`plan/journal/bound-too-small-for-its-own-burst.md` (a one-minute
`TestDockerBuildRetainsImageAcrossRetag` bound overrun while flow-export built
its image in parallel) concerns this population.

Landed: `00ad396fd0`. `./le test integration docker-peers` after it: code 0,
population gnmi (120.3 s), tacacs (36.1 s), interoplab (152.4 s),
interoplab/labtest (28.9 s), plugins/flowexport (118.4 s), plugins/ntp (50.2 s).

## Docker test bounds, 2026-10-19

Landed: `3681720e48`. `TestDockerBuildRetainsImageAcrossRetag` no longer sets
`Timeout: time.Minute` on its build or a 3-minute outer context. Its context is
`testDeadlineContext` (new, `internal/le/interoplab/deadline_docker_test.go`),
which ends 10 s before `t.Deadline()`. `TestKilledLabRunIsCleanedByNextStart`
had the same pattern: a fixed 5-minute kill timer around a child that builds,
and `-test.timeout=10m` on the child. It now kills the child at the parent's
deadline and hands the child that deadline. Every assertion is unchanged.
`interop.md` carries the sentence.

The producer, read: `Docker.Build` takes a non-zero `ImageBuild.Timeout` in
place of the machine bound and does not take the larger of the two
(`Pull` does, through `max`). So the one-minute field shortened the 90-minute
bound. "Only ever lengthens" in `interop.md` and in spec D-3 is a convention
that no code enforces. Code left unchanged: making the field lengthen-only
also changes how `BUILD_TIMEOUT` lowers a bound, so that is the owner's call.

Other docker tests checked (labtest, tacacs, ntp, gnmi, flowexport): none sets
a build or pull bound. labtest builds and pulls with the wrapper defaults
(`labtest/container.go`, `Pull(ctx, image, 0)`). What remains are
ready-probe and client-run bounds on containers already started or pulled:
`ReadyTimeout` 30 to 60 s, `gnmicTimeout` 60 s, flowexport's `peer.wait` 10 to
15 s. They are not build budgets and stayed green under docker-peers.

Evidence (`./le job run`, `ZE_REQUIRE_DOCKER=1`, load average about 6): the
retention test alone PASS 41.6 s, the orphan test alone PASS 26.5 s, and
afterwards `docker ps -a` and `docker network ls --filter label=ze.lab` empty.
`./le --name dockerpeers test integration docker-peers`: code 0, gnmi
111.8 s, tacacs 37.1 s, interoplab 145.0 s, labtest 31.8 s, flowexport
117.8 s, ntp 53.5 s. The red is the journal row's run before the fix. It was
not re-forced: forcing it would put the one-minute bound back under a parallel
population.

Gates OWED. This agent's contract and `ai/rules/commands.md` forbid a
subagent to run any command over five minutes. The main thread or a fresh
`/ze-verify` agent runs: `./le go lint run scope "<the spec's packages>"`,
covering the docker and live flavors (`internal/le/go/lint/matrix.go`),
then `./le test unit all` and `./le test functional gating`. Already run:
`go vet -tags docker ./internal/le/interoplab/` clean, and the post-write lint
hook on each edited file.

2026-10-19, build bound: `Docker.Build` now runs under max(`ImageBuild.Timeout`, machine bound), so the field only lengthens it, as `Pull` does (D-3); no non-test caller sets the field.
Proof: `TestDockerBuildUsesDeclaredTimeout` red on the old code (1m field gave 1m, want 1h30m), green after; interop.md "Running" and the field comment state the rule once. Gates owed: lint scope on `internal/le/interoplab`.

## Independent-review fixes, 2026-10-19

| Finding | Commit | Proof (red on old code, green after) |
|---------|--------|--------------------------------------|
| BLOCKER R-1/AC-7: teardown removed names `claimName` refused | `62512fcace` | `TestRunScenarioRefusedClaimRemovesNothingForeign` (old: `rm -f lab-peer`, `network rm lab-net` after a refusal); `TestRunScenarioTakenNameIsNotTornDown` |
| user-facing-errors: `ZE_<LAB>_INTEROP_SUFFIX` placeholder | `4c41912146` | `TestPreCleanRefusalNamesTheRealSuffixVariable` |
| AC-9: hand-started container from a lab image swept | `008afd8b65` | `TestCleanupOrphansSkipsContainerWithInheritedImageLabels` |
| R-1: gone judged on machine id and boot id only | `69dd08092b` | seven new `TestCleanupOrphansVerdictTable` rows |

Shape of each: teardown records a container or network only after its create
(a failed `docker run` counts unless the daemon refused the name as taken), and
`CreateNetwork` now refuses an existing name rather than adopt it, which also
covers `perfrunner`. Every lab Suite passes `Environment.SuffixVariable`. Every
create labels `ze.lab.kind`, `ze.lab.uid`, `ze.lab.os` and, on Linux,
`ze.lab.pid-ns`. Resources labeled before these commits carry no kind, uid or
os label, so the sweep skips them as malformed (the safe direction); remove
those by hand once.

Residual, not settled: a live clone whose owner started before this clone
booted still reads as gone. No client-side fact tells that apart from a reboot.

Reds seen, not caused here (scenario population): `interoplab/bgp`
`TestCheckerPopulationMatchesProducer` and `TestAnnouncementPlanOrderAndTimingFixture`,
`interoplab/pppoe` `TestPPPoELabPeersRunWithoutPrivilege`.

Gates owed (over five minutes, not run here): `./le go lint run scope
"internal/le/interoplab/..."` in both the docker and live flavors, `./le test unit
all`, and a `ZE_REQUIRE_DOCKER=1` run of `TestKilledLabRunIsCleanedByNextStart`,
which reads the real pid namespace and boot time.

## 2026-10-19: review findings on the docker verbs, buildtag, docker route, pinning

| Finding | Commit | Red before, green after |
|---------|--------|-------------------------|
| ISSUE AC-8/AC-22: docker-peers and live-rpki ran `go test` on `context.Background` | `d83fb6a817` | `TestDockerChildStopsOnSignal`: back after 30 s, code 0, for SIGINT and SIGTERM; now the child's process group is killed (`gaterun.StreamContext`/`RunContext`), and `TestStreamContextKillsTheGroupOnCancel` proves the grandchild dies |
| buildtag counted negated terms | `976d74655a` | `TestPackagesSelectOnlyFilesThatNeedTheTag` and the extended `TestDockerTagNeverJoinsIntegration` fixture: `!docker`, `integration && !docker` and `docker \|\| integration` were taken or refused; now `TestFile.Needs` evaluates the constraint with `go/build/constraint` |
| AC-16 gate missed const chains, package vars, renamed packages | `e619f62e15` | `TestNoDockerExecOutsideInteroplab`: four new violators unreported; now resolved through name chains and package clauses, and a docker-holding package var is refused at its declaration |
| Pinning, offline labtest | `7edc3e7e89` | `TestLabtestPullsAPinnedImageOnlyWhenAbsent`, `TestLabtestFailsWhenThePinnedInspectFails`; chrony and `debian:bookworm-slim` pinned by digest; `ZE_REQUIRE_DOCKER=1` docker-tag runs of `internal/plugins/ntp` (Chrony) and `internal/plugins/flowexport` pass |

Unsettled: `docker || integration` now needs neither tag, so it joins neither
population and the overlap test does not refuse it. Both builds compile it, the
guest skips it, and docker-peers runs it only when another file brings its
package in. The owner decides whether that shape is refused too.

Gates owed (over five minutes, not run here): `./le go lint run` over
`internal/le/gaterun`, `internal/le/test/...`, `internal/le/arch/tier` and
`internal/le/interoplab/labtest`; `./le test unit all`.

M-3 red `TestDockerDeploymentRefusesMissingKernelFeature` (2026-10-19): a false positive. Its `strings.Contains(record, "build")` matched `go-build` in the `ze.lab.purpose` label, which quotes os.Args. Under a cacheable `go test` that is `-test.testlogfile=.../go-build.../testlog.txt`. The label is one argv element, so real runs never pass `-test.` flags to docker, and the preflight running before the kernel check is the documented order (interop.md, `Suite.Run`). The test now records argv split on 0x1f and judges `build` by subcommand (uncommitted in `dockerkernel_test.go`).
Open real defect: the l2tp, pppoe, radiussubscriber and rsvpte preflights put `--name ze-*-preflight-<suffix>` in `OneShotContainer.Arguments` on top of RunOneShot's `--name ze-oneshot-*`. Docker keeps the last one, so a canceled preflight's removal-by-name misses. The fix is to drop the suffix parameter and the name from those four producers. The new exactly-one-`--name` assertion stays red until it lands. In `pppoe.RunAt`, `preflight()` must go back to `preflight(options.Suffix)`: the permission classifier refused the restore edit.
Overlap guard (2026-10-19): `TestDockerTagNeverJoinsIntegration` now refuses any `_test.go` whose constraint NAMES both `docker` and `integration` (`buildtag.TestFile.Names`, negation included), not only one that needs both; red observed on the new fixtures (`docker || integration`, `(docker && linux) || integration`, `!docker && integration`, `integration && !docker`), green after, the real tree holds no such file.
Gates owed by the main thread for that change: `./le go lint run` over `internal/le/test/buildtag` and `internal/le/test/qemu`; qemu-integration.md "The docker population" carries the new rule.

## 2026-10-19: re-review findings (docker route locals, G-2 for docker-peers, spec record)

| Finding | Commit | Red before, green after |
|---------|--------|-------------------------|
| ISSUE AC-16: a function-scoped `const`, `:=`, `=` or `var` holding docker passed the gate | `38739450f3` | `TestNoDockerExecOutsideInteroplab`: five new sites unreported (local const, `:=`, `=`, `var` chained to `interoplab.DockerExecutable`, an implicitly repeated const spec) and a local shadowing a package docker const falsely refused; now each top-level declaration's local assignments are followed before package names. `./le arch tier check` green on the real tree; module-tiers.md "The docker route" updated |
| G-2: Ctrl-C of docker-peers / live-rpki left the killed tests' containers to the next sweep | `dda6f2de0c` | `TestDockerChildStopsOnSignal` (now asserts one non-dry sweep on an uncanceled context), `TestDockerChildExitRunsNoSweep`, `TestSignalSweepReports`: red on the signature, green after. `runRequiringDocker` runs `CleanupOrphans` after a signalled exit, bounded at 60 s, and prints its summary; qemu-integration.md updated |
| Spec record | this commit | Data Flow step 4 matches the code (only `Suite.Run` sweeps at start, plus the cleanup verb and the docker-peers post-signal sweep); A-7 and A-9 confirmed; A-2, A-3, A-5 (Colima, Docker Desktop) open for the owner's Mac; A-5's guest dockerd and A-8 open but not Mac items (a `docker-lab` run over five minutes; the first nightly run after an owner-ordered push); the TestDockerTagNeverJoinsIntegration row names the names-both rule |

`TestKilledLabRunIsCleanedByNextStart`, run alone 2026-10-19 07:18 UTC at
`dda6f2de0c` with `ZE_REQUIRE_DOCKER=1`, tag docker: PASS in 65.8 s (3 removed
/ 3 kept, then 3 removed / 0 kept).

Known window in the post-signal sweep: a SIGKILLed test binary is judged by
`procscan.Lookup` reading `/proc/<pid>`, and until init reaps it a zombie
still has that directory, so a resource can be reported "kept, owner still
running" by this sweep and removed by the next one. Not observed; noted, not
fixed (the judge is in `orphans.go`, which another agent owns).

Gates owed (not run here): `./le go lint run` over `internal/le/arch/tier` and
`internal/le/test/integration`; `./le test unit all`.

## 2026-10-19: re-review findings (own-run reclaim, sweep error, teardown recording, free-form args)

| Finding | Commit | Red before, green after |
|---------|--------|-------------------------|
| ISSUE: claimName refused this run's own leftover ("stop pid <self>") | `2f74338d3f` | `TestClaimNameReclaimsThisRunsOwnLeftover`: own labels refused, red; now `ownerIdentity.created` (every owner label equal, run id included) removes it; another run id or uid is still refused. The preClean fixtures standing for another live run now carry run id `run-other` |
| ISSUE: pre-run sweep listing failure was a bare docker error | `055e1e91a2` | `TestSuiteRunOrphanSweepFailureNamesWhatWhyAndFix`: red on "pre-run", "Fix:", the cleanup command; green after |
| NOTE: teardown rm -f'd a name docker never ran; perfrunner recorded before RunDetached and ignored name-taken | `075674363e` | `TestRunScenarioPreRunFailureRemovesNoContainer` (Host 0) and `TestPerfStartContainerRecordsOnlyWhatItMayHaveCreated/name_taken`: red, green after `issuedRunError` + `ContainerMayExist`; perf `removeContainer` now leaves an unrecorded name alone |
| NOTE: free-form args could carry `--name` or `ze.lab.*` labels | `8f6d5722eb` | `TestFreeArgumentsRefuseNameAndOwnerLabels`: nine spellings over PeerConfig and OneShotContainer, red, green. `DetachedContainer` has no free-form docker-argument field |
| Docs: tag removed "by the listing's id" | `8c112f5591` | interop.md: tag released by name, only a `ze-interop-retained` tag a candidate; plus the four behaviors above |

Unrelated reds seen in scoped runs, not caused here: `interoplab/bgp`
`TestCheckerPopulationMatchesProducer` (bgp-vpn-frr fallback),
`TestAnnouncementPlanOrderAndTimingFixture` (digest), and `interoplab/pppoe`
`TestPPPoELabPeersRunWithoutPrivilege` (ipv6cp-missing-option has no checker).

Gates owed (not run here): `./le go lint run` over `internal/le/interoplab` and
`internal/test/perfrunner`; `./le test unit all`.

Zombie owners (commit "procscan: an exited, unreaped process is absent"): `procscan.Scan`/`Lookup` now read a `Z`/`X` pid as absent on /proc and ps (`state=` column); red then green on `TestAnUnreapedChildIsAbsent` (real unreaped `true` child), `TestParseProcStatReadsTheExitedStates`, `TestParsePSLeavesOutExitedProcesses`; callers session, scratch, interoplab unchanged and green.
Gates owed for it: scoped `./le go lint run` over `internal/le/procscan` (linux and darwin flavors), `./le test unit all`.

Wiring vs tier contradiction (commit "doc wiring: a test-only package's export needs no production reference"): the test-only list moved (not copied) from `archtier.TestOnlyPackages` to the leaf `internal/le/arch/testonly` (`Packages`, `Owner`); `test-only-imports` and `checkWiring` both read it, so `labtest.TCPReady` no longer reports. Red then green on `TestATestOnlyPackageExportNeedsNoProductionReference` (the same unreferenced export is exempt in labtest, still reported in an ordinary package).
Gates owed for it: scoped `./le go lint run` over `internal/le/arch/...` and `internal/le/doc/wiring`, `./le test unit all`.

Final review notes (commit "lab: sweep after the killed group empties; refuse -l in a cluster"): (1) `Lab.preClean` comment now says a same-name resource also goes when every owner label equals this run's own (`ownerIdentity.created`), matching `claimName`. (2) `gaterun.StreamContext`/`RunContext` answer the child's `ProcessGroup`; `sweepAfterSignal` calls `ProcessGroup.AwaitExit` (polls `kill(-group, 0)` for ESRCH, 10 s `groupExitTimeout` inside the 60 s sweep bound) before sweeping, and past the bound sweeps anyway with the pids left (from `ps -A -o pid=,pgid=`) and the cleanup fix in the summary; red (await disabled) then green on `TestSignalSweepWaitsForTheGroup`, `TestSignalSweepNamesMembersLeft` (real group outliving its leader), plus `TestAwaitExit*` in gaterun. (3) `labelFlagValue` walks a short-flag cluster as pflag does (past `docker run`'s value-free `-d -i -P -q -t`, stop at the first value-taking flag), so `-dl <v>`, `-dl<v>`, `-dl=<v>`, `-itl <v>`, `-dPqtil<v>` are refused and `-el<v>` (env) passes; `--name` has no short form (`docker run --help`); red on the five cluster spellings then green in `TestFreeArgumentsRefuseNameAndOwnerLabels`. Pages: `interop.md` (cluster spellings), `qemu-integration.md` (group wait).
Gates owed for it: scoped `./le go lint run` over `internal/le/gaterun`, `internal/le/interoplab`, `internal/le/test/integration` (linux and darwin flavors), `./le test unit all`.

## 2026-10-19: closure attempt stopped

`38ccef54f5` ("gaterun: a group that empties during its listing is empty"): `ProcessGroup.leftError` asks `kill(-group, 0)` again after the `ps` listing and answers nil for an emptied group; red then green on `TestAwaitExitGroupEmptiedDuringTheListing` (fake `ps` that kills the member). It is not independently reviewed yet. Gates owed: scoped `./le go lint run` over `internal/le/gaterun`, `./le test unit all`.
`/ze-close` did not close the spec. The spec's "Closure status (2026-10-19): blocked" section names each blocker and its rule: A-2, A-3, A-5 and A-8 unvalidated, the first nightly run id (AC-25), Work Not Done with no owning spec, no hash-pinned review artifact, and the unreviewed `38ccef54f5`. Goal Validation, Work Not Done, Review Gate and Assumptions Resolved are filled there. This handover stays live until the spec closes.
Review of `38ccef54f5` acted on in the follow-up commit: `TestAwaitExitGroupPsListsNoMember` (fake `ps` that lists nothing) asserts the exact "members that ps did not list" error, red with that branch returning nil and green restored; `TestAwaitExitGroupEmptiedDuringTheListing` checks the staged race before its nil assertion; `sweepAfterSignal` now puts the group error after a colon instead of inside parentheses. Gates still owed: scoped `./le go lint run` over `internal/le/gaterun` and `internal/le/test/integration`, `./le test unit all`.

## Real Ctrl-C proofs (2026-10-19)

Owner authorisation 2026-10-19 ("3 yes"). Linux host, KVM, no `/dev/vhost-net`. Each run was started as its own process-group leader (`setsid`) and sent SIGINT to the whole group (`kill -INT -<pgid>`), as a terminal Ctrl-C does, once a `docker events --filter label=ze.lab` stream showed its resource (or after a fixed delay where none is host-visible). One run at a time. Leftovers were read with `docker ps -a`/`docker network ls` on `label=ze.lab`, `docker images --filter label=ze.lab.run`, and the `ze-interop-retained` tags (30 present, all unlabeled and older than the owner labels, none of these runs'). Then `./le test integration cleanup dry`.

| # | Verb | Run label | Signal landed in | Signal to exit | Exit | Final message | Leftovers of its run | cleanup dry |
|---|------|-----------|------------------|----------------|------|---------------|----------------------|-------------|
| 1a | `./le test deployment l2tp-test` | `5F2GRJEWTDZOCROEQSSLU3K27E` | the kernel-probe one-shot (`ze-oneshot-bbt6...`, Up) | 5.8 s | 1 | `error: docker image inspect failed (exit 124): context canceled` then `FAIL: xl2tpd session establishment not observed` | none (one-shot removed) | 0 to remove, 0 kept |
| 1b | same | `QYRH6MSC2E2A2HTVHDGBXGT7M5` | `apk add` exec in `ze-l2tp-evidence-2351625` | 1.6 s | 1 | `error: docker exec ze-l2tp-evidence-2351625 failed (exit 124): context canceled` then `FAIL: xl2tpd session establishment not observed` | none (events: kill, die, destroy) | 0 to remove, 0 kept |
| 2 | `./le test deployment vpp-test` | `A3RDNVXVCSQJQ3P4XLEETPVX2Q` | not sent: the run ended first | n/a | 1 | `error: failed to start the VPP evidence container: docker run -d failed (exit 127): ... adding custom device "/dev/vhost-net": no such file or directory` | DEFECT: `ze-vpp-evidence-2578373` left in `Created` | would remove it (owner exited) |
| 3 | `./le test deployment vpp-iface-test` | `TJ26MSP4PX27RGUTE7I2T3HQ2W` | after the run's own failed start | n/a | 1 | same `/dev/vhost-net` refusal | DEFECT: `ze-vpp-iface-2608404` left in `Created` | would remove it (owner exited) |
| 3' | same, fixed build (`./le --name ctrlc`) | `MYCSMVH4PRGX45W3QY2S6TN3YX` | the deferred `docker rm -f` of the failed start | 0.07 s | 1 | refusal, then `FAIL: container ze-vpp-iface-2636940 was not removed: docker rm -f failed (exit 130)` | none: `Dead` at exit, gone moments later (the daemon finished the removal) | 0 to remove, 0 kept |
| 4 | `./le verify evidence release-candidate` | none created | not sent: refused before any container | n/a | 1 | `error: clean release-candidate evidence requires a clean git worktree` listing other sessions' 14 dirty paths | none | n/a |
| 5 | `./le site terminal-demo image-build` | no labeled resource before the signal | the `docker build` (8 s in) | 0.1 s | 1 | `error: build the renderer image ze-terminal-demo-render-all:...: docker build --label failed (exit 130): ERROR: failed to build: failed to solve: Canceled: context canceled` | none (no image, no container) | 0 to remove, 0 kept |
| 6 | `./le test qemu docker-lab lab "test integration interop-ipsec" env "IPSEC_INTEROP_SCENARIO=mobike-initiator"` | none on the host (Docker runs in the guest) | the lab over ssh, after `verdict pass` (240 s in) | 0.31 s | 1 | `error: command ssh exceeded its deadline: context canceled`, `QEMU VM: INVALID` | none: no `qemu-system` process after exit | 0 to remove, 0 kept |
| 6' | same, fixed build | none on the host | same point (250 s in) | not captured | 1 | `error: command ssh was canceled before it finished, by an interrupt or by the run stopping: context canceled`, `QEMU VM: INVALID` | none: no `qemu-system` process | 0 to remove, 0 kept |

The two `Created` leftovers (rows 2, 3) were removed with `./le test integration cleanup` after recording ("2 removed").

Fixed here:
- `081d93a502` deployment: the proofs registered their container removal only after a successful `docker run -d`, and the daemon creates the container before it refuses the start. `startProofContainer` now answers the removal whenever `interoplab.ContainerMayExist` holds (l2tp-test, vpp-test, vpp-iface-test, the SRv6 integration test). `TestFailedProofStartStillRemovesItsContainer` red then green. The same gap covered a Ctrl-C during `docker run -d`.
- `4138a91e01` qemu: `runHostCommand` called every context error a deadline. `TestHostCommandNamesCancelAndDeadlineApart` red then green; row 6' is the real run.

Open defects, with a fix plan, not fixed here:
- D-a. The docker client runs in le's own process group (`systemProcessRunner.Run`, `internal/le/interoplab/process.go`, `exec.CommandContext` with no `Setpgid`), so a terminal Ctrl-C also kills the docker CLI in flight. A Ctrl-C (or a second Ctrl-C) that lands during teardown kills the `docker rm -f` client (row 3', exit 130), and the report says the container was not removed although the daemon removes it anyway. Plan: start docker children in their own process group, so only le's signal root decides what stops. Check `ExecAttached` first: none of its callers may read the terminal, because a background group that reads the tty is stopped with SIGTTIN. Test: a fixture child that records its pgid differs from le's.
- D-b. An interrupted deployment proof prints a verdict it never reached. `runL2TP` (`internal/le/test/deployment/actions.go`) returns the report together with the step error, and `L2TPReport.Text` then prints `FAIL: xl2tpd session establishment not observed`. The function's own doc says "a step that could not be performed is an error and answers 1 with no verdict". The step error also prints `exit 124` (`timeoutExitCode`, set for any `runCtx.Err()` in `process.go`) for a cancel that was no timeout. Plan: render no verdict line when the run returned an error (keep the container-removal line), and set the exit code for a canceled context to 130 (or omit it), so the message reads as an interrupt. The VPP reports need the same check.
- D-c. `./le site terminal-demo image-build` has no signal root (`runImageBuild`, `internal/le/site/terminaldemo/actions.go`, uses `context.Background()`). It stopped cleanly only because the docker client got the terminal's SIGINT itself. A SIGTERM to le alone leaves the build running. Plan: wrap it in the file's existing `signalRooted`.
- D-d. Not proven on this host: vpp-test and vpp-iface-test past the container start (no `/dev/vhost-net`), and release-candidate (needs a clean worktree, which this shared checkout never has: run it from a clean clone or worktree).

Gates owed (not run here): scoped `./le go lint run` over `internal/le/test/deployment` and `internal/le/test/qemu`, and `./le test unit all`.

### D-a, D-b, D-c fixed, and the re-run (2026-10-19)

- D-c `e980769d62`: `runImageBuild` is wrapped in `signalRooted`. `TestImageBuildCancelsOnSignal` red (SIGINT and SIGTERM let the build finish, exit 0), then green.
- D-b `dac71530c3`: a proof run that returned an error answers `proofNotReached` (no verdict, only a failed removal, exit 1) for l2tp-test, vpp-iface-test and vpp-test. Only a deadline answers exit 124. A cancel answers no exit code and reads "was canceled before it finished, by an interrupt or by the run stopping". `TestInterruptedProofPrintsNoVerdict` and `TestProcessRunnerNamesCancelAndDeadlineApart` were red, then green.
- D-a `41397b4328`: every docker child heads its own process group (`Setpgid`, and the cancel kills the group). A child with a terminal `*os.File` stream stays in the group of le (`sharesTerminal`, because of SIGTTIN). `TestDockerChildRunsInItsOwnProcessGroup` was red, then green. Teardown already ran on `context.WithoutCancel` (RunOneShot, Lab.cleanup, deployment removeContainer).

Re-run of row 1 on the fixed build (`./le --name ctrlc test deployment l2tp-test`, run label `M7Z36TTL24YVY52KSNGPXYEQ6I`, same setsid plus group SIGINT method). The signal arrived 3 s after the kernel-probe one-shot started, during `docker run -d` of `ze-l2tp-evidence-2866412`. Signal to exit took 0.06 s, exit 1. The only output line was `error: docker run -d was canceled before it finished, by an interrupt or by the run stopping: context canceled`, with no FAIL or OK verdict and no `exit 124`. The one-shot was removed (events: die, destroy). No group member was left after exit. A different interop run was active on the host at the same time (pid 2855665, `le test integration interop`); its 6 retained tags are not from this run.

- D-e, fixed on 2026-10-20 (see "D-e fixed, and the re-run" below). The canceled `docker run -d` left `ze-l2tp-evidence-2866412` in `Created` state (daemon created it at 22:43:44.11Z, the second the signal arrived), and the report showed no removal failure. Cancelling the client does not cancel the create in the daemon. The deferred `docker rm -f` ran right away (le exited 0.06 s after the signal), apparently before the container existed, and its answer counted as removed. `./le test integration cleanup dry` then said "1 to remove" (owner exited), and `./le test integration cleanup` removed it ("1 removed"). Plan: after a start that was canceled or failed, the removal must confirm absence again once the daemon has settled. One way: re-inspect by name for a short bounded window, or remove by the owner label of the run instead of by name. A fixture test then answers `run -d` with a cancel and makes the container appear after the first `rm -f`. Row 1b of the earlier table had the same race (it landed later, in an exec), so this does not come from D-a.

Gates owed: scoped `./le go lint run` over `internal/le/interoplab`, `internal/le/test/deployment` and `internal/le/site/terminaldemo`, then `./le test unit all`. Also owed: `./le verify current` over e980769d62..41397b4328.

### D-e fixed, and the re-run (2026-10-20)

Cause, verified at the producer. `RemoveContainer` ran one `docker rm -f` and took any non-refusal as removed. Docker 29.9 on this host answers `docker rm -f` of an absent name with exit 0 and no output (checked by hand: `docker rm -f <missing>` printed nothing, exit 0), and prints the name only when it removed a container. So the deferred removal, issued before the daemon finished the create it had already accepted, "succeeded" on nothing. The earlier guess (a "No such container" answer read as removed) was the older Docker dialect, and was not what this host does.

- `d670f1e2e8`: every start that issues a `docker run` and fails or is canceled marks its name (`RunDetached`, a lab peer's `runContainer`, `RunOneShot`; `startsettle.go`). `RemoveContainer` of a marked name does not accept a first absent answer. It runs `docker rm -f` again every `createSettleInterval` until `createSettleWindow` (5 s) ends, and removes the container if it appears. One that appears and cannot be removed, or a wait that is stopped, is an error naming the container and `./le test integration cleanup`. A removal after a successful start still takes one `rm -f`. `TestCanceledStartRemovesAContainerThatAppearsLate` and `TestCanceledStartNamesAnAppearedContainerItCannotRemove` (all three start paths) were red, then green, but only for the "No such container" dialect.
- `3a1fe640ac`: the first real re-run on `d670f1e2e8` (attempt 1 below) still left the container, which exposed the silent dialect. `forceRemoveContainer` now reads a success with no output as absent. The tests run every start path under both dialects. The silent-exit-0 cases were red (container left behind, removal answered success), then green. Four fixtures that answered `rm -f` of a present container with empty output (`oneshot_test`, deployment `signalroot_test`, perfrunner `run_test`, `verify/evidence`) now print the name, as Docker does. `docs/architecture/testing/interop.md` describes both answers and the window.

Re-run, `./le --name ctrlc test deployment l2tp-test` under `setsid`, with group SIGINT sent right after the kernel probe one-shot's `destroy` event, so it lands during `docker run -d` of the evidence container:

| Attempt | Build | Run label | Signal landed in | Signal to exit | Final message | Leftovers of its run | cleanup dry |
|---------|-------|-----------|------------------|----------------|---------------|----------------------|-------------|
| 1 | `d670f1e2e8` | `L6Z4N7DU43UOLL2S7AADDLDAG7` | `docker run -d` of `ze-l2tp-evidence-2931514` | 0.05 s | `error: docker run -d was canceled before it finished, ...` | DEFECT: `ze-l2tp-evidence-2931514` left (the silent `rm -f` answer) | "would remove container ze-l2tp-evidence-2931514: its owner, pid 2931514, has exited"; removed by hand by name after recording (this run created it) |
| 2 | `3a1fe640ac` | `H7VMAIZ3NGRNNJEA5ORBGIEB3W` | not sent: the run ended first | n/a | the kernel probe one-shot did not finish within 2m0s under host load 107, then `docker rm -f failed (exit 1): ... removal of container ze-oneshot-... is already in progress` | none | not run (no labeled resource left) |
| 3 | `3a1fe640ac` | `GWUT6TRHJATI5DJCCQENKT7BO5` | `docker run -d` of the evidence container | 5.9 s (the settle window) | only `error: docker run -d was canceled before it finished, by an interrupt or by the run stopping: context canceled`, with no verdict and no removal failure | none: no labeled container, network or run-labeled image | `0 to remove, 0 kept, 0 failed, 0 unknown` |

Attempt 3 proves G-2 for this window: nothing of the run was left. The events stream shows no `create` for the evidence container in attempt 3, so the daemon did not create it within the window there, and the "appeared late, removed" branch is proven by the fixture tests, not by this real run.

- D-f, OPEN (new, seen in attempt 2). A one-shot whose own bound ended reported `docker rm -f failed (exit 1): ... removal of container ... is already in progress`, while the events stream shows that the daemon destroyed it. `--rm` had already started the removal. `confirmRemoval` inspects once, sees the container still there while its removal runs, and reports a failure. Plan: treat "removal ... is already in progress" like an absent answer that needs confirming, and look again over the same bounded window until the container is gone. Test: a fixture that answers "already in progress" and then makes the inspect absent.

Gates owed (not run here): scoped `./le go lint run` over `internal/le/interoplab`, `internal/le/test/deployment`, `internal/test/perfrunner` and `internal/le/verify/evidence`, then `./le test unit all` and `./le verify current` over `d670f1e2e8..3a1fe640ac`.

### D-f fixed (2026-10-20)

- `3cdd0c3324`. Cause, verified at the producer: `forceRemoveContainer` sent the daemon's "removal of container <name> is already in progress" refusal to `confirmRemoval`, which inspects once and reports the removal failed if the container is still there. `--rm` was still removing it. `strings /usr/bin/dockerd` (29.9.0) holds one "already in progress" answer, `removal of container %s is already in progress`, so it is container-only, and networks and images keep the single inspect. That refusal now goes to `Docker.awaitRemoval` (`startsettle.go`). It inspects at once, then every `createSettleInterval` until `createSettleWindow` ends. Gone counts as removed, a failed inspect is unknown, and a container still present is `pendingRemovalError`, which names the container and `./le test integration cleanup`. `removeSettlingContainer` returns that error unwrapped, so the fix appears once. The `settlingDaemon` stuck fixture now refuses with a could-not-kill answer, and its in-progress variant is the new `inProgress` flag. Red before the fix, then green: `TestOneShotRemovalAlreadyInProgressIsNotAFailure` (the exact false report), `TestRemovalAlreadyInProgressThatNeverEndsNamesTheCleanup`, and `TestCanceledStartWithARemovalStuckInProgressNamesTheCleanupOnce` (two fixes before the unwrap). `go test -race` and `go vet` of `internal/le/interoplab` pass. `interop.md` "Crash-safe teardown" describes the wait.
- Gates owed: scoped `./le go lint run` over `internal/le/interoplab`, then `./le test unit all` and `./le verify current` over `3cdd0c3324`. A real re-run of attempt 2 (a one-shot whose own bound ends) is not done.
