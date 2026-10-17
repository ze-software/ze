# What Actually Runs Each Suite

Validation runs on GitHub Actions (`.github/workflows/`). The repository lives
at github.com/ze-software/ze and is pushed nowhere else. GitHub's
`ubuntu-latest` grants the root and `CAP_NET_ADMIN` the integration suites
need.

## Where each suite runs

| Suite | Workflow and job | Trigger | Blocking |
|-------|------------------|---------|----------|
| Every native verification stage. The job reads its list from `./le verify list mode full` and runs each stage as a native action | `verify.yml`, job `verify` | push and pull_request | yes |
| `./le test fuzz run` | `evidence-nightly.yml`, job `fuzz` | schedule `17 3 * * *` | advisory |
| `./le test integration iface`, `fib`, `firewall`, `traffic`, `gtsm`, `as112` | `evidence-nightly.yml`, job `integration`, under `sudo` | schedule | advisory |
| `./le test integration interop`, inside `./le test qemu docker-lab` | `evidence-nightly.yml`, job `interop` | schedule | advisory |
| `./le test integration interop-ipsec`, inside `./le test qemu docker-lab` | `evidence-nightly.yml`, job `ipsec-interop` | schedule | advisory |
| `./le test integration interop-radius`, inside `./le test qemu docker-lab` | `evidence-nightly.yml`, job `radius-interop` | schedule | advisory |
| `./le test deployment docker-l2tp-ppp-test`, inside `./le test qemu docker-lab` | `evidence-nightly.yml`, job `l2tp-interop` | schedule | advisory |
| `./le test deployment docker-pppoe-accel-test`, inside `./le test qemu docker-lab` | `evidence-nightly.yml`, job `pppoe-interop` | schedule | advisory |
| `./le test qemu all-tests`, inside a guest booting the runtime kernel | `qemu-nightly.yml`, job `needs-linux` | schedule `43 4 * * *` | advisory |
| The LDP, IS-IS and VRRP protocol labs, each inside `./le test qemu run` | `qemu-nightly.yml`, job `protocol-labs` | schedule | advisory |
| The L2TP appliance proof, the PPPoE labs, the ASPA and RFC 2545 plugin namespace subset, and the traffic-usage eBPF proof | `qemu-nightly.yml`, job `runtime-kernel-labs` | schedule | advisory |
| `./le perf track --check` against `test/perf/history/ze.ndjson` | `perf-nightly.yml`, job `perf-regression-check` | schedule `42 3 * * *` | advisory | <!-- doc-links: ignore (the history file is written by the nightly job and is not committed; perf-nightly.yml:33 guards on its absence) -->
| `./le verify deps vulnerability` | `govulncheck.yml` | schedule `37 5 * * *` | advisory |
| CodeQL | `codeql.yml` | push, pull_request, schedule `21 16 * * 3` | as configured by the action |

## What every verify shard prepares

Any stage can land on any shard, so each of the six `verify` jobs prepares the
runner the same way before it reads its stage list:

| Step | Why |
|------|-----|
| `git archive` of `origin/gh-pages` into `../gh-pages` | A site build seeds itself from the last published artifact beside the main checkout, and the recorded terminal demos exist only there: a render needs the recorder image, which is built locally and published to no registry. Without the tree, `TestBuildRendersEveryPublishedRoute` refuses with "this checkout holds no rendered terminal demonstrations" |
| `sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0`, when the kernel has the key | Ubuntu 24.04 lets AppArmor restrict unprivileged user namespaces, and the unit tests that build their own network namespace (bfd, ike/engine, ospf, internal/test/userns) then fail `lo up: operation not permitted`. Owner decision, 2026-10-17 |
| `sudo modprobe` of `l2tp_core`, `l2tp_netlink`, `pppox` and `l2tp_ppp` | The l2tp kernel capability probes the l2tp generic netlink family (`CONFIG_L2TP`) and the PPPoL2TP socket (`CONFIG_PPPOL2TP`); the hosted kernel ships both as modules it has not loaded. A module the image lacks fails the job with its name and the package that carries it |
| `go install -mod=vendor` of golangci-lint and staticcheck | Both build from `vendor/`, so CI runs the release `go.mod` requires. An `@version` pin is a second copy of that fact, and it drifted: v2.13.1 against `go.mod`'s v2.14.0 cannot read Go 1.27 export data |

The race pass is three stages rather than one (`verify-freshness-scope.md`),
because on a CI clone it races the whole tree, and the undivided stage outran
the 120-minute job bound.

A Docker lab prints one progress line as each scenario starts and ends
(`Suite.Progress`), and `docker-lab` streams the guest's output over ssh as it
arrives, so a lab that hangs names its scenario in the job log before the
deadline kills it.

A nightly is advisory because no merge waits on it, not because its jobs hide a
failure. The jobs of `evidence-nightly.yml` and `perf-nightly.yml` state their
own verdict: a red suite marks the run failed. The three jobs of
`qemu-nightly.yml` still carry `continue-on-error: true`, so a red suite there
reports without marking the run failed. A `qemu-nightly.yml` guest may run under
TCG emulation, so it is slower than the merge gate. Run the QEMU target locally
when you add a test, and say so.

## The Docker labs run on Ze's kernel

A Docker lab runs Ze in containers, so the kernel under test is the Docker
host's. Ze enrolls features the hosted runner's kernel lacks, so the five Docker
lab jobs (`interop`, `ipsec-interop`, `radius-interop`, `l2tp-interop`,
`pppoe-interop`) do not run on it. Each restores the amd64 runtime kernel cache entry under the key
`qemu-nightly.yml` uses, builds the entry on a miss with `./ze-host appliance
kernel --target runtime --arch amd64`, and saves it. It then runs its lab as
`./le test qemu docker-lab lab "<le words>"`: an Alpine guest boots that kernel
under KVM, never TCG, starts Docker, runs `./le setup docker-kernel check` and
only then the lab. A last step runs the same check against the hosted runner's
own Docker daemon, as information: it may fail without failing the job, and its
log shows the day GitHub's kernel starts to pass. `radius-interop` is among
them although its admin path needs no kernel feature: the lab stages a ze that
enrols every capability, and the interop harness checks that ze against the
Docker kernel for every suite, so on the runner's kernel it would be refused.
`docs/architecture/testing/qemu-integration.md`, "Docker labs in the Ze-kernel
guest", is the guest path.

The RFC ledger credits a lab run this way to the workflow that schedules it.
`docker-lab` declares its `lab` keyword as `le-words`
(`leaction.ValueLeWords`), and the workflow reader in `internal/le/rfc`
credits the command in that value as well as `docker-lab` itself.

The ASPA and RFC 2545 replay cases run through `qemu netns-test suites plugin`,
whose closed selection contains the seven `rpki-aspa-*` validation and policy
carriers plus `adj-rib-in-replay-rfc2545-next-hop`.
It uses per-test namespaces and the existing functional registry's `bgp plugin`
command prefix. `all-tests` still runs the full plugin suite in the guest root
namespace, where the ASPA cases' `netns-link` prerequisite causes a skip.
The replay carrier instead owns distinct speaker, recipient and next-hop
namespaces and has no `netns-link` prerequisite. It can run when its Linux
privilege requirements are met; the separately scheduled subset remains.

## Why the privileged suites are on GitHub

The native integration actions need `CAP_NET_ADMIN` or
`CAP_NET_BIND_SERVICE`. A GitHub job can run the action with those privileges.

## The cron lives in the workflow

Each schedule is declared in its own workflow file (`on: schedule: - cron:`), so
merging the file to the default branch creates the schedule. Woodpecker kept the
cron as a repository setting that nothing in the repository recorded. One caveat
comes with the GitHub form: GitHub disables a scheduled workflow after 60 days
with no repository activity, so a long quiet period silently stops the nightly.
Each nightly therefore also declares `workflow_dispatch`, which is the re-arm.

## What pins the workflow set

`internal/le/workflowcheck/workflowcheck_test.go` pins the shape of these files.
It asserts that `verify.yml` stays a push and pull_request gate whose only
direct native action is `verify list`, that every verify shard lifts the
user-namespace restriction and loads the L2TP modules before its first stage,
that each nightly is scheduled-only with every job advisory, that each nightly
invokes the native actions it is supposed to, that the four Docker lab jobs
reach their labs through `docker-lab` on the cached runtime kernel, that every `./le` action a workflow names is registered
in the Go action tables, and that a capability-gated `.ci` test has a VM home.

<!-- source: .github/workflows/verify.yml -- the merge gate -->
<!-- source: internal/le/interoplab/lab.go -- Suite.Run, Suite.progress, scenarioLine -->
<!-- source: internal/le/test/qemu/run_exec.go -- Run.sshRun -->
<!-- source: internal/component/l2tp/kernelcap_linux.go -- l2tpKernelCapabilities -->
<!-- source: .github/workflows/evidence-nightly.yml -- fuzz, integration, interop, ipsec-interop, radius-interop, l2tp-interop, pppoe-interop -->
<!-- source: internal/le/test/qemu/dockerlab.go -- runDockerLabHere -->
<!-- source: internal/le/rfc/carriers.go -- workflowNestedLe, nestedLeWords -->
<!-- source: .github/workflows/qemu-nightly.yml -- needs-linux, protocol-labs, runtime-kernel-labs -->
<!-- source: internal/le/workflowcheck/workflowcheck_test.go -- TestVerifyWorkflowIsTheFastMergeGate, TestVerifyPreparesTheRunnerKernelBeforeReadingStages, TestVerifyInstallsPinnedNativeTools, TestEvidenceNightlyScheduleActionsAndPrivileges, TestQEMUNightlyScheduleActionsCachesAndBudgets, TestEveryWorkflowNativeActionExists, TestCapabilityGatedTestsHaveANativeVMHome -->
