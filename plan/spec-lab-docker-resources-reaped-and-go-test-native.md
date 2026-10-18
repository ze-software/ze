# Spec: lab-docker-resources-reaped-and-go-test-native

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1/5 |
| Handoff | - |
| Updated | 2026-10-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner asked about Testcontainers (testcontainers.com). Ze does NOT adopt
testcontainers-go: it would put a second Docker orchestrator beside
`internal/le/interoplab`, which already wraps the docker CLI (networks,
run/exec/logs/pause/signal, readiness waits, image build and retention,
AppArmor profiles, the kernel-feature probe, Colima socket discovery in
`internal/le/dockerhost`). `ai/rules/no-layering.md` forbids keeping both.

Instead Ze builds the two things Testcontainers offers into `interoplab`
itself. Owner, 2026-10-18: "Add these improvements to our solution, then make
sure it is being used by our testing where it should and must."

| ID | Goal |
|----|------|
| G-1 | Crash-safe cleanup (the Ryuk equivalent): every container, network, one-shot container and run-owned image tag a Ze lab run creates carries labels naming its owner, and a sweep removes exactly the resources whose owner is provably gone, at lab start and through an explicit `./le` command. It never touches an unlabeled resource, a resource owned by a live process, or one whose owner it cannot judge, and it says which it skipped and why |
| G-2 | Clean interruption: a Ctrl-C or SIGTERM of a lab run runs the teardown the run already owns, so only SIGKILL, a lost machine or a panic leaves anything for G-1 |
| G-3 | `go test`-native containers: a package `_test.go` can start one real server container through the `interoplab` wrapper, with a readiness wait, its logs on failure, `t.Cleanup` teardown and the G-1 labels, under a build tag of its own that a registered `./le` verb and a nightly job actually run, so no such test exists without a home that gates it |

G-1 and G-2 are the container form of the lifetime model
`docs/architecture/testing/runner-architecture.md` ("How a suite's child
processes end") already requires of every suite child: a `defer` beside the
start, context cancellation, and `Pdeathsig` for the runner dying outright.
The labs already have the `defer`. G-2 adds the cancellation route the page
names as "the gap that hides best", a runner with no signal handler. `Pdeathsig`
cannot reach a container, because the daemon, not the runner, is its parent,
so G-1's labels plus the next-start sweep stand in for it.
| G-4 | Adoption: every site in the inventory below that starts Docker resources goes through the `interoplab` wrapper (MUST rows, own docker invocation deleted, not wrapped), and the package tests the owner selects from the SHOULD rows get a real peer |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - "How It Works", "Running", "Debugging Failures"
  → Constraint: step 5 of "How It Works" promises teardown "including after setup or checker failure"; the page is silent on a killed run, which is the gap this spec investigated at the producer (Current Behavior, verdict V-1). The page gets a "Crash-safe teardown" section in the phase that lands G-1, not at closure.
  → Constraint: the shared orchestrator does not read container logs; each suite's checker adds them (`checkerFailure`). The `labtest` helper (G-3) is the first caller that dumps logs itself, on `t.Failed()`, and the page's Debugging table gets its row.
  → Decision: `NO_BUILD=1` reuses images by the shared cache tag (`prepareImages` pulls `build.Tag`); the sweep never removes a cache tag, only the run-owned `ze-interop-retained:` tag, so `NO_BUILD` keeps working after a cleanup.
- [ ] `docs/architecture/module-tiers.md` - placement and the production-importer exclusions
  → Constraint: "Test files, test/chaos/performance harnesses, `internal/le` and `bin` tools are not production importers." A `_test.go` under `internal/component` or `internal/plugins` may import `internal/le/interoplab/...` (precedent: `internal/plugins/vrrp/lab_profile_linux_test.go`). No gate today refuses a NON-test file outside `internal/le` importing `internal/le/...`; AC-17 adds that refusal for the helper package.
- [ ] `docs/contributing/testing.md` - "Interop and integration tests"
  → Constraint: Docker-needing tests are outside the normal cycle and run behind a build tag (`integration`, or `live` for internet-needing ones) through `./le test integration <verb>`.
- [ ] `docs/architecture/testing/qemu-integration.md` - "Docker labs in the Ze-kernel guest", "Build Tags", "Graceful Degradation", "Registering a New Package", "Existing Integration Test Packages"
  → Decision: on macOS and in the nightly, labs run inside a throwaway QEMU guest with its own dockerd (`./le test qemu docker-lab`), so the guest's resources die with the guest; the sweep running inside the guest is harmless and needs no special case.
  → Constraint: the `integration` tag means "needs kernel capabilities, runs inside the QEMU guest through `./le test qemu all-tests`", and that guest has no Docker. `integrationPackages` in `internal/le/test/qemu/alltests.go` is the population; `TestEveryIntegrationPackageIsNamed` derives every `integration`-tagged package from the tree and fails on one missing from it unless `excludedIntegrationPackages` gives a reason. `internal/le/interoplab` and `internal/le/test/deployment` sit in that exclusion map with "run through ./le job run as documented": no verb and no nightly runs them. A `labtest` test tagged `integration` would therefore skip in the guest forever or be excluded and run nowhere. → Decision: `labtest` tests take a tag of their own, `docker` (Q-9), never combined with `integration`.
  → Constraint ("Graceful Degradation"): a missing capability is a `t.Skip`, not a `t.Fatal`, so a file stays usable on hosts without it. → Decision: `labtest` skips when the `docker` binary is absent, unless the run sets `ZE_REQUIRE_DOCKER=1`, which the registered verb always sets; under it a missing binary fails. An ad hoc `go test -tags docker` on a laptop skips; the gate cannot.
  → Constraint ("Registering a New Package"): one population, one native action, no second lifecycle. → Decision: the `docker` population is derived from the tree at run time by the verb itself (no list to register into), with the tree scanner moved out of `integration_coverage_test.go` into a leaf both use.
- [ ] `docs/architecture/testing/runner-architecture.md` - "How a suite's child processes end"
  → Constraint: three routes end a child (`defer`, context cancellation, `Pdeathsig`) and a suite owes all three; "A runner with no signal handler is the gap that hides best". → Decision: G-2 adds the cancellation route with the shape the repo already uses, `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` at the entry root (`guestContext` in `internal/le/test/qemu/guestlabs.go`, `internal/le/test/stressrepro/run.go`, `internal/le/test/deployment/mobike.go`); no new helper. G-1 is the stand-in for `Pdeathsig`, which a daemon-owned container never receives. The page gains one paragraph saying so.
- [ ] `docs/architecture/testing/ci-workflows.md` - "Where each suite runs"
  → Constraint: every Docker lab runs in `evidence-nightly.yml` inside `./le test qemu docker-lab`, because the hosted runner kernel fails the staged ze's Docker kernel check. `./le test integration live-rpki` has no row: no workflow runs it. `internal/le/workflowcheck/workflowcheck_test.go` `TestEvidenceNightlyScheduleActionsAndPrivileges` pins the action list and refuses a `docker-lab` job that is not one of the five named labs.
  → Decision: the `docker` verb's nightly job runs on the hosted runner's own Docker, not in `docker-lab`: nothing it runs puts Ze in a container, so the Docker kernel check does not apply (Q-10).
- [ ] `docs/architecture/testing/interop.md` - "Prerequisites", "Test Hierarchy"
  → Constraint: both tables list each Docker-needing suite with what it needs; the new verb and `cleanup` get rows there.
- [ ] `docs/architecture/core-design.md` - declared by `internal/le/test/deployment/actions.go` and `internal/le/site/terminaldemo/render.go`
  → Constraint: unaffected; grep of the page for docker, deployment and terminal demo finds no statement about how those callers start containers, so routing them through `interoplab.Docker` makes nothing on it false. No package boundary or registration moves.
- [ ] `docs/architecture/testing/ci-format.md` - declared by `internal/le/verify/evidence/evidence.go`
  → Constraint: unaffected; the page describes the `.ci` format and failure blocks, not how the evidence container is started, so moving `Runner.dockerArgs` onto `RunOneShot` changes nothing it states.
- [ ] `ai/rules/user-facing-errors.md`, `.claude/rules/docker.md`, `ai/rules/no-layering.md`, `ai/rules/simplicity.md`, `ai/rules/testing.md`, `ai/rules/architecture.md`
  → Constraint (docker.md): never a host-wide prune; remove only what a Ze lab run created, by identity. Here identity is the label set plus the exact resource id, never a name pattern.
  → Constraint (user-facing-errors): every skip and refusal the sweep prints names the resource, why it was not removed, and the exact command that resolves it.

**Key insights:**
- A killed lab run leaks today (V-1): detached `docker run -d` containers with no `--rm`, the scenario network, and the run-owned `ze-interop-retained:<random>` image tag all survive, because teardown is a Go `defer` and the native actions run on `context.Background()` with no signal handler, so even Ctrl-C skips it.
- No `interoplab` call applies a Docker label anywhere; nothing can tell a dead run's resources from a live one's today.
- Container names are `<role>-<suffix>`, suffix = pid by default or an env override; the pre-clean force-removes by name, so it can remove a LIVE run's container that shares the name (another machine on a shared daemon with the same pid, or two runs with the same explicit suffix).
- An exact pid-reuse token already exists: `session.ScanProcesses` (`internal/le/session/reap.go`) answers `Process.Start` from the `/proc/<pid>/stat` start-time field on Linux and `ps lstart` elsewhere, and `./le session reap` already carries the "owners are provably gone" contract this sweep copies.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/lab.go` - `Suite.Run` probes, runs Preflight, the kernel check, `prepareImages`, then `runScenario` per scenario. Image release is a `defer` on a `context.WithoutCancel` context; scenario teardown (`Lab.cleanup`: `RemoveContainer` per declared peer, then `removeNetwork`) is a `defer` in `runScenario`. `Lab.preClean` runs the same `cleanup` by name before the network is created.
- [ ] `internal/le/interoplab/docker.go` - `Docker` wraps the CLI through `processRunner`. `runContainer` argv is `docker run -d --name <c> --network <n> --ip ...` with no `--rm` and no `--label`. `createNetwork` has no `--label` and treats "already exists" as success (adopts a foreign network of the same name). `Build` tags `build.Tag` plus `ze-interop-retained:<rand.Text()>`, no `--label`. `RunOneShot` is `docker run --rm` foreground. `RemoveContainer` is `docker rm -f <name>`; `releaseImage` is `docker image rm --no-prune <retained tag>`.
- [ ] `internal/le/interoplab/process.go` - `systemProcessRunner.Run` uses `exec.CommandContext`; no process group, no Pdeathsig. Killing le kills nothing the daemon owns.
- [ ] `internal/le/interoplab/discover.go` - `ReadEnvironment`: suffix defaults to `os.Getpid()`, overridable per lab (`ZE_IPSEC_INTEROP_SUFFIX`, `ZE_L2TP_INTEROP_SUFFIX`, `ZE_RSVPTE_INTEROP_SUFFIX`, ...).
- [ ] `internal/le/interoplab/kernelcheck.go` - `kernelProbeArgv`: `docker run --rm --network none` one-shot, no label. Callers: `setup docker-kernel`, `verify/evidence`, `perf/bench`, `site/terminaldemo`, `test/deployment/daemonkernel`.
- [ ] `internal/le/test/integration/actions.go` - `nativeRunner` calls `run(context.Background(), root)`: no `signal.NotifyContext`. `ipsec.Run` and `radius.Run` also root on `context.Background()`.
- [ ] `internal/le/job/job.go` - `forwardSignals` forwards SIGINT/SIGTERM to the child; the child has no handler, so Go's default action terminates it without running defers.
- [ ] `internal/le/session/reap.go` - `ScanProcesses`, `Process{PID, Start, StartedAt, Argv}`, `procFact` (Linux `/proc` stat field), `scanProcessesWithPS` (`ps -eo pid=,lstart=,...`, macOS and BSD).
- [ ] `internal/le/test/deployment/{deployment,l2tp,vppiface,vppevidence,vppevidencerun}.go` - own `exec.CommandContext("docker", ...)`: `L2TP.containerArgs` and `vppIface.containerArgs` build `run --rm -d ... sleep infinity` (detached, privileged caps, `/dev/net/tun`, `/dev/vhost-net`), `docker exec` daemons on `context.Background()`, `removeContainer` that discards its error.
- [ ] `internal/test/perfrunner/run.go` - own docker: `network create`, `Runner.startDUT` `run -d` DUT containers, a `sleep 3600` sender in `Runner.runPerf`, image builds; `rm -f` in `Runner.stop` and a `defer`, errors discarded.
- [ ] `internal/le/verify/evidence/evidence.go` - `Runner.dockerArgs`: own `docker run --rm` of a long build container with the runner capability table, named cache volumes `ze-gomod-cache` and siblings (deliberately persistent).
- [ ] `internal/le/site/terminaldemo/render.go` - own `docker run --rm --network none` (browser render with `--privileged`).
- [ ] `internal/component/bgp/plugins/rpki/{rpki_live_test,rtr_downgrade_test}.go` (`//go:build live`) - own `docker run -d` of stayrtr, `docker rm -f` cleanup, skip on missing binary, on `docker info` failure and on pull failure.
- [ ] `internal/le/interoplab/image_retention_integration_test.go` (`//go:build integration`) - skips on missing binary, FAILS on `Probe` error; uses raw `DockerExecutable run --rm` argv.
- [ ] `internal/plugins/flowexport/interop_integration_linux_test.go` (`//go:build integration && linux`) - collectors are host processes from a provisioned `-flowexport-peer-dir`; skips with "an unprovisioned skip is not interoperability evidence". `test/interop-flowexport/Dockerfile.collectors` exists.
- [ ] `plan/journal/stop-leaves-the-work-running.md` - open row 2026-10-11: the appliance kernel builder's `docker run --rm` keeps compiling after a stop SIGKILLs the CLI.

**Verdict V-1 (asked: does a killed run leak today?): YES, and so does Ctrl-C.**

| Kill | What survives | Evidence at the producer |
|------|---------------|--------------------------|
| SIGKILL of le, lost ssh session (SIGHUP, unhandled) | every running peer container (detached, no `--rm`, the daemon owns it), the scenario network, the `ze-interop-retained:<rand>` tag of every built image | `Docker.runContainer` argv `run -d` with no `--rm`; teardown is a `defer` in `Suite.runScenario` and `Suite.Run`; a killed process runs no defer |
| Ctrl-C or SIGTERM (through `./le job run` or directly) | the same | `nativeRunner` uses `context.Background()`; no `signal.Notify` in `internal/le/test/integration` or `internal/le/interoplab`; `forwardSignals` delivers SIGINT to a child whose default action is exit without defers |
| `go test -timeout` panic in a package test | for the rpki live tests: the stayrtr container | the stayrtr helper in `rpki_live_test.go` reaches its `docker rm -f` only on the normal path, which a timeout panic does not take |
| SIGKILL of `docker-l2tp-ppp-test` or a VPP proof | a privileged `sleep infinity` container, forever (`--rm` fires only when its process exits, which `sleep infinity` never does) | `L2TP.containerArgs`, `vppIface.containerArgs` |
| SIGKILL of a one-shot (`RunOneShot`, kernel probe, evidence, terminal demo) | the container keeps running its command to the end, then `--rm` removes it | foreground `--rm`; the CLI cannot forward a SIGKILL |

A later run does not collect the leftovers: its pre-clean removes only names
built from ITS pid, so a different pid leaves the old set untouched.

**Behavior to preserve:**
- Every lab's container names, network names, subnet candidates, addresses, readiness probes, checker semantics, report shape (`SuiteReport`, `ScenarioResult.CleanupErrors`) and exit codes.
- `releaseImage` removes only the run-owned tag (`--no-prune`), never the shared cache tag; `NO_BUILD=1` reuse by cache tag.
- The named cache volumes `verify/evidence` mounts survive between runs; nothing in this spec removes a volume.
- `dockerhost` socket discovery and every AppArmor and kernel-check behavior.
- The argv-shape tests (`terminaldemo_test.go` `assertSubsequence`, `grantcheck`) keep asserting the same flags; they read the argv through the wrapper's runner instead of a package-local builder.

**Behavior to change:**
- Every resource the wrapper creates carries the owner labels (AC-1..AC-3).
- `preClean` no longer force-removes a same-name container it cannot prove dead (AC-7).
- Lab roots cancel on SIGINT/SIGTERM so teardown runs (AC-8).
- MUST sites lose their own docker invocations (AC-11..AC-15).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration <interop verb>`, `./le test deployment <verb>`, `./le perf ...`, `./le verify evidence`, the terminal-demo render (operator commands), `./le test integration docker-peers` (the G-3 population), and an ad hoc `go test -tags docker <pkg>`.
- New: `./le test integration cleanup` (explicit sweep; the `dry` keyword lists without removing). Owner, 2026-10-18, on the proposed `lab-reap`: "looks like a bad name. Cleanup ?"

### Transformation Path
1. Process start: `interoplab` computes the owner identity once: machine id, boot id, pid, pid start token, start time, run id.
2. Every create argv the wrapper builds (`runContainer`, `RunOneShot`, `createNetwork`, `Build`, the kernel probe) appends the label set from one helper.
3. Sweep (`Docker.CleanupOrphans`, in `orphans.go`; "orphan" because `Lab.cleanup` already names scenario teardown): list labeled containers, networks and images through `docker ps -a`, `docker network ls`, `docker image ls` filtered on the marker label; judge each owner (gone / alive / unknown); remove gone ones in the order containers, networks, run-owned tags, by resource id; collect skips with reasons.
4. `Suite.Run` and the other entry points run the sweep once after `Probe`, before creating anything; the sweep's report joins the run's progress output.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le process ↔ Docker daemon | docker CLI argv through `processRunner` (`--label`, `--filter label=`, `--format`) | No |
| Client host ↔ daemon host (Colima VM, remote `DOCKER_HOST`) | owner facts are CLIENT-side facts carried in labels; the daemon is only storage | No |
| `_test.go` in a component ↔ `internal/le/interoplab/labtest` | Go import from test files only | No |

### Integration Points
- `interoplab.Docker` - the only Docker client; gains `CleanupOrphans` and the label append in its argv builders.
- `session.ScanProcesses` / `session.Process.Start` - the pid-reuse token. If importing `internal/le/session` from `interoplab` pulls action registration into `interoplab`, the process-identity code moves to a leaf package both import (moved, not copied).
- `interoplab.ReadyProbe` and `Lab.waitPeer` - readiness for G-3.
- `leaction` area registration for the new verb.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes, after AC-11..AC-15 | every MUST site's own `exec.Command("docker")` deleted; AC-16 refuses survivors |
| No unintended coupling (components stay isolated) | Yes | component code imports nothing new; only `_test.go` files import `labtest` (AC-17) |
| No duplicated functionality (extends existing, does not recreate) | Yes | extends `interoplab.Docker`; reuses `session.ScanProcesses`; testcontainers-go rejected |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path |
| Registration over hardcoding, outbound | Yes | the `cleanup` verb registers through `leaction`; no switch learns a lab name |
| Registration over hardcoding, inbound | Yes | the sweep reads labels, not a list of lab names or name prefixes; searched: `declaredCleanupPeers`, the bgp lab's `containerRoles`, each lab's `containerName` helper; none needs to learn the orphan cleanup |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `session.Process.Start` changes whenever a pid is reused, on Linux and macOS | `reap.go` comment "Start is an opaque token that changes when a pid is reused"; `/proc` start-time field; `ps lstart` | pid reuse makes a dead owner look alive (safe direction: skip); the reverse needs pid reuse within one `lstart` second on macOS (R-2) | unit test with a fixture scan | confirmed on Linux (phases 1-2): `TestCleanupOrphansVerdictTable` over a fixture scan, and `TestKilledLabRunIsCleanedByNextStart` judged a SIGKILLed child gone and a live one alive by `/proc` start token; macOS not run |
| A-2 | Linux boot id (`/proc/sys/kernel/random/boot_id`) and macOS `kern.bootsessionuuid` are readable without privilege and change on every boot | kernel ABI; macOS sysctl | the "same machine, rebooted" verdict is unavailable; resources from before a reboot become "unknown" (skipped, reported) | `TestOwnerIdentityReadsBootID` on Linux; a manual Mac check recorded in this spec | Linux confirmed (the real-daemon run labeled with the host boot id, unprivileged); the macOS manual check is still owed |
| A-3 | A machine identity is readable on both hosts: Linux `/etc/machine-id` (as `internal/core/identity` reads it), macOS IOPlatformUUID | `identity.go` `readMachineIDFile`; macOS `ioreg -rd1 -c IOPlatformExpertDevice` | without it, "another machine" and "this machine rebooted" cannot be told apart; both become "unknown" (skipped) | unit test with injected readers; the Mac check | Linux confirmed; the darwin parser is unit-tested with injected `ioreg`/`sysctl` output; the macOS manual check is still owed |
| A-4 | `docker build --label` gives each run's final image a distinct id while the layer cache still hits | Docker build semantics: labels are image config | if ids were shared, safety is unchanged because only the run-owned TAG is ever removed | `image_retention_integration_test.go` extended (AC-3) | confirmed 2026-10-18: two runs built the same `FROM alpine:3.21` Dockerfile with their own labels and got distinct image ids (4eba20710da2, b382d29fd516). The `image_retention` extension is phase 4 (AC-14) |
| A-5 | Every daemon Ze labs use (Linux host, Colima, Docker Desktop, the QEMU guest's Alpine dockerd) supports `--label` and `--filter label=` on `run`, `network create`, `build`, `ps`, `network ls`, `image ls` | Docker CLI and API | the sweep cannot list; it reports a setup error naming the daemon version | the integration test on Linux; the nightly docker-lab on the guest daemon | Linux host daemon confirmed by `TestKilledLabRunIsCleanedByNextStart`; Colima, Docker Desktop and the guest dockerd unvalidated |
| A-6 | Importing `internal/le/session` from `interoplab` creates no cycle | `go list -deps ./internal/le/interoplab` shows no `internal/le/session` today, and session does not import interoplab | move the scanner to a leaf package | `go build ./internal/le/...` in phase 2 | broken in phase 1: no cycle, but `internal/le/session` registers `./le` actions in `register.go` and imports `internal/component/command`, so the scanner moved (not copied) to the leaf `internal/le/procscan`; session and scratch call it |
| A-8 | The hosted `ubuntu-latest` runner's own Docker can run the `docker` population: no `labtest` peer needs a Ze-kernel feature, and Ze runs in the test process, not in a container | `ci-workflows.md` says the labs go to the guest only because the staged ze fails the Docker kernel check; `labtest` stages no ze | the job moves into `./le test qemu docker-lab`, which also needs a Go toolchain in that guest (unverified: `dockerlab.go` cross-builds on the host) | the first nightly run; Q-10 | unvalidated |
| A-9 | A `ZE_`-prefixed test switch set by a `./le` verb needs no `env.MustRegister` entry, as `ZE_QEMU_LINUX_ONLY` does not | `qemu-nightly.yml` and the workflowcheck test use `ZE_QEMU_LINUX_ONLY=1` | the switch gets registered where that one is | grep for `ZE_QEMU_LINUX_ONLY`'s declaration in phase 4 | unvalidated |
| A-7 | No production package imports `internal/le/...` today | grep of non-test files under `internal/component`, `internal/plugins`, `internal/core`, `cmd/ze`, `pkg` found none | AC-17's refusal would break an existing importer | the AC-17 gate's first run | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The sweep removes a live run's resources (the one unforgivable failure) | a concurrent session's scenario fails with "No such container" | three-valued verdict; remove only on "gone"; "gone" requires the same machine id AND (a different boot id, OR the same boot id with the pid absent or its start token different); every other case is "unknown" and skipped. AC-5 and AC-6 test both directions; the integration test runs a live owner beside a dead one |
| R-2 | pid reuse within the same second on macOS (`lstart` resolution) makes a new process look like the dead owner | none at run time | that direction is SAFE (the dead run's resources are kept and reported as owned by a live pid); the next sweep after that process exits removes them |
| R-3 | A run on another machine shares this daemon (remote `DOCKER_HOST`, a shared Colima): its resources can never be judged here | the sweep's "unknown owner" lines name the machine id | skip and report with the command to run on that machine; never a forced removal (Q-2) |
| R-4 | Colima / Docker Desktop: the daemon's pid namespace is the VM's, not the Mac's | none if done right | owner facts are taken on the CLIENT (le on the Mac), never from the daemon; the sweep judges on the client too. AC-4 asserts no daemon-side pid is ever read |
| R-5 | A sweep races a run that is creating resources | flaky "No such container" in the creating run | a live creator's resources carry a live owner, so the sweep skips them; labels are set in the creating argv, so there is no unlabeled window |
| R-6 | Two sweeps run at once | duplicate removal errors | name-specific "absent" answers are success (the existing `RemoveContainer`, `removeNetwork`, `releaseImage` semantics), so the loser reports nothing |
| R-7 | Removing a network fails because a container of an UNKNOWN owner is still attached | `docker network rm` "has active endpoints" | a network is removed only when its own owner is gone; an attached foreign endpoint makes it a reported skip, not an error that fails the run |
| R-8 | Pre-existing UNLABELED leftovers from before this spec stay forever | `docker ps -a` shows old `<role>-<pid>` containers | the rule forbids touching them; Q-3 decides whether `cleanup dry` lists them |
| R-9 | Ctrl-C mid-`docker build` and the cleanup bound is too short | cleanup errors after Ctrl-C | cleanup already runs on `context.WithoutCancel` with `dockerCommandTimeout`; a second signal is the operator's SIGKILL, handled by the next sweep |
| R-10 | Deleting the deployment, perf, evidence and terminal-demo argv builders breaks their argv-shape tests | red package tests | the tests move to asserting the argv the wrapper's fake `processRunner` records (same flags); no assertion is weakened |
| R-12 | A test file carries both `docker` and `integration`, so it lands in the guest (no Docker) and skips there forever | it shows up in `all-tests` as a skip | `TestDockerTagNeverJoinsIntegration` refuses the combination, naming the file |
| R-13 | A `docker`-tagged package the verb's scan cannot see (outside `internal/` and `cmd/`) runs nowhere | none | the scan covers the same roots as `integrationTestPackages`; `TestDockerPeersVerbRunsEveryDockerPackage` fails on a fixture package the verb's argv lacks |
| R-14 | The rpki live tests stay on `live` and still run nowhere, because `live-rpki` has no nightly | `ci-workflows.md` has no row | Q-11 |
| R-11 | A sweep failure (the daemon refuses `ps`) blocks every lab | setup error on every run | a listing failure is a setup error naming the command and the fix, as `Probe` already does; a skip is never an error |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Another session's lab containers removed mid-run (R-1), or leftovers not removed (back to today). No product behavior: tooling only |
| How is it reverted? | Single commit revert per phase |
| Who else touches this path? | `plan/pre-release/spec-lab-containers-least-privilege.md` (in-progress, edits `runContainer` argv and lab peer configs), `plan/spec-interop-image-copies-a-prebuilt-ze.md`, `plan/spec-interop-wire-capture.md`; phase 2 rebases over whichever lands first in `docker.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration interop` (any lab verb) | → | `Suite.Run` → `Docker.CleanupOrphans` before Preflight | `TestSuiteRunCleansOrphansBeforePreflight` (fake runner, asserts the labeled `ps -a` listing precedes the first create) |
| `./le test integration cleanup [dry]` | → | the registered action → `Docker.CleanupOrphans` | `TestLabCleanupActionDispatches`, `TestLabCleanupDryRemovesNothing` |
| A lab run SIGKILLed, then any lab start | → | labels on create + `Docker.CleanupOrphans` | `TestKilledLabRunIsCleanedByNextStart` (`//go:build docker`, real daemon, re-exec child, SIGKILL) |
| Ctrl-C of `./le test integration interop` | → | `signal.NotifyContext` root → existing deferred cleanup | `TestNativeRunnerCancelsOnSIGINT` |
| `./le test integration docker-peers` | → | tree scan for `docker`-tagged packages → `go test -tags docker` with `ZE_REQUIRE_DOCKER=1` → `labtest.Start` | `TestDockerPeersVerbRunsEveryDockerPackage`; `TestLabtestStartsServerAndCleansUp` (`//go:build docker`) |
| `evidence-nightly.yml` job `docker-peers` | → | `./le test integration docker-peers` on the hosted runner | `TestEvidenceNightlyScheduleActionsAndPrivileges` (its `want` list gains `test integration/docker-peers`) |
| `./le test integration live-rpki` | → | `labtest.Start` (stayrtr) | the existing `TestLiveRPKIValidation`, `TestLiveASPAValidation`, `TestLiveRTRv2DowngradeToV1`, converted |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any container the wrapper starts (`runContainer`, `RunOneShot`, the kernel probe) | `docker inspect` shows the marker label and every owner label: run id, owner pid, owner start token, boot id, machine id, start time (RFC 3339), purpose (the lab or caller name). Key names are declared once in `orphans.go` and listed in `interop.md` from that declaration |
| AC-2 | Any network the wrapper creates | carries the same label set |
| AC-3 | Any image `Build` produces | carries the label set; the run-owned `ze-interop-retained:` tag is the only tag the sweep may remove; the shared cache tag is never removed; `TestDockerBuildRetainsImageAcrossRetag` still passes |
| AC-4 | Owner identity on a client whose daemon is in a VM (Colima) or remote | every owner fact comes from the client process; no code path reads a pid, boot id or machine id from the daemon (`docker info`, `docker inspect` State.Pid). A fake-runner test asserts no such call |
| AC-5 | Sweep over resources whose owner is (a) same machine, same boot, pid absent; (b) same machine, same boot, pid present with a different start token; (c) same machine, different boot | each is removed, containers (`rm -f <id>`) then networks (`network rm <id>`) then run-owned tags (`image rm --no-prune <tag>`); the report counts them |
| AC-6 | Sweep over resources whose owner (a) is alive with the same start token; (b) has a different machine id; (c) has the marker but missing or malformed owner labels; (d) cannot be judged because this client's machine id or boot id is unreadable | none is removed; each is reported as a skip naming the resource, the reason and the fix: for (b) "run `./le test integration cleanup` on machine <id>", for (a) the live pid and its command line |
| AC-7 | `preClean` meets an existing container with a scenario's name | removed only when the sweep's verdict for its labels is "gone"; otherwise the scenario fails with an error naming the container, its owner and the fix (set the lab's suffix variable, or run `./le test integration cleanup`); an unlabeled same-name container is refused the same way |
| AC-8 | SIGINT or SIGTERM delivered to a running lab verb, a deployment verb, `./le perf`, `./le verify evidence`, or a terminal-demo render | the root context cancels, the existing teardown runs, the process exits non-zero, and no labeled resource of that run remains |
| AC-9 | Any resource without the marker label, any volume, any image tag other than the run-owned prefix | never listed for removal and never removed, in every mode |
| AC-10 | `./le test integration cleanup dry` | lists what would be removed and what is skipped, removes nothing, exit 0; without `dry` removes the gone set, exit 0 when every removal succeeded, 1 with the failing command named otherwise; the report renders through `| json` |
| AC-11 | `internal/le/test/deployment` (`deployment.go`, `l2tp.go`, `vppiface.go`, `vppevidence.go`, `vppevidencerun.go`) | no `exec.Command*("docker", ...)` remains; containers, execs, image ensure and removal go through `interoplab.Docker`; a removal error is reported in the verb's report, not discarded |
| AC-12 | `internal/test/perfrunner/run.go` | no own docker invocation remains; network, DUT and sender containers and image builds go through `interoplab.Docker`, labeled; removal errors reported |
| AC-13 | `internal/le/verify/evidence/evidence.go` and `internal/le/site/terminaldemo/render.go` | their one-shot containers run through `Docker.RunOneShot`, labeled; the named cache volumes are untouched; the argv tests assert the same flags via the fake runner |
| AC-14 | `internal/le/interoplab/image_retention_integration_test.go` | its raw `run --rm` argv is replaced by `RunOneShot`, and the file moves from the `integration` tag to `docker` (renamed `image_retention_docker_test.go`), so `docker-peers` runs it; the `internal/le/interoplab` reason in `excludedIntegrationPackages` is rewritten to name only `TestDockerKernelCheckOnThisHost`, which needs a Ze-kernel daemon and stays where it is |
| AC-15 | `rpki_live_test.go`, `rtr_downgrade_test.go` | stayrtr starts through `labtest.Start`; no own docker invocation remains; they keep the `live` tag (stayrtr's `-cache` is `https://rpki.cloudflare.com/rpki.json`, real internet data), and `live-rpki` sets `ZE_REQUIRE_DOCKER=1` like `docker-peers`; skip and fail follow AC-19 |
| AC-16 | A gate over non-vendor Go outside `internal/le/interoplab` | refuses an `exec.Command`/`exec.CommandContext` whose program is `docker` (literal or constant), naming `interoplab.Docker` as the route, except where the program is reached through a row this inventory marks NOT (N-4); hosted in an existing `./le arch` gate, its exemption read from the inventory decision, not a hand list beside it |
| AC-17 | A non-test Go file outside `internal/le` imports `internal/le/interoplab/labtest` | `./le arch tier check` refuses it, naming the file and the rule |
| AC-18 | `labtest.Start(t, spec)` with an image, a network or published port, a ready probe | starts one labeled container, waits for the probe, returns its address; on `t.Failed()` writes the last N log lines to `t.Log`; `t.Cleanup` removes the container and its network; a test killed by `-timeout` leaves only labeled resources a later sweep removes |
| AC-19 | `labtest.Start` where the `docker` binary is absent / present but `Probe` fails / the image cannot be pulled | absent: `t.Skip` naming the reason and the install route ("Graceful Degradation"), except under `ZE_REQUIRE_DOCKER=1`, where it is `t.Fatal` naming the same fix; `Probe` failing: `t.Fatal` naming the probe error and the fix (as the wrapper's own retention test already does); pull failure: per owner decision Q-6 |
| AC-22 | `./le test integration docker-peers` | derives its packages from the tree (every package under `internal/` and `cmd/` holding a `_test.go` whose build constraint names `docker`), runs `go test -tags docker -count=1` over exactly that set with `ZE_REQUIRE_DOCKER=1` and a signal-cancelled root, and fails when the set is empty; adding a package needs no edit to the verb |
| AC-23 | A `_test.go` whose constraint names both `docker` and `integration` | `TestDockerTagNeverJoinsIntegration` (beside `TestEveryIntegrationPackageIsNamed`) refuses it, naming the file and the reason (the guest has no Docker) |
| AC-24 | The tree scan | one scanner, moved out of `integration_coverage_test.go` into a non-test leaf (`integrationTestPackages` becomes its caller, moved, not copied), serves both the qemu population test and AC-22 |
| AC-25 | `evidence-nightly.yml` | gains a job `docker-peers` running `./le test integration docker-peers` on the hosted runner, stating its own verdict like every job there; `TestEvidenceNightlyScheduleActionsAndPrivileges` lists `test integration/docker-peers` in `want`; the job holds no `docker-lab`, so the guest-lab branch of that test is unchanged; `ci-workflows.md` gets the row (job host per Q-10) |
| AC-26 | A `labtest` test | runs no Docker host kernel check (`interoplab.DockerKernel`): Ze runs inside the test process on the host, and the check guards a containerised Ze. A peer that needs Ze-kernel features belongs in an `interoplab.Suite` lab, not in `labtest` |
| AC-20 | Each SHOULD row the owner accepts (Q-5) | its package gets a `//go:build docker` test driving Ze's client or server code against the real peer image through `labtest`, with the discrimination walk of `ai/rules/interop-and-goal-validation.md` recorded (revert, red, restore, green) |
| AC-21 | Each NOT row | its reason stays recorded in the inventory; the owner confirms or overrides it (Q-7) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOwnerLabelsOnEveryCreate` | `internal/le/interoplab/orphans_test.go` | AC-1, AC-2, AC-3 argv over the fake runner for each create path | |
| `TestOwnerIdentityNeverReadsTheDaemon` | `internal/le/interoplab/orphans_test.go` | AC-4 | |
| `TestOwnerIdentityReadsBootID` | `internal/le/interoplab/orphans_test.go` | A-2, A-3 with injected readers | |
| `TestCleanupOrphansVerdictTable` | `internal/le/interoplab/orphans_test.go` | AC-5 (a, b, c) gone and AC-6 (a, b, c, d) skip, table-driven over injected identity and scan | |
| `TestCleanupOrphansOrderContainersNetworksTags` | `internal/le/interoplab/orphans_test.go` | AC-5 order; a network with a foreign endpoint is a skip (R-7) | |
| `TestCleanupOrphansNeverTouchesUnlabeledVolumesOrCacheTags` | `internal/le/interoplab/orphans_test.go` | AC-9 | |
| `TestCleanupOrphansConcurrentAbsentIsSuccess` | `internal/le/interoplab/orphans_test.go` | R-6 | |
| `TestCleanupOrphansSkipMessagesNameFix` | `internal/le/interoplab/orphans_test.go` | AC-6 messages carry what, why and the command | |
| `TestPreCleanRefusesLiveOrUnlabeledSameName` | `internal/le/interoplab/lab_test.go` | AC-7 | |
| `TestSuiteRunCleansOrphansBeforePreflight` | `internal/le/interoplab/lab_test.go` | wiring | |
| `TestDockerPeersVerbRunsEveryDockerPackage` | `internal/le/test/integration` | AC-22: over a fixture tree, the argv names exactly the `docker`-tagged packages, sets `ZE_REQUIRE_DOCKER=1`, and refuses an empty set; a fixture package added later appears with no verb edit | |
| `TestDockerTagNeverJoinsIntegration` | `internal/le/test/qemu/integration_coverage_test.go` | AC-23, R-12 | |
| `TestLabtestSkipsOrFailsOnAbsentDocker` | `internal/le/interoplab/labtest/labtest_test.go` | AC-19 both arms with an injected lookup: skip without the switch, fail with it | |
| `TestLabtestRunsNoKernelCheck` | `internal/le/interoplab/labtest/labtest_test.go` | AC-26 over the fake runner | |
| `TestEvidenceNightlyScheduleActionsAndPrivileges` (changed) | `internal/le/workflowcheck/workflowcheck_test.go` | AC-25 | |
| `TestNativeRunnerCancelsOnSIGINT` | `internal/le/test/integration/actions_test.go` | AC-8 | |
| `TestLabCleanupActionDispatches`, `TestLabCleanupDryRemovesNothing` | `internal/le/test/integration` | AC-10 | |
| deployment, perfrunner, evidence, terminal-demo argv tests | their existing `_test.go` files | AC-11..AC-13, same flags via the fake runner | |
| `TestNoDockerExecOutsideInteroplab` | the `./le arch` gate chosen in phase 3 | AC-16, with a refusing fixture | |
| tier selftest fixture for `labtest` | `internal/le/arch/tier/testdata/` | AC-17 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| owner pid label | 1..2147483647 | 2147483647 | 0 (malformed → skip) | 2147483648 (malformed → skip) |
| log tail lines on failure (`labtest`) | 1..1000 | 1000 | 0 (refused at spec construction) | 1001 (refused) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestKilledLabRunIsCleanedByNextStart` | `internal/le/interoplab/orphans_docker_test.go` (`//go:build docker`, so `docker-peers` runs it) | a re-exec'd child creates a labeled network, container and retained tag and is SIGKILLed; a second, live child's resources exist beside them; one sweep removes exactly the first set | |
| `TestLabtestStartsServerAndCleansUp` | `internal/le/interoplab/labtest/labtest_docker_test.go` (`//go:build docker`) | a package test starts a server, reaches it, and leaves nothing after | |
| `./le test integration cleanup dry` | run in phase 2 against a seeded daemon, output pasted in this spec | the operator sees what would go and why the rest stays | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A for G-1..G-3 (tooling, no wire change) | - | - | the SHOULD rows of AC-20 are the interop additions, one per accepted row | |

## Adoption Inventory ("where it should and must")

Searched: `exec.Command*("docker"`, `DockerExecutable`, `"docker",` argv,
`"run", "-d"` / `"--rm"`, `"network", "create"`, `.sh`, Makefile and workflow
`docker` uses, `.ci` and `.et` files, `internal/le/{verify/evidence,test/deployment,test/qemu,setup/dockerkernel,perf,feature,site/terminaldemo,test/integration}`,
`internal/test/*`, `internal/appliance`, `contrib/`, `.github/workflows`.

| # | Site | What it starts | Through `interoplab.Docker`? | Labels / cleanup today | Verdict | AC / decision |
|---|------|----------------|------------------------------|------------------------|---------|---------------|
| M-1 | `interoplab.Suite` labs: bgp, ipsec, radius, radiussubscriber, rsvpte, l2tp, pppoe | networks, detached peers, built images and retained tags, preflight one-shots | yes | none; defer-only teardown; Ctrl-C leaks | MUST | AC-1..AC-10 |
| M-2 | `interoplab/kernelcheck.go` kernel probe (used by setup docker-kernel, evidence, perf, terminal demo, deployment) | `run --rm` one-shot | own argv inside `interoplab` | none | MUST (move onto `RunOneShot`) | AC-1 |
| M-3 | `internal/le/test/deployment` (`docker-l2tp-ppp-test`, the VPP proofs, `ensureImage`, `removeContainer`) | privileged `run --rm -d ... sleep infinity`, `docker exec` daemons, pulls | no, own `exec.Command("docker")` | none; removal error discarded; SIGKILL leaves `sleep infinity` forever | MUST | AC-11, AC-8 |
| M-4 | `internal/test/perfrunner/run.go` (`./le perf`) | network, DUT containers, `sleep 3600` sender, image builds | no | none; `rm -f` errors discarded | MUST | AC-12, AC-8 |
| M-5 | `internal/le/verify/evidence/evidence.go` | long privileged `run --rm` build container | no | none; persistent cache volumes (kept) | MUST | AC-13, AC-8 |
| M-6 | `internal/le/site/terminaldemo/render.go` | `run --rm` (`--privileged` for the browser render) | no | none | MUST | AC-13, AC-8 |
| M-7 | `rpki/rpki_live_test.go`, `rpki/rtr_downgrade_test.go` | stayrtr `run -d` | no | `rm -f` on the normal path only | MUST (through `labtest`) | AC-15, AC-19 |
| M-8 | `interoplab/image_retention_integration_test.go` | `run --rm` | raw argv | test-owned cleanup; tagged `integration` yet excluded from the guest, so it runs nowhere today | MUST (onto `RunOneShot`, retagged `docker`) | AC-14 |
| N-11 | `interoplab/kernelcheck_host_integration_test.go`, `internal/le/test/deployment/vpp_srv6_*integration*_test.go` (8 files) | kernel probe; privileged VPP containers | yes / through M-3 after AC-11 | excluded from the guest; no verb or nightly runs them | NOT in this spec's tag move: they need a Ze-kernel Docker host or privileged VPP, which the hosted runner may not give; their missing home is a pre-existing gap of the same shape, Q-12 | Q-12 |
| S-1 | TACACS+ client (`internal/component/tacacs`) | none: `.ci` tests against `internal/test/mock/tacacs` | - | - | SHOULD: no real TACACS+ server proves it anywhere (tac_plus-ng image) | AC-20, Q-5 |
| S-2 | NTP client (`internal/plugins/ntp`) | none: in-process fake | - | - | SHOULD (chrony server) | AC-20, Q-5 |
| S-3 | gNMI server (`internal/component/gnmi`) | none: one in-process test client | - | - | SHOULD (gnmic as the real client) | AC-20, Q-5 |
| S-4 | flow-export (`internal/plugins/flowexport/interop_integration_linux_test.go`) | host-process collectors from a provisioned directory; skips otherwise | no | n/a | SHOULD: `Dockerfile.collectors` exists; `labtest` removes the unprovisioned skip | AC-20, Q-5 |
| S-5 | DHCP server (`internal/plugins/dhcpserver`) and DHCP client (`internal/plugins/iface/dhcp`) | in-process fakes | - | - | SHOULD (udhcpc or dhclient as client; dnsmasq or Kea as server); needs an L2 path into Ze's namespace, so larger | AC-20, Q-5 |
| S-6 | syslog sender (fake receiver `internal/test/syslog`) | in-process fake | - | - | SHOULD (rsyslog receiver) | AC-20, Q-5 |
| N-1 | RADIUS client (`internal/component/radius`) | `.ci` mock | - | - | NOT: FreeRADIUS already proves it in `interop-radius` and `interop-radius-subscriber` (M-1) | Q-7 |
| N-2 | RPKI/RTR beyond M-7, and BMP | mocks for fault injection | - | - | NOT: stayrtr (bgp interop `rpki-frr`, and M-7) and pmacct (`bmp-*-pmacct`) are real peers already; the mocks stay for faults a real peer cannot produce | Q-7 |
| N-3 | `internal/le/test/qemu/dockerlab.go` | runs labs in a throwaway guest dockerd | inherits M-1 | guest discarded | NOT: the guest dies; M-1 labels apply inside it anyway | Q-7 |
| N-4 | `internal/appliance/kernelbuilder` (`ze-host appliance kernel --builder docker`), and `deployment/gokrazyimage.go`, which runs it | `run --rm` kernel build | no | journal row in `stop-leaves-the-work-running.md` (2026-10-11), open | NOT here: `ze-host` is a shipped host-driver binary and may not import `internal/le` | Q-7 |
| N-5 | `internal/component/host/platform_linux.go` | nothing (detects running inside a container) | - | - | NOT: starts nothing | Q-7 |
| N-6 | `internal/le/setup/tools.go`, `internal/test/toolstub`, `internal/test/fixture` install fixtures, `test/install/*.ci`, `test/ui/doctor-*.ci` | a stubbed `docker`, no daemon | - | - | NOT: no real resource | Q-7 |
| N-7 | `internal/plugins/vrrp/lab_profile_linux_test.go`, `internal/le/feature/recordrun.go`, `internal/le/setup/dockerkernel` | nothing of their own (registry read, catalog run, kernel check) | inherit M-1, M-2 | - | NOT: covered by M-1 and M-2 | Q-7 |
| N-8 | `internal/le/test/integration` stress verbs, `interoplab/ipsec` mobike native lab | network namespaces, not Docker | - | - | NOT: no Docker; a namespace leak is a different resource and class | Q-7 |
| N-9 | `contrib/netlab` (containerlab) | an operator-run containerlab topology | no | containerlab owns its lifecycle | NOT: an operator-facing integration, not Ze's test tooling | Q-7 |
| N-10 | `.github/workflows/*` (evidence-nightly through `qemu docker-lab`, perf-nightly through `./le perf`) | through M-1 and M-4 on ephemeral runners | inherit | runner discarded | NOT: inherit M-1 and M-4 | Q-7 |

Counts: MUST 8, SHOULD 6, NOT 11. S-2 (NTP) and S-4 (flow-export) packages are already in the guest population for their `integration` files; their new `docker` files join `docker-peers` beside them, which is allowed because the two populations are per file, not per package.

## Files to Modify
- `internal/le/interoplab/docker.go` - label append in `runContainer`, `RunOneShot`, `createNetwork`, `Build`; labeled list helpers
- `internal/le/interoplab/kernelcheck.go` - the probe through `RunOneShot`
- `internal/le/interoplab/lab.go` - `Suite.Run` sweeps after `Probe`; `preClean` label-guarded
- `internal/le/interoplab/ipsec/ipsec.go`, `internal/le/interoplab/radius/radius.go` - roots off `context.Background()` onto the signal context
- `internal/le/test/integration/actions.go` - `nativeRunner` roots on `signal.NotifyContext` (the `guestContext` shape)
- `internal/le/test/integration/gates.go` - the `cleanup` and `docker-peers` rows; `live-rpki` sets `ZE_REQUIRE_DOCKER=1`
- `internal/le/test/qemu/integration_coverage_test.go` - `integrationTestPackages` calls the moved scanner; `TestDockerTagNeverJoinsIntegration`
- `internal/le/test/qemu/alltests.go` - the `internal/le/interoplab` reason in `excludedIntegrationPackages` names only the kernel-check test (AC-14)
- `.github/workflows/evidence-nightly.yml` - the `docker-peers` job (AC-25)
- `internal/le/workflowcheck/workflowcheck_test.go` - `want` gains `test integration/docker-peers` (and `test integration/live-rpki` if Q-11 says so)
- `internal/le/test/deployment/{deployment,l2tp,vppiface,vppevidence,vppevidencerun}.go` - own docker deleted, `interoplab.Docker` used
- `internal/test/perfrunner/run.go` - same
- `internal/le/verify/evidence/evidence.go` - same
- `internal/le/site/terminaldemo/render.go` - same
- `internal/component/bgp/plugins/rpki/{rpki_live_test,rtr_downgrade_test}.go` - `labtest`
- `internal/le/interoplab/image_retention_integration_test.go` - `RunOneShot`; renamed `image_retention_docker_test.go`, tag `docker`
- `internal/le/arch/tier/*` - the AC-17 refusal; the AC-16 gate (phase 3 picks its host)
- `internal/le/session/reap.go` - only if A-6 forces the process scanner into a leaf (moved, not copied)
- Docs: `docs/architecture/testing/interop.md` ("How It Works" step 5, a new "Crash-safe teardown" section, a Debugging row for `labtest`, "Prerequisites" and "Test Hierarchy" rows for `docker-peers` and `cleanup`), `docs/architecture/testing/runner-architecture.md` ("How a suite's child processes end": the container paragraph), `docs/architecture/testing/qemu-integration.md` ("Build Tags" gains the `docker` row, "Graceful Degradation" the `ZE_REQUIRE_DOCKER` exception, "Existing Integration Test Packages" the `docker` population and the changed exclusion), `docs/architecture/testing/ci-workflows.md` (the `docker-peers` row, and `live-rpki` per Q-11), `docs/contributing/testing.md` ("Interop and integration tests" block and the cheat sheet: `docker-peers`, `cleanup`, the `docker` tag), `docs/guide/vpp.md` (deployment cleanup), `docs/functional-tests.md` (rpki live tests), `docs/architecture/module-tiers.md` (AC-17), `ai/INDEX.md` keyword row (orphan cleanup, labels, labtest, `docker` tag)

## Files to Create
- `internal/le/interoplab/orphans.go` - owner identity, label keys, verdict, `Docker.CleanupOrphans`
- `internal/le/interoplab/orphans_test.go`, `internal/le/interoplab/orphans_docker_test.go`
- `internal/le/interoplab/labtest/labtest.go`, `labtest_test.go`, `labtest_docker_test.go`
- the build-constraint tree scanner, moved from `integration_coverage_test.go` into a non-test leaf under `internal/le/test/` (AC-24)
- one `_test.go` per accepted SHOULD row (Q-5)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `./le` tooling, no Ze config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `./le test integration cleanup [dry]` and `./le test integration docker-peers`, rows in `internal/le/test/integration/gates.go` |
| CLI grammar (keyword before value) | Yes | `dry` is a bare keyword; `ai/rules/cli.md` is read in phase 1 before the verb lands |
| Editor autocomplete | N-A | not a ze CLI command |
| Functional test for new RPC/API | Yes | `TestKilledLabRunIsCleanedByNextStart`; the pasted `cleanup dry` run; one pasted `docker-peers` run |
| Pipe completeness | Yes | the cleanup report renders through leaction's `| json | yaml | table` |
| Env var registration | Yes | `ZE_REQUIRE_DOCKER`, a test-harness switch set by the verbs, declared the way `ZE_QEMU_LINUX_ONLY` is (A-9); no Ze daemon env var |
| Doctor check for runtime dependencies | N-A | no Ze runtime dependency; the orphan cleanup reads `/proc`, `/etc/machine-id` and the boot id on the development host only |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | development tooling, no `features/<id>.md` entry; a SHOULD row that adds interop evidence updates its protocol feature's evidence pointer in its own phase |
| 2 | Config syntax changed? | N-A | none |
| 3 | CLI command added/changed? | Yes | `docs/contributing/testing.md` (the `./le` reference for test verbs) |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` (the deployment proofs' cleanup) |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A for G-1..G-3; per SHOULD row if its test carries an RFC tag | that row's `rfc/short/` file |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/interop.md` (incl. "Prerequisites", "Test Hierarchy"), `docs/architecture/testing/runner-architecture.md`, `docs/architecture/testing/qemu-integration.md`, `docs/architecture/testing/ci-workflows.md`, `docs/contributing/testing.md` (block and cheat sheet), each in the phase whose change makes it wrong |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/module-tiers.md` (the labtest refusal) |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | a `./le` verb only |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/spec-lab-docker-resources-reaped-and-go-test-native.md` in phase 1; known: `interop.md` anchors `lab.go` and `docker.go`; `ci-workflows.md` and `features/interoperability-testing.md` anchor `lab.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `interop.md` "Running" examples stay valid; add the `test integration cleanup` example |

## Implementation Steps

1. **Phase: Wiring** - register `cleanup` and `docker-peers`, the `Suite.Run` sweep call, the signal context in `nativeRunner`, the moved tree scanner.
   - Tests: `TestSuiteRunCleansOrphansBeforePreflight`, `TestLabCleanupActionDispatches`, `TestNativeRunnerCancelsOnSIGINT`, `TestDockerPeersVerbRunsEveryDockerPackage`, `TestDockerTagNeverJoinsIntegration` (red)
   - Files: `lab.go`, `internal/le/test/integration/{actions,gates}.go`, `integration_coverage_test.go`, the scanner leaf
2. **Phase: Labels and verdict (G-1, G-2)** - owner identity, labels on every create, the verdict, `Docker.CleanupOrphans`, the label-guarded `preClean`, signal roots for ipsec, radius and the other entry points, the `interop.md` and `runner-architecture.md` sections.
   - Tests: every `orphans_test.go` row, `TestPreCleanRefusesLiveOrUnlabeledSameName`, `TestKilledLabRunIsCleanedByNextStart`
   - Files: `orphans.go`, `docker.go`, `kernelcheck.go`, `lab.go`, `ipsec.go`, `radius.go`
3. **Phase: MUST adoption** - M-3..M-6 and M-8 onto the wrapper, own invocations deleted; the AC-16 gate.
   - Tests: the moved argv tests, `TestNoDockerExecOutsideInteroplab`
   - Files: deployment, perfrunner, evidence, terminal demo
4. **Phase: `labtest` (G-3)** - the helper, the AC-17 refusal, the M-7 conversion, `ZE_REQUIRE_DOCKER`, the M-8 retag from `integration` to `docker`, the `docker-peers` nightly job (and `live-rpki` per Q-11), and the doc edits of qemu-integration.md, ci-workflows.md and testing.md.
   - Tests: `TestLabtestStartsServerAndCleansUp`, the converted rpki live tests, the tier selftest fixture, `TestEvidenceNightlyScheduleActionsAndPrivileges` with the new job
5. **Phase: SHOULD rows** - one sub-phase per row the owner accepts (Q-5), each with its discrimination walk.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N and every inventory row has an implementation at a file and symbol, or an owner decision recorded |
| Correctness | "gone" is reachable only through the three proven paths of AC-5; every other path is a skip with a message (R-1) |
| Correctness | no owner fact is ever read from the daemon (AC-4) |
| Naming | label keys under one prefix, declared once in `orphans.go`, documented from that declaration |
| Gating | every `docker`-tagged test is reached by `docker-peers`, `docker-peers` by a nightly job, and no `docker`-tagged file also carries `integration` |
| Data flow | every create argv gets its labels from one helper; no caller builds its own label argv |
| Rule: docker.md | no `prune`, no name-pattern removal, no volume removal anywhere in the diff |
| Rule: no-layering | each MUST site's own docker code is deleted in the same commit that routes it through the wrapper |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| labels on every create | `TestOwnerLabelsOnEveryCreate`; a `docker inspect` of a lab container, pasted |
| crash cleanup | `TestKilledLabRunIsCleanedByNextStart` output pasted |
| no stray docker exec | the AC-16 gate green, and red when its fixture adds one |
| `labtest` | `TestLabtestStartsServerAndCleansUp` and the converted rpki tests, pasted |
| `docker-peers` has a home | a pasted `./le test integration docker-peers` run, and the id of the first nightly run that executed it |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | label values read back from the daemon are untrusted text: the pid parses as a bounded integer, ids as fixed-format hex or UUID; anything else is "malformed → skip" |
| Destructive scope | the removal argv takes a resource id read from the filtered listing, never a name or a pattern |
| Command injection | argv only, no shell; label values never reach a shell |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The pre-clean by name was the only thing standing in for a reaper, and it is both too weak (it misses other pids) and too strong (it can remove a live run's same-name container).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Owner labels plus a client-side liveness sweep | (a) testcontainers-go; (b) a Ryuk-style reaper container holding a connection from the run and reaping on disconnect; (c) `--rm` everywhere plus Pdeathsig | (a) is a second orchestrator (no-layering). (b) needs the Docker socket mounted into a container (the Colima path differs), a privileged sidecar per run and a new image, and it reaps a live run when an ssh session flaps. (c) `--rm` does nothing for a `-d` daemon that never exits, and Pdeathsig kills the CLI, not the container. Labels plus a verdict handle SIGKILL, panics and lost sessions at the next start, with no new process. Forecloses: cleanup at the moment of death (it happens at the next start or `test integration cleanup`), and automatic cleanup of a run from another machine |
| Three-valued verdict, remove only on "gone" | two-valued alive/dead | an owner on another machine, or one whose identity cannot be read, is "unknown"; the principles forbid answering it as dead. Forecloses: unattended cleanup of foreign resources |
| Identity = machine id + boot id + pid + start token, taken on the client | a daemon-side pid, the hostname, the pid alone | the daemon's pid namespace is the VM's on the Mac; hostnames repeat; pids are reused. Forecloses: judging a resource whose creator's machine id could not be read |
| Images: label the build, remove only the run-owned tag | label-free images, removal by tag prefix | prefix removal would touch live runs' tags; per-run labels make each run's image exact. Forecloses: identical image ids across runs (the layer cache stays shared) |
| `labtest` under `internal/le/interoplab/labtest` | `internal/test/...`, or inside `interoplab` itself | it needs `interoplab` internals and `testing`; `internal/le` is outside production by the tier doc; a separate package keeps `testing` out of the le binary. Forecloses: shipped code using it (AC-17) |
| A signal context at each entry root | per-lab handlers | one root per entry point; the teardown code already uses `WithoutCancel`. Forecloses nothing |
| `labtest` tests carry their own `docker` tag, run by `docker-peers` whose package set is derived from the tree, scheduled nightly | tag them `integration`; tag them `live`; leave them to `./le job run` | `integration` means "runs in the QEMU guest", which has no Docker, so they would skip forever or join `excludedIntegrationPackages` and run nowhere; `live` means internet; an unrun test gates nothing (`docs/contributing/testing.md`, "A test that exists is not a test that gates"). Forecloses: a `docker` test inside the QEMU `all-tests` population |
| Missing `docker` binary skips, except under `ZE_REQUIRE_DOCKER=1`, which `docker-peers` and `live-rpki` set | always skip; always fail | always-skip contradicts gating, always-fail contradicts "Graceful Degradation" (`qemu-integration.md`) for a developer without Docker. Forecloses: a gating run that passes because Docker was absent |

## Known Limitations
- A run whose machine is gone for good leaves resources this machine reports and never removes (R-3, Q-2).
- Unlabeled leftovers from before this spec are not removed (R-8, Q-3).

## Open Questions for the Owner (each a "which way")

Owner, 2026-10-18, presented with Q-2..Q-12 and their recommendations: "go".
Every recommended answer below is therefore the decision.

| # | Question | Recommended answer |
|---|----------|--------------------|
| Q-1 | Where does the explicit sweep live, and what is it called? | DECIDED 2026-10-18: `./le test integration cleanup [dry]`; the owner rejected `lab-reap` as a name. It stays out of `./le session reap`, which judges session directories by different rules |
| Q-2 | A resource whose owner is on another machine or cannot be judged: report only, or a `foreign` keyword that removes it? | Report only, naming the machine id and the command to run there; no forced mode |
| Q-3 | Unlabeled leftovers from before this spec: never mention them, or list them (by the name shapes the labs use) as "unlabeled, not removed"? | List them in `cleanup dry` as a one-time aid; never remove them<br>**Deviation, for the owner (phase 2):** `cleanup dry` lists unlabeled `ze-interop-retained:` tags only, never unlabeled containers or networks. A retained tag carries one repository name the code already declares (`retainedTagRepository`), so matching it adds no list. Matching an unlabeled container by "the name shapes the labs use" would need a hand-kept list of every lab's container names beside the labs that declare them, a second declaration of one fact (`ai/rules/principles.md`), so it was not built. Unlabeled containers are still never removed. Test: `TestCleanupOrphansListsUnlabeledRetainedTags` |
| Q-4 | One spec in five phases, or split G-1/G-2, the MUST adoption, `labtest`, and the SHOULD rows into separate specs? | One spec, five phases, every row in scope; a SHOULD row moves to its own spec only if it proves larger than a package test (S-5 likely) |
| Q-5 | Which SHOULD rows run: S-1 TACACS+, S-2 NTP, S-3 gNMI, S-4 flow-export, S-5 DHCP, S-6 syslog? | S-1..S-4 in this spec (no real-peer proof exists for any of them); S-5 and S-6 as their own specs, because S-5 needs L2 plumbing and S-6 proves the least |
| Q-6 | In `labtest`, an image that cannot be pulled: skip or fail? (The rpki live tests skip today.) | Fail under `docker`; under `live`, skip when run ad hoc and fail when `ZE_REQUIRE_DOCKER=1` is set; a failing `Probe` fails under both |
| Q-7 | The NOT rows, especially N-4 (the appliance kernel builder in `ze-host`, journal row open): confirm NOT for this spec? | Confirm every NOT row; N-4 stays on its journal row because `ze-host` is shipped and must not import `internal/le` |
| Q-8 | Label key prefix: `ze.lab.*` or reverse-DNS `io.ze-software.lab.*`? | `ze.lab.*`: short and greppable |
| Q-9 | Names for the package-test build tag and its verb? | Tag `docker`, verb `./le test integration docker-peers` |
| Q-10 | Where the `docker-peers` nightly runs: the hosted runner's own Docker, or inside `./le test qemu docker-lab`? | The hosted runner's Docker: no container runs Ze, so the Ze-kernel check does not apply (AC-26), and the guest may lack a Go toolchain (A-8) |
| Q-11 | Give `live-rpki` a nightly job? | Yes: without one the converted rpki tests still run nowhere (R-14) |
| Q-12 | The other Docker tests that run nowhere today (the host kernel-check test and the eight VPP SRv6 tests, N-11): in this spec, or out? | Out of this spec, with one journal row for the problem class "Docker test with no runner" |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A: Scope is tooling)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-lab-docker-resources-reaped-and-go-test-native.md` only, in the same `./le commit create` script
