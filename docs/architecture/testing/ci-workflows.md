# What Actually Runs Each Suite

Validation runs on GitHub Actions (`.github/workflows/`), not Codeberg. The
repository is pushed to both codeberg.org and github.com/ze-software/ze. CI is
on GitHub for two reasons: running heavy nightly sweeps on Codeberg's donated
shared runners is inconsiderate of a free service, and GitHub's `ubuntu-latest`
grants the root and `CAP_NET_ADMIN` the integration suites need, which the
shared Woodpecker instance could not.

## Where each suite runs

| Suite | Workflow and job | Trigger | Blocking |
|-------|------------------|---------|----------|
| Every native verification stage. The job reads its list from `./le verify list mode full` and runs each stage as a native action | `verify.yml`, job `verify` | push and pull_request | yes |
| `./le fuzz run` | `evidence-nightly.yml`, job `fuzz` | schedule `17 3 * * *` | advisory |
| `./le integration iface`, `fib`, `firewall`, `traffic`, `gtsm`, `as112` | `evidence-nightly.yml`, job `integration`, under `sudo` | schedule | advisory |
| `./le integration interop` | `evidence-nightly.yml`, job `interop` | schedule | advisory |
| `./le integration interop-ipsec` | `evidence-nightly.yml`, job `ipsec-interop` | schedule | advisory |
| `./le deployment docker-l2tp-ppp-test` | `evidence-nightly.yml`, job `l2tp-interop` | schedule | advisory |
| `./le deployment docker-pppoe-accel-test` | `evidence-nightly.yml`, job `pppoe-interop` | schedule | advisory |
| `./le qemu all-tests`, inside a guest booting the runtime kernel | `qemu-nightly.yml`, job `needs-linux` | schedule `43 4 * * *` | advisory |
| The LDP, IS-IS and VRRP protocol labs, each inside `./le qemu run` | `qemu-nightly.yml`, job `protocol-labs` | schedule | advisory |
| The L2TP appliance proof, the PPPoE labs and the traffic-usage eBPF proof | `qemu-nightly.yml`, job `runtime-kernel-labs` | schedule | advisory |
| `bin/ze-perf track --check` against `test/perf/history/ze.ndjson` | `perf-nightly.yml`, job `perf-regression-check` | schedule `42 3 * * *` | advisory | <!-- doc-links: ignore (the history file is written by the nightly job and is not committed; perf-nightly.yml:33 guards on its absence) -->
| `./le verify deps vulnerability` | `govulncheck.yml` | schedule `37 5 * * *` | advisory |
| CodeQL | `codeql.yml` | push, pull_request, schedule `21 16 * * 3` | as configured by the action |

Every job in the three nightly workflows carries `continue-on-error: true`: a
red suite reports without marking the run failed. The nightly may run under TCG
emulation, so it is slower than the merge gate and reports rather than blocks.
Run the QEMU target locally when you add a test, and say so.

## Why the privileged suites are on GitHub

The native integration actions need `CAP_NET_ADMIN` or
`CAP_NET_BIND_SERVICE`. A GitHub job can run the action with those privileges;
the shared Codeberg runner cannot grant them.

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
direct native action is `verify list`, that each nightly is scheduled-only with
every job advisory, that each nightly invokes the native actions it is supposed
to, that every `./le` action a workflow names is registered in the Go action
tables, and that a capability-gated `.ci` test has a VM home.

<!-- source: .github/workflows/verify.yml -- the merge gate -->
<!-- source: .github/workflows/evidence-nightly.yml -- fuzz, integration, interop, ipsec-interop, l2tp-interop, pppoe-interop -->
<!-- source: .github/workflows/qemu-nightly.yml -- needs-linux, protocol-labs, runtime-kernel-labs -->
<!-- source: internal/le/workflowcheck/workflowcheck_test.go -- TestVerifyWorkflowIsTheFastMergeGate, TestEvidenceNightlyScheduleActionsAndPrivileges, TestQEMUNightlyScheduleActionsCachesAndBudgets, TestEveryWorkflowNativeActionExists, TestCapabilityGatedTestsHaveANativeVMHome -->
