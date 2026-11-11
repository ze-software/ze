# What Actually Runs Each Suite

Validation runs on GitHub Actions (`.github/workflows/`). The repository lives
at github.com/ze-software/ze and is pushed nowhere else. GitHub's
`ubuntu-latest` grants the root and `CAP_NET_ADMIN` the integration suites
need. A unit test that needs kernel privilege does not use that root: it runs
in a user namespace of its own (`userns.Enter`), so it runs the same way on a
developer host.
<!-- source: internal/test/userns/userns_linux.go -- Enter, childSetup, childFailure -->

## Where each suite runs

| Suite | Workflow and job | Trigger | Blocking |
|-------|------------------|---------|----------|
| Every native verification stage. The job reads its list from `./le verify list full` and runs each stage as a native action | `verify.yml`, job `verify` | push and pull_request | yes |
| `./le test fuzz run` | `evidence-nightly.yml`, job `fuzz` | schedule `17 3 * * *` | advisory |
| `./le test integration docker-peers`: every package test tagged `docker`, on the hosted runner's own Docker, since no container runs Ze | `evidence-nightly.yml`, job `docker-peers` | schedule | advisory |
| `./le test integration live-rpki`, against a real stayrtr on the hosted runner's own Docker | `evidence-nightly.yml`, job `live-rpki` | schedule | advisory |
| `./le test integration iface`, `fib`, `firewall`, `traffic`, `gtsm`, `as112` | `evidence-nightly.yml`, job `integration`, under `sudo` | schedule | advisory |
| `./le test integration interop`, inside `./le test qemu docker-lab` | `evidence-nightly.yml`, job `interop` | schedule | advisory |
| `./le test integration interop-ipsec`, inside `./le test qemu docker-lab` | `evidence-nightly.yml`, job `ipsec-interop` | schedule | advisory |
| `./le test integration interop-radius`, inside `./le test qemu docker-lab` | `evidence-nightly.yml`, job `radius-interop` | schedule | advisory |
| `./le test deployment docker-l2tp-ppp`, inside `./le test qemu docker-lab` | `evidence-nightly.yml`, job `l2tp-interop` | schedule | advisory |
| `./le test deployment docker-pppoe-accel`, inside `./le test qemu docker-lab` | `evidence-nightly.yml`, job `pppoe-interop` | schedule | advisory |
| `./le test qemu all`, inside a guest booting the runtime kernel | `qemu-nightly.yml`, job `needs-linux` | schedule `43 4 * * *` | advisory |
| The LDP, IS-IS and VRRP protocol labs, each inside `./le test qemu run` | `qemu-nightly.yml`, job `protocol-labs` | schedule | advisory |
| The L2TP appliance proof, the accel-ppp lab and the traffic-usage eBPF proof, plus the PPPoE suite and the ASPA and RFC 2545 plugin namespace subset inside `./le test qemu docker-lab harness` | `qemu-nightly.yml`, job `runtime-kernel-labs` | schedule | advisory |
| `./le perf track --check` against `test/perf/history/ze.ndjson` | `perf-nightly.yml`, job `perf-regression-check` | schedule `42 3 * * *` | advisory | <!-- doc-links: ignore (the history file is written by the nightly job and is not committed; perf-nightly.yml:33 guards on its absence) -->
| `./le verify deps vulnerability` | `govulncheck.yml` | schedule `37 5 * * *` | advisory |
| CodeQL | `codeql.yml` | push, pull_request, schedule `21 16 * * 3` | as configured by the action |
| Re-applying the vendor patches (`internal/le/vendorpatch/patches`) that a Dependabot Go update drops, then dispatching `verify.yml` on the patched commit | `dependabot-vendor-patches.yml`, job `apply` | push to `dependabot/go_modules/**` by Dependabot | yes: a patch that no longer applies fails it |

"Advisory" means only that the job does not block a push. A job that fails
before its first test is broken, not advisory: nothing it would have run is
judged until it is fixed. A step a job cannot run without (a binary it builds,
a restriction it lifts) is pinned by a test in `internal/le/workflowcheck`,
which goes red when the step leaves the workflow.
<!-- source: internal/le/workflowcheck/qemu_guest_binaries_test.go -- TestQEMUNightlyBuildsEveryGuestDaemon -->
<!-- source: internal/le/workflowcheck/docker_peers_userns_test.go -- TestDockerPeersLiftsTheUsernsRestrictionBeforeItsTests -->

### CodeQL's Go population

The Go job selects the patched GOROOT with `./le go stdpatch goroot` and builds
the distro, appliance and setup tag contexts. The tree comes from le rather
than from the bootstrap generator because the build step runs `./le` too: the
generator run first made `cache/` a real directory, the launcher moved it aside
to link `cache/` to `~/.cache/ze`, and the exported GOROOT went with it (run
42915194844). Started first, le makes `cache/` the link before the tree is
written behind it. CodeQL initializes after that step, against the Go
executable the traced builds use, so its Go-path capture and tracing wrapper
see the patched toolchain.

`CGO_ENABLED=0` binds the extraction contexts, not the GOROOT selection. The
prepare, build and check steps set it because cgo build constraints decide
which files a package holds, so the prepared population, the traced builds and
the checked database must agree on it, and `codeql_build` refuses any other
assignment. The selection step compiles nothing: it lists the standard library
under both cgo settings itself, and the launcher builds le with cgo off
whatever the caller sets.
<!-- source: .github/workflows/codeql.yml -- Select the patched Go standard library, Initialize CodeQL -->
<!-- source: internal/le/go/stdpatch/gen/tree.go -- listStdlib, embedSet -->
<!-- source: internal/le/repo/codeql/context.go -- CGO_ENABLED -->
<!-- source: le -- ensure_cache_link, build_le -->

Each build passes its actual tag argument to
`./le repo codeql packages tags '<tags>'`; no second tag list or authored-root
list supplies the population. The native selector derives package directories
from tracked Go files in that complete Go build context. Vendor, the
`gokrazy/modcache` cache, `testdata`, root tool pins, ruleguard data and the
separate-module plugin example are explicit classifications. Unsupported
tracked forms refuse rather than disappear.

Before returning any package arguments, the selector runs strict
`go list -deps` for those roots in the same context and checks canonical package
and source paths. A direct cache root, or an authored dependency or source
symlink resolving into `gokrazy/modcache`, refuses before the traced build.
The workflow checks the selector's exit status and refuses an empty answer;
there is no `./...` fallback. `paths-ignore` is not extraction evidence.

Before initialization, `./le repo codeql prepare file <manifest> tags ze_setup`
reserves a new manifest, identifies the tracked
`test/codeql/authored_sentinel.go` and creates one tiny sentinel in its own
`gokrazy/modcache/codeql-extraction-*` directory. It calls the same `Guard` with
that exact cache package and saves the actual refusal, source paths, hashes and
tag context. An occupied manifest refuses before any cache input is created.
The authored package is independent of the launcher bootstrap; only the normal
tracked build population selects it.

After the normal security analysis and upload,
`./le repo codeql check file <manifest> database <database> executable <codeql>`
uses the database and executable reported by the pinned CodeQL actions. It
checks manifest confinement and both source hashes before invoking CodeQL, then
queries every extracted `GoFile` for relative and absolute paths. Files without
a repository-relative path remain in the absolute-path check. The CSV decoder
checks every row through the existing canonical cache boundary, including older
cache files and aliases, before requiring the exact authored sentinel. Empty,
malformed and unresolved answers refuse rather than certify exclusion. The
source hashes are checked again before a successful report is saved.

The query and its tiny QL pack live in a new task-owned directory beside the
manifest. The pack resolves the Go library already in the selected CodeQL
bundle; the checker installs or downloads nothing and a missing library fails
the query. Compilation caches and intermediate BQRS are removed when the check
ends. The workflow retains only the manifest, query, pack, decoded CSV and
report in `codeql-go-extraction-proof`, with a 14-day artifact lifetime.

Local fixture builds and synthetic CodeQL-process answers prove these native
admission and checking boundaries. Actual extraction remains proved only by a
successful CI Go database check on the named commit; no local selector result
or synthetic CSV discharges that evidence debt. A manual run keeps its manifest,
owned cache sentinel and small proof directory until that run's evidence is
reviewed, then removes those exact task-owned paths. It never removes another
cache entry or copies the full database as a proof artifact.
<!-- source: internal/le/repo/codeql/population.go -- Select, Guard -->
<!-- source: internal/le/repo/codeql/extraction_prepare.go -- PrepareExtraction -->
<!-- source: internal/le/repo/codeql/extraction_check.go -- CheckExtraction -->
<!-- source: .github/workflows/codeql.yml -- Build Go shipped feature combinations -->
<!-- source: .github/workflows/codeql.yml -- Prepare Go extraction evidence, Check Go extraction evidence, Retain Go extraction evidence -->

## What every verify shard prepares

Any stage can land on any shard, so each of the six `verify` jobs prepares the
runner the same way before it reads its stage list:

| Step | Why |
|------|-----|
| `git archive` of `origin/gh-pages` into `../gh-pages` | A site build seeds itself from the last published artifact beside the main checkout, and the recorded terminal demos exist only there: a render needs the recorder image, which is built locally and published to no registry. Without the tree, `TestBuildRendersEveryPublishedRoute` refuses with "this checkout holds no rendered terminal demonstrations" |
| `sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0`, when the kernel has the key | Ubuntu 24.04 lets AppArmor restrict unprivileged user namespaces. Every test that calls `userns.Enter` (`internal/test/userns`) builds its own user, network and mount namespace, and most kernel-touching tests do: links, routes, XFRM, nftables, raw sockets, named network namespaces. Under the restriction the child holds no capability, its first setup step fails with `operation not permitted`, and `Enter` fails the test rather than skipping it. Owner decision, 2026-10-17 |
| `linux-modules-extra-$(uname -r)` installed when the archive publishes one, then `sudo modprobe` of `l2tp_core`, `l2tp_netlink`, `pppox` and `l2tp_ppp` | The l2tp kernel capability probes the l2tp generic netlink family (`CONFIG_L2TP`) and the PPPoL2TP socket (`CONFIG_PPPOL2TP`); up to 6.17 the hosted azure kernel ships `l2tp_core`, `l2tp_netlink` and `l2tp_ppp` only in its `linux-modules-extra` package (`pppox` is in the base package), unloaded. From 7.0 Ubuntu publishes no extra package for the azure kernel, so the install is skipped there rather than failing every shard at setup. A module the image lacks fails the job with its name |
| `go install -mod=vendor` of golangci-lint, staticcheck and govulncheck | All three build from `vendor/`, so CI runs the release `go.mod` requires (`govulncheck.yml` installs govulncheck the same way). An `@version` pin is a second copy of that fact, and it drifted twice: golangci-lint v2.13.1 against `go.mod`'s v2.14.0 cannot read Go 1.27 export data, and govulncheck v1.4.0 against v1.7.0 panicked in its x/tools v0.46.0 SSA builder (`ForEachElement called on type containing *types.TypeParam`, run 40867375691) |

The race pass is three stages rather than one (`verify-freshness-scope.md`),
because on a CI clone it races the whole tree, and the undivided stage outran
what was then a 120-minute job bound.

## What a verify shard keeps between runs

le builds through `~/.cache/ze/go-cache` and lints through
`~/.cache/ze/golangci-lint-cache` (`gotoolchain.GoCache`, `LintCache`; the
launcher links `cache/` to `~/.cache/ze`). setup-go caches `~/.cache/go-build`,
which le never uses, so until each shard kept its own, every shard compiled and
linted from nothing: about 9 minutes of compile before each Staticcheck piece,
and on shard 1 of run 41295105997 a 79-minute `go lint run` that ended in a
cancellation at the 120-minute bound. The job bound is now 180 minutes, so
the first run after a key change, which starts cold, can finish and save.

| Step | What it does |
|------|--------------|
| Restore | Both directories, keyed `ze-go-build-<os>-go<version>-<hash of go.sum and both patch sets>-shard<n>of<count>-<commit>`, falling back to the shard's latest entry, then to any entry of the same toolchain. Both caches are content-addressed, so an entry from an older tree is reused or ignored, never wrong |
| Mark unused | Every restored entry is set two hours back, then a stamp is taken. Go and golangci-lint refresh an entry's mtime when they use one an hour old or more, so at the end an entry newer than the stamp is one the run wrote or used |
| Drop unused | After the stage loop, every entry no newer than the stamp is deleted, so the entry saved holds what this shard's stages used and nothing older |
| Measure | The two directories are compressed with zstd, as the cache action does, and compared with `GO_CACHE_SAVE_LIMIT_MIB` (1024). A shard past it saves nothing and says so |
| Replace | A push to the default branch deletes the shard's previous entry, then saves the new one. Any other run restores the default branch's entries and saves none |

One entry per shard at the limit is 6 GiB, which leaves room in the
repository's 10 GB of Actions cache for the Staticcheck pieces and the nightly
kernel and CodeQL caches, so a save does not evict the runtime kernel the QEMU
nightly takes about 30 minutes to rebuild. The trim is shell in the workflow,
not `./le scratch clean`: verify.yml runs no native action but `verify list`
(`TestVerifyWorkflowIsTheFastMergeGate`).
<!-- source: .github/workflows/verify.yml -- Restore le's Go build and lint caches -->

A Docker lab prints one progress line as each scenario starts and ends
(`Suite.Progress`), and under `docker-lab` the host le running the lab streams it as it
arrives, so a lab that hangs names its scenario in the job log before the
deadline kills it. Before its first scenario, `Suite.Run` also prints one line
for each labeled container, network or retained tag it removed or kept from an
earlier lab run, so a job log names what a crashed run left behind.

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
`./le test qemu docker-lab lab "<le words>"`: Ze's docker-host image for amd64,
built from that kernel on a miss of its own cache, boots under KVM, never TCG;
its Docker socket is forwarded over ssh, and the runner's own le runs
`./le setup docker kernel check` against it and only then the lab. A last step runs the same check against the hosted runner's
own Docker daemon, as information: it may fail without failing the job, and its
log shows the day GitHub's kernel starts to pass. `radius-interop` is among
them although its admin path needs no kernel feature: the lab stages a ze that
enrols every capability, and the interop harness checks that ze against the
Docker kernel for every suite, so on the runner's kernel it would be refused.
`docs/architecture/testing/qemu-integration.md`, "Docker labs on the
docker-host image", is the guest path.

The RFC ledger credits a lab run this way to the workflow that schedules it.
`docker-lab` declares its `lab` keyword of kind `words`
(`leaction.KindWords`), and the workflow reader in `internal/le/rfc`
credits the command in that value as well as `docker-lab` itself.

The ASPA and RFC 2545 replay cases run through `qemu netns suites plugin`,
whose closed selection contains the seven `rpki-aspa-*` validation and policy
carriers plus `adj-rib-in-replay-rfc2545-next-hop` and
`redistribute-chain-static-ospf` (static routes judged by the redistribute
import chain into OSPF, a `netns-link` case) and `ddos-detect-internal-blind`
(an armed ddos detector that sees no received traffic is reported blind by
`show doctor`, a `netns-link` case).
It uses per-test namespaces and the existing functional registry's `bgp plugin`
command prefix. `test qemu all` still runs the full plugin suite in the guest root
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
that a shard keeps le's build caches as described above, that every workflow
file parses as YAML,
that each nightly is scheduled-only with every job advisory, that each nightly
invokes the native actions it is supposed to, that a Dependabot Go branch gets
every vendor patch back and a verify run on the result, that the four Docker lab jobs
reach their labs through `docker-lab` on the cached runtime kernel, that every `./le` action a workflow names is registered
in the Go action tables, and that a capability-gated `.ci` test has a VM home.

<!-- source: .github/workflows/verify.yml -- the merge gate -->
<!-- source: internal/le/interoplab/lab.go -- Suite.Run, Suite.progress, scenarioLine -->
<!-- source: internal/le/test/qemu/run_exec.go -- Run.sshRun -->
<!-- source: internal/component/l2tp/kernelcap_linux.go -- l2tpKernelCapabilities -->
<!-- source: .github/workflows/evidence-nightly.yml -- fuzz, integration, interop, ipsec-interop, radius-interop, l2tp-interop, pppoe-interop -->
<!-- source: internal/le/test/qemu/dockerlab.go -- runDockerLabHere -->
<!-- source: internal/le/rfc/carriers.go -- nativeActionsIn, actionsInCommand, nestedLeWords -->
<!-- source: .github/workflows/qemu-nightly.yml -- needs-linux, protocol-labs, runtime-kernel-labs -->
<!-- source: internal/le/workflowcheck/workflowcheck_test.go -- TestVerifyWorkflowIsTheFastMergeGate, TestVerifyPreparesTheRunnerKernelBeforeReadingStages, TestVerifyInstallsPinnedNativeTools, TestEvidenceNightlyScheduleActionsAndPrivileges, TestQEMUNightlyScheduleActionsCachesAndBudgets, TestEveryWorkflowNativeActionExists, TestCapabilityGatedTestsHaveANativeVMHome, TestDependabotBranchesGetTheVendorPatches, TestVerifyKeepsLeBuildCachesBetweenRuns, TestEveryWorkflowParsesAsYAML -->
<!-- source: .github/workflows/dependabot-vendor-patches.yml -- apply -->
