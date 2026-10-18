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
