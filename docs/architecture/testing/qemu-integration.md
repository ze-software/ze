# QEMU Integration Testing

Ze runs on Linux (gokrazy appliances, Debian/Ubuntu servers). Code that uses
Linux-only syscalls (termios, netlink, nftables, sysctl) needs a Linux guest on
macOS and Docker Desktop. Platform proof requires **Ze's own kernel and system**.
The legacy `test qemu run` route described below still boots Alpine userland;
even with Ze's kernel, it does not establish own-system proof. The existing
`test qemu docker-lab` route boots Ze's docker-host image. Its `lab` branch runs
le on the host against the guest Docker daemon; its `command` branch runs
an unprivileged command without Docker. See "One command in Ze's own system"
for the native validation and its limits.

## Quick Start

```bash
# Build the host driver, then the amd64 runtime kernel with Docker.
CGO_ENABLED=0 ./le --name qemu-setup build-artifacts host
./ze-host appliance kernel --target runtime --arch amd64 --builder docker

```

The kernel builder prints its output directory. This example targets an amd64
guest. Use the matching architecture for other
guests. `build-artifacts host` builds only `ze-host`; it does not stage a kernel.

Run the guest commands below through the command branch described in
"One command in Ze's own system", with its `harness` switch: it builds the guest
le, binds the main checkout at `/workspace`, names the caches and runs the command
as root. Cross-build the daemons first (`./le test qemu guest-binaries`).
The legacy Alpine bootstrap is not an entry point for `test qemu all`.

The Go release the guest unpacks is DERIVED from the `go` directive of `go.mod`
(`goversion.DeclaredRelease`), so the guest and the host compile with one
toolchain. It was a constant in `internal/le/test/qemu/run.go` until 2026-09-05, went
a minor behind, and every guest `go` command then downloaded a second toolchain
until the run died on `error: command ssh exceeded its deadline`.

That unpacked release is the guest's real GOROOT (`/usr/local/go`), and no guest
compile uses it directly. Ze compiles only against its patched `crypto/tls`, so
both guest profiles, the setup command and the keep-alive login profile
(`/etc/profile.d/ze.sh`), then run `go run ./internal/le/go/stdpatch/goroot` in
`/workspace` and export its answer as `GOROOT`, with `GOTOOLCHAIN=local` and the
tree's `bin` first on `PATH` (`guestPatchedTree`). The generator writes its tree
to `/workspace/cache/stdpatch`, and the same line first mounts a guest-only tmpfs
there (`guestStdpatchCache`), so the tree stays in the guest. Every link in it
names the guest's `/usr/local/go`, which does not exist on the host: written
through the 9p share, it broke every host walk of the checkout, and the pretool
hook then refused each Bash read under `ai/` (2026-10-15). A marker file inside
the tmpfs makes the login profile mount it only once. A refusal stops the setup
command; in a login shell it leaves the real GOROOT, where a build of
`internal/core/eap` fails on the compile-time marker
(`docs/contributing/patching-the-standard-library.md`).
<!-- source: internal/le/test/qemu/run.go -- guestPatchedTree, setupCommand -->
<!-- source: internal/le/test/qemu/run_exec.go -- keepAliveProfile -->

The setup command also mounts cgroup2 at `/sys/fs/cgroup` when no cgroup2 is
mounted (`guestCgroup2Mount`). Ze's own system mounts it at boot (gokrazy's
`mountfs`), as systemd does, but the Alpine guest's OpenRC does not, and without
it the default `vpn ipsec application-override deny` cannot attach its guard, so
every IPsec apply in the guest would be refused.
<!-- source: internal/le/test/qemu/run.go -- guestCgroup2Mount, setupCommand -->

Prerequisites: `qemu` (`brew install qemu` on macOS). On macOS the run uses HVF
acceleration when it is available and falls back to TCG software emulation.

On Linux the invoking user has to be in the `kvm` group. `/dev/kvm` is
`root:kvm` 0660, so a user outside the group gets no run rather than a slow one:
QEMU exits with `Could not access KVM kernel module: Permission denied` and the
caller reports the generic "did not reach SSH within the timeout", which reads
as flakiness. `./le setup install` checks this as `kvm-access` and applies
`sudo usermod -aG kvm $USER`. The new group only reaches a new login, so use
`sg kvm -c '<command>'` in an existing shell. A host with no `/dev/kvm` reports
`n/a` and runs under TCG.

Native Linux arm64 requests `kvm:tcg`, not Apple's `hvf:tcg`. QEMU tries KVM
before its software fallback. Hardware-only runs still require the host's usable
hypervisor and do not permit TCG.
<!-- source: internal/le/test/qemu/run.go -- acceleration -->

The first run fetches the Alpine ISO and Go toolchain. Alpine is cached under
`${XDG_CACHE_HOME:-$HOME/.cache}/ze/alpine-iso/`; Go archives remain under
`tmp/qemu/go-dl/`. Later runs reuse those downloads.

### The two entry points

The own-system command carrier can run a whole population in one VM. These
commands execute inside that guest; `test qemu all` runs as root.

| Command | Population |
|---------|------------|
| `./le test qemu netns suites <comma-separated-suites>` | The explicit kernel-dependent subset for each selected suite. `plugin` selects the seven ASPA cases, the RFC 2545 next-hop replay case, the static-into-OSPF redistribution chain case and the blind ddos-detect case |
| `le test qemu all` | Every functional suite, the Linux unit pass, the installer phase, and every registered integration package. Six suites run in a per-test network namespace, requiring full `ip`, `setcap` and `getcap` tools |
| `le test qemu all only needs-linux` | The same suites, each narrowed to the `.ci` tests marked `option=needs-linux`. The unit, installer and integration phases stay whole, and the report names the population it covered |
| `le test qemu all test test/<dir>/<name>.ci` | That one `.ci`, run from inside the guest; the host command that boots a guest for it is `./le test qemu ci test test/<dir>/<name>.ci` ([below](#running-one-ci-test-in-a-ze-guest)). It runs through the suite that walks its directory (`vmSuiteFor`); a draft, `test/draft/<dir>/<name>.ci`, runs through the same suite with `--draft`. Either takes the same shim, environment and network-namespace preparation. No other suite or phase runs. `./le feature run` and `./le rfc record` reach a capability-gated test this way, through `testqemu.OneCICommand` under `docker-lab harness` |

`test qemu all` discovers the suites' tests through the native runner, so a new
`needs-linux` test needs no registration there. `netns` uses the explicit
named populations in `netnsSelections`; selecting a suite does not run every
test in its directory.

Functional suite children run under a native deadline, without an external
`timeout` executable. `ZE_QEMU_SUITE_TIMEOUT` is a positive Go duration such as
the default `900s`; malformed, zero and negative values refuse before starting
children. On expiry the lifecycle owner sends TERM to the whole process group,
grants fifteen seconds, then sends KILL if needed and joins the owner. Ordinary
exit codes and the streamed test-count summary are retained. TERM expiry answers
124; a child killed by KILL answers 137, preserving the previous wrapper's
status contract.
<!-- source: internal/le/test/qemu/alltests_deadline.go -- allTestsRun.suiteChild, allTestsRun.suiteTimeout -->
<!-- test: internal/le/test/qemu/alltests_timeout_linux_test.go -- TestQEMUAllSuiteDeadlineUsesNativeOwnership, TestQEMUAllSuiteDeadlineRefusesUnboundedBudgets -->

Both full and selected `test qemu all` runs prepare the guest's conntrack flow
source before starting a suite. The preparation requires UID 0, Ze's runtime
kernel and `/gokrazy/init` as PID 1 before changing kernel state. Ze builds the
required conntrack and ctnetlink support in, so an absent `modprobe` is harmless.
The action enables accounting, reads back `1`, and atomically installs one
accept-only output tracking rule in its reserved `inet le_test_conntrack`
table. It replaces only that table's output-chain rule on repeated calls;
other tables remain untouched. Full chain readback verifies the output hook,
priority, accept policy and single `ct state` counter rule.

The preparation uses the native process owner with a 30-second deadline and
the same fifteen-second TERM-to-KILL grace. Setup or readback failure stops
before a fixture can receive a verdict. The distro test DUT deliberately
does not perform appliance-only kernel setup, so an own-system boot alone
does not make its conntrack recent-flow ring live.
<!-- source: internal/le/test/qemu/alltests_conntrack.go -- allTestsRun.prepareConntrack, guestConntrackSetup -->
<!-- source: internal/le/test/qemu/alltests.go -- allTestsRun.Execute, allTestsRun.executeOne -->
<!-- test: internal/le/test/qemu/alltests_conntrack_test.go -- TestQEMUAllPreparesConntrackBeforeSuites, TestQEMUAllRefusesConntrackPreparationFailure -->

<!-- source: internal/le/test/qemu/alltests.go -- AllTestsRun.Run, the phase population -->
<!-- source: internal/le/test/qemu/guestlabs.go -- the netns suite selection -->

### `test qemu all` is a GUEST action, and four things must be true before it runs

`le test qemu all` is a guest action. A host without the checkout mounted at
`/workspace` refuses immediately. Before kernel preparation, the action also
checks root and actual Ze-system identity; an environment flag cannot make a
host or an Alpine guest pass those checks. Use the own-system command carrier
with `harness`, which prepares the mount, the caches and the root.

Four preconditions, each of which fails with a message that does NOT name the
precondition. Measured 2026-09-04 and 2026-09-05, seven guest boots to establish:

| Precondition | What its absence looks like |
|--------------|-----------------------------|
| `packages iproute2` | `ZE-OBSERVER-FAIL: ... ip: invalid argument 'replace' to 'ip'`. BusyBox `ip` has no `neigh replace`, so an observer that programs a neighbour dies in test setup, before any assertion |
| The three daemon binaries, under canonical names | `qemu: bin/ze-stripped-flow is missing or not executable -- cross-compile it on the host first`. `./le test qemu guest-binaries` builds the whole set on the host into `tmp/qemu/linux-<arch>/bin` through `testfunctional.PrepareGuest`, the one list of what a guest needs, and answers the `ZE_BIN`, `ZE_STRIPPED_BIN` and `ZE_STRIPPED_FLOW_BIN` assignments the guest command passes (`GuestBinaryEnvironment`, `guestbin.go`). `shim()` copies them into the guest-local `/tmp/ze-qemu-bin` as `ze`, `ze-stripped` and `ze-stripped-flow`, copies `ze-fakefilter` from beside `ZE_BIN` (or links its path when it was not built), and links `le` to the running guest le (`guestLeLink`), because the tools dispatch on basename. The daemons are copied, not linked into the 9p share: the guest keeps no page cache for a 9p file nothing maps, so a linked `ze` paged its own text in over the share at every start, about 7300 major faults each, and on a loaded host that outlasted a 20s `.ci` budget before ze wrote anything (`copyIntoShim`, `alltests_shim.go`). Every shim copies again rather than reusing an earlier copy: size and modification time miss a daemon replaced through `cp -p` or `rsync -t`, and a content check would read the whole source over the share, which is what the copy costs. A hand-written build list in the nightly once left `ze-stripped-flow` out, and every run refused here before its first test |
| The three binaries, STATICALLY linked | `qemu: bin/ze: is dynamically linked against /lib64/ld-linux-x86-64.so.2, which the musl guest does not have`, and nothing runs. `runnableInGuest` (`alltests.go`) reads each binary's PT_INTERP header before the first child. Until 2026-09-05 `verify()` only stat'd the file, and the same cause surfaced as 326 identical per-test failures (`start ze: fork/exec ... : no such file or directory`), because the kernel answers ENOENT for the LOADER while naming the BINARY. Build with `CGO_ENABLED=0`, which is what the native toolchain sets (`internal/le/go/toolchain`) |
| `packages libcap` | `qemu: setcap is not on the guest PATH, so the per-test network namespace suites cannot hold their capabilities`. The five routed suites run ze as an ordinary user inside a fresh namespace, and `setcap` is what gives that user CAP_NET_ADMIN |
| The `bgp` verb before the suite | The `ze plugin` help text, and exit 1. `le test plugin <name>` names no suite; the suite form is `le test bgp <suite> <name>`, which is what `vmSuites` passes (`alltests.go`) |

`le test qemu run` installs only `git curl musl-dev` beyond the base image
(`internal/le/test/qemu/run.go`), so anything else a test shells out to has to be
named in `packages`. `iproute2` and `libcap` are both preconditions of a whole
run, and the refusal for each one names it.

`git` is there for the launcher as much as for the tests. A guest command that
starts with `./le` runs the dispatcher over the shared checkout, and the
dispatcher reads HEAD's input key with `git ls-tree` before it picks a build
(`docs/contributing/running-commands.md`, "Launcher builds do not overwrite
running binaries"). The setup command also marks every directory safe
(`git config --global --add safe.directory '*'`), because the checkout the
guest mounts belongs to the host's user. The newest build in `bin/builds/` is
the host's, and its header is not the guest's, so the guest picks or builds its
own `<time>-<sha>-linux-<arch>` build there, from a HEAD export, exactly as
another machine sharing the checkout would. A checkout git cannot read has no
key, and the dispatcher then runs the newest build the guest can start without
building one. A command naming `tmp/qemu/linux-<arch>/le` directly runs the
guest le `./le test qemu run` built on the host and bypasses the dispatcher.
<!-- source: internal/le/test/qemu/run.go -- setupCommand -->
<!-- source: le -- le_input_key, choose_build, le_runs_here -->

`coreutils` used to be a fifth precondition. The external timeout wrapper first
used a GNU long option, then `-k 15`; minimal BusyBox versions reject one or
both before executing any suite. The native deadline owner now supplies the
cap and process-group cleanup directly, so neither spelling nor an external
`timeout` executable is a prerequisite.

### Running ONE `.ci` test in a Ze guest

From the host, one command runs it:

```bash
./le test qemu ci test test/plugin/<test-name>.ci
```

That is the command a `needs-linux` skip reason ends with, with the skipped
test's own path filled in (`runner.GuestRoute`). It refuses a path no guest
suite walks, or a file the checkout does not hold, before it builds anything.
Then it cross-builds the daemons as `guest-binaries` does, resolves the runtime
kernel Ze's image boots (`GuestRuntimeKernel`), and boots the own-system
carrier with `harness` and the guest command below (`runOneCIHere`,
`oneCIArguments`). The runner cannot import le, so it spells the command as a
literal, and `TestRunnerGuestRouteIsTheOneCIAction` parses the line it prints
through this action's own parser.
<!-- source: internal/le/test/qemu/onetest.go -- runOneCIHere, oneCIAdmits, oneCIArguments -->
<!-- source: internal/test/runner/guest_route.go -- GuestRoute, guestRouteAdvice -->

By hand, the same run is the
[own-system command carrier](#one-command-in-zes-own-system) with Ze's
matching runtime kernel and `harness`. Cross-build the daemons first and pass
`ZE_BIN`, `ZE_STRIPPED_BIN` and `ZE_STRIPPED_FLOW_BIN` for the selected build,
as `./le test qemu guest-binaries` answers them:

```bash
./le test qemu guest-binaries
./le test qemu docker-lab kernel "<runtime-tree>/vmlinuz" harness timeout 1800s \
  command 'ZE_BIN=tmp/qemu/linux-<arch>/bin/ze ZE_STRIPPED_BIN=tmp/qemu/linux-<arch>/bin/ze-stripped ZE_STRIPPED_FLOW_BIN=tmp/qemu/linux-<arch>/bin/ze-stripped-flow tmp/qemu/linux-<arch>/le test qemu all test test/plugin/<test-name>.ci'
```
<!-- source: internal/le/test/qemu/onetest.go -- OneCICommand -->
<!-- source: internal/le/feature/recordrun.go -- repoRunner.runInGuest -->
<!-- source: internal/le/rfc/discriminate_guest.go -- observationRunner.guestCIArgv -->

The selected entry point creates the binary shim, prepares conntrack and any
required network namespace, then runs that CI alone. A direct `test bgp plugin`
command with a manual shim would bypass the kernel preparation.

Add `save <directory>` to the guest's `test qemu all test <path>` command to
retain the runner's complete stdout and stderr artifacts through its existing
`--save` option. Use a directory beneath the shared checkout when the files
must survive guest shutdown. This option accepts one ordinary selected CI;
whole-suite and stress requests are refused so repeated runs cannot overwrite
the same capture. Without `save`, output handling is unchanged.
<!-- source: internal/le/test/qemu/actions.go -- runAllTestsHere -->
<!-- source: internal/le/test/qemu/alltests.go -- verify, suiteCommand -->
<!-- source: internal/test/runner/runner_output.go -- saveTestOutput -->

If private namespace preparation fails, the selected run refuses before any
test starts. Its operator error names the selected CI, retains the preparation
failure as evidence and gives `./le test qemu ci test <path>` to start that
test in a fresh guest. Successful preparation continues with the selected
suite.
<!-- source: internal/le/test/qemu/alltests.go -- executeOne -->

Inside the same own-system guest, repeat one selected live or draft CI under
load by adding `stress <iterations>` and optionally `stress-parallel <workers>`:

```text
<GuestBinaryEnvironment> tmp/qemu/linux-<arch>/le test qemu all test test/draft/plugin/<test-name>.ci stress 80 stress-parallel 4
```

Iterations must be 1..1000 and workers 1..4; the default is one worker.
Both keywords require `test`, and `stress-parallel` also requires `stress`.
Invalid counts and paths refuse before the shim or kernel preparation. The
selected CI's parsed `netns-link` declaration still chooses its private network
namespace. Namespace and conntrack preparation happen once, followed by the
existing stress-repro orchestrator using the canonical suite words and stable
test stem. Namespace children use the prepared capability copies of both `le`
and `ze`; they retain the suite's environment, PATH and guest job parent, and
never rebuild a pinned daemon.

The invocation timeout remains `ZE_QEMU_SUITE_TIMEOUT`. The stress budget is
`ceil(iterations / workers) * (invocation timeout + 15s) + 1m`, with checked
duration arithmetic and another fifteen seconds on the enclosing stress
context. The ordinary 900-second suite cap therefore does not cap the whole
stress sequence. Set the host `docker-lab ... timeout` above the sum of these
bounds for every selected command in that guest, allowing boot and preparation
time too. A zero-minute stress-repro budget expires immediately and proves no
repetitions.

The phase retains a typed stress report, the exact executable and argument
array, namespace and capture path. Success requires the requested and completed
iteration counts to match exactly, one actually executed and passed selected
test in every child, and no setup error, failed child, reproduction, skip or
interruption. A zero-exit child with no summary or an all-skipped summary fails.
The ordinary selected and whole-suite forms keep their existing behavior.
<!-- source: internal/le/test/qemu/alltests_stress.go -- selectedStressOf, selectedStress.budget, allTestsRun.selectedStressPhase, selectedStressCount -->
<!-- source: internal/le/test/stressrepro/selected.go -- RunSelected -->

`test qemu all` supplies `LE_TEST_NO_BUILD=1` and the explicit `ZE_BIN` to every
suite (`environment`, `internal/le/test/qemu/alltests.go`). Without them the runner
builds its own ze from the source tree inside the guest and never looks at the
`ze` on `PATH` (`NewRunner` and `setupBinShims`, `internal/test/runner/runner.go`).
The test then passes or fails on a binary you did not build. A red you forced by
cross-building a broken ze, or by an overlay, never reaches the test, and the
green it reports proves nothing.

Keep the host session identity on the host invocation. Do not export
`CLAUDE_CODE_SESSION_ID` or `ze.session.id` into the guest command. Without a
session identity, the functional runner creates fresh work directories in the
guest's `TMPDIR`, which `harness` sets to the guest-local `/tmp`.

Exporting the host identity selects
the host session directory through `/workspace` instead. Files created through
that share can have the host UID, so the guest-root daemon correctly refuses
their stores. The host admission slot still reaches the guest through
`ZE_RUN_JOB`. Test ownership does not require a shared store directory.
<!-- source: internal/test/sessionpath/sessionpath.go -- ID, Root, DefaultScratchRoot -->
<!-- source: internal/test/runner/runner_exec.go -- runTest -->
<!-- source: internal/le/test/qemu/run.go -- setupCommand -->

The own-system command carrier supplies the guest's LABEL-only `ZE_RUN_JOB`
stand-in; retain that parent rather than passing the host's PID-bearing registry
entry. Its `harness` branch and the legacy Alpine `./le test qemu run` route each
hold a host admission slot themselves, under the label `qemu-run`, and re-execute
the typed words as a child in it (`admitQEMURun`). A plain `docker-lab` command
holds none, so a command that starts le goes through `harness`. The guest setup writes a stand-in entry in the guest's own
`/tmp` and exports it as `ZE_RUN_JOB`, so the guest runner runs inside that slot
instead of touching the host's PID registry: neither machine can judge whether
a process on the other is alive. A guest le that admitted itself would also
fingerprint the whole checkout over 9p, where `git diff HEAD` reads every
tracked file because the index's stat data matches none of them; on 2026-10-27
that ran past the 60-second git limit and killed `le test firewall` before its
first test. The run never attaches to another run's verdict: its kernel image
and cross-built binaries are untracked, so equal words over an equal tree can
still judge different code.
<!-- source: internal/le/test/qemu/dockerlab_command.go -- Run.dockerLabCommand -->
<!-- source: internal/le/test/qemu/guestle.go -- guestRunParentSetup -->

The host invocation establishes one signal root before admission or the guest-le
build. SIGINT, SIGTERM and SIGHUP cancel its own admission wait, active build or
VM lifecycle. A claimed slot's reexecuted child runs through
`Admission.RunClaimedContext`: the host joins it before releasing that ticket,
and retains the originating signal's exit status. An inherited parent slot
remains owned by its parent; canceling a waiter never releases another holder.
The guest-le compiler explicitly runs in an owned noninteractive process group
with no terminal input. Cancellation sends its leader TERM and grants the
runner's existing ten-second grace; group cleanup also stops compiler tools
before the Go leader is reaped. Admission joins that cleanup before releasing
its ticket. This compiler-only policy does not change generic job stdin,
terminal-attached commands, or the QEMU action's self-reexecution.
This is a cancellation contract for the legacy Alpine route, not evidence that
Ze's own system booted or that a guest test executed.
<!-- source: internal/le/test/qemu/actions.go -- runQEMUHere, runQEMUInSlot -->
<!-- source: internal/le/job/job.go -- Admission.RunClaimedContext -->
<!-- source: internal/le/job/nonterminal.go -- Admission.streamNonterminal -->
<!-- source: internal/le/test/qemu/guestle.go -- buildGuestLe -->

<!-- source: internal/le/test/qemu/run_admit.go -- admitQEMURun, runArgv -->
<!-- source: internal/le/test/qemu/guestle.go -- guestRunParentSetup, guestRunJobParent -->

<!-- source: internal/le/test/qemu/actions.go -- test qemu all is registered as a guest action -->
<!-- source: internal/le/test/qemu/alltests.go -- suiteCommand, the ZE_*_BIN knobs -->
<!-- source: internal/le/test/qemu/alltests_shim.go -- shim, copyIntoShim -->
<!-- source: internal/le/test/qemu/run.go -- runBootstrapCommand and the package list -->
<!-- source: internal/le/test/qemu/guestle.go -- buildGuestLe, guestJobParent -->
<!-- source: internal/le/test/qemu/run_iso.go -- durableCacheDir, ensureISO -->

### Running the BGP stress harness in the guest

`./le test integration stress` needs root and network namespaces, which macOS
has neither of. BGP performance is measured on the owner's Mac because its
hardware is known, so the harness runs there in the guest:

```bash
./le test qemu stress scenario 05-profile-1m pprof output tmp/perf-ac1
```

| Keyword | Meaning |
|---------|---------|
| `scenario <name>` | Required. One scenario of the harness registry, refused on the host when unknown. The help placeholder is the registry itself |
| `output <directory>` | Required. A host directory that is absent or empty, so one directory never mixes two runs |
| `prefixes <count>` | A smoke run: every Ze round sends this many prefixes. Never a measurement |
| `pprof` | Capture the CPU, heap and goroutine profiles (`ZE_PPROF=1` in the guest) |
| `timeout <duration>` | The guest command's bound, `1h` by default: a cold DUT build in the guest, then every round |

The host cross-builds the guest `le`, boots the guest with `packages "iproute2
ethtool"`, and runs `STRESS_SCENARIO=<name> <guest le> test integration stress
'|' json` from `/workspace` as root. The harness builds its DUT inside the
guest from the shared checkout, so the profile measures that tree. It starts
the DUT on a copy of the scenario config in `/tmp/ze-stress-config-<suffix>`,
never in the checkout: Ze opens its config store beside its config and refuses
a store another user owns, and over 9p the checkout carries the host user's uid
while the DUT runs as root. The harness removes the copy when the run ends. The
guest writes the report to `tmp/qemu/stress-report.json`, a file the host removes
before the boot. The host then writes `report.json` into the output directory
and copies every file the report names: each profile under `profiles`, and the
DUT under `binary`. The paths come from the report, and a path outside
`/workspace` is refused, because it died with the guest.

A stress run demands a hypervisor (`RunOptions.HardwareOnly`): HVF on macOS,
a read-write `/dev/kvm` on Linux. `le test qemu run` lets QEMU fall back to TCG,
which is right for a functional test and wrong for a measurement, so this route
names the accelerator in `-machine` and refuses a host that has none. The run
report's `plan.qemu-argv` records it, with the guest's memory and CPU count:
8 CPUs and 16384 MiB unless `ZE_QEMU_CPUS` or `ZE_QEMU_MEMORY` says otherwise.
Keep both fixed between the runs a comparison reads.

A smoke run on a Linux host proves the route reaches the harness and the
harness reaches the paths it profiles. Its numbers are never a measurement: the
Linux development machine is itself a VM whose performance varies.

**macOS status: written, not yet run on a Mac.** The runner's darwin branch
selects `qemu-system-aarch64`, `-machine virt,highmem=on,accel=hvf`, `-cpu max`
and Homebrew's `share/qemu/edk2-aarch64-code.fd`, and the guest is arm64 on
Apple silicon (`GuestArch`). Unit tests cover those choices; nothing has booted
them on a Mac. On the first run, check in this order:

| Check | How | What failure looks like |
|-------|-----|-------------------------|
| QEMU and firmware are installed | `brew install qemu`; `ls "$(brew --prefix)/share/qemu/edk2-aarch64-code.fd"` | `missing required command qemu-system-aarch64`, or `cannot find aarch64 UEFI firmware` |
| QEMU carries 9p, which shares the checkout | `qemu-system-aarch64 -device help` lists `virtio-9p-pci` | the guest never mounts `/workspace` and the setup command fails at `mount -t 9p` |
| HVF accepts the machine | the run reaches `Booting Alpine VM`, then SSH | QEMU exits at once on a `highmem` or IPA-size error; report it with the serial output |
| The guest fits in memory | `ZE_QEMU_MEMORY=8192` on a 16 GB Mac | QEMU cannot allocate guest RAM |
| The guest `le` cross-builds for linux/arm64 | the `qemu-build-le` job line succeeds | a build error before the boot |
| The report and profiles come back | `ls <output>`: `report.json`, `stress-profile-*.pb.gz`, `ze` | `evidence:` in the answer names the missing file |

`go tool pprof` reads a Linux profile on macOS; pass the copied `ze` as the
binary for source listings.
<!-- source: internal/le/test/qemu/stress.go -- stressAction, runStressGuest, copyStressEvidence -->
<!-- source: internal/le/test/integration/stress.go -- stressConfigDir, stageConfig -->
<!-- source: internal/le/test/qemu/run.go -- RunOptions.HardwareOnly, acceleration, qemuArgs -->
<!-- source: internal/le/test/qemu/hugepages.go -- accelerator, Hardware -->

### Six suites do not run in the guest root namespace

`test qemu all` runs inside an SSH session, and that session's transport lives in
the guest ROOT network namespace. A suite that programs the firewall, the
routing policy or an interface THERE reaches its own transport.

It did. On 2026-09-05 `test/firewall/firewall-nat-exclude.ci` installed a nat
prerouting chain in the guest root namespace, the run died inside
`functional/firewall` with ssh's own exit 255, and the twelve suites, the unit
pass, the installer phase and the integration phase still to come never ran
(`plan/journal/gate-excludes-part-of-its-population.md`).

The `ipsec` suite did the same through the SPD. A daemon on the real XFRM
backend whose `vpn ipsec unmatched` is unset installs the RFC 4301 Section
4.4.1 catch-all DISCARD for in, out and fwd, any address to any address
(`installUnmatched`, `internal/component/ike/engine/unmatched.go`). Only IKE's
own ports, the tunnels and the operator's bypass entries stay open, so in the
guest root namespace the run's SSH session went unanswered, ssh gave up after
its keepalives (`ServerAliveInterval=30`) and exited 255. Each namespace holds
its own XFRM tables, so a test also sees only its own states and policies.
A guest command that runs `le test ipsec` itself, as
`./le test qemu run ... command './le test ipsec --draft -a'` does, gets no
namespace and cuts the session the same way: run one test through
`le test qemu all test test/<dir>/<name>.ci`, or a draft through
`le test qemu all test test/draft/<dir>/<name>.ci`, which takes the suite's namespace.
<!-- source: internal/component/ike/engine/unmatched.go -- installUnmatched, unmatchedPolicies -->

Every row of `vmSuites` therefore states its namespace, and a row that states
none is refused before the run starts.

| Namespace | Suites | What the child gets |
|-----------|--------|---------------------|
| `guest-root` | every other suite | the guest's own namespace, and ze as root |
| `per-test` | `firewall`, `policy`, `ospf`, `ospfv3`, `pppoe`, `ipsec` | a fresh namespace for each test, entered by the `.ci` runner before it spawns anything, and ze as uid 1000 holding file capabilities |

The runner enters the namespace itself, once per test (`enterTestNetns`,
`internal/test/runner/netns_linux.go`), and only when `ZE_TEST_NETNS` is set
with a non-root `ZE_TEST_UID`. `test qemu all` sets both for a routed suite, copies
`ze`, `ze-stripped` and the running `le` to the guest's own tmpfs with file
capabilities (a 9p mount carries no extended attribute, so a `setcap` on
`/workspace` would buy nothing). The two daemon copies receive the set the systemd unit
grants a non-root daemon: `netnsDaemonCapBits` in `internal/test/runner/caps.go`
derives them from `privilege.DaemonCapabilities`, and they are written
through `runner.NetnsDaemonSetcap` as
`cap_net_admin,cap_net_raw,cap_net_bind_service,cap_bpf+ep`, and the `.ci`
runner's `caps=` gate reads the same list in this mode, so a test that needs a
capability the dropped daemon will not hold is skipped with its name rather
than admitted by the root runner's own probe. The fixture copy of `le` receives
that same set plus `CAP_KILL` and `CAP_SYS_PTRACE` through
`runner.NetnsFixtureSetcap`. It can stop an owned privileged metadata helper
and inspect a nondumpable daemon's executable directly. The namespace role is
numeric UID1000 even when the host owner's cidata UID is different, so direct
inspection cannot depend on UID1000 having a passwd or sudo account. These
test-controller authorities do not reach either daemon copy or the `caps=` gate.
The preparation also gives the
dropped user a state directory to
write in. The `le` copy is what the dropped daemon finds when a plugin block runs
`le test fixture ...` by name: the runner puts that tmpfs on the daemon's PATH as
`ZE_BIN`'s directory, and the only other `le` there is the shim link into the
checkout, whose root keeps the host owner's 0700 mode over 9p. Without the copy,
`pppoe-client-lcp-renegotiation` on Ze's own image logged
`/bin/sh: le: Permission denied`, its fixture never started, and the test timed
out (`netnsHarnessSource`). That tmpfs is mounted on
`/tmp/zebin` itself (`netnsCapMountScript`), because the kernel ignores a file
capability on a `nosuid` mount and gokrazy, the init of Ze's own image, mounts
`/tmp` with `nosuid`: there the copies ran as uid 1000 with no capability, and
an ipsec daemon's XFRM install answered `operation not permitted`. The
preparation refuses a `nosuid` mount it finds on `/tmp/zebin`.

Both `test qemu all` and `test qemu netns` also make `/dev/ppp` and
`/dev/net/tun` mode 0666 (`netnsDevNodes`, `netnsDevNodesScript`). devtmpfs
creates each 0600 root, because neither driver declares a mode, and the
dropped user's capabilities carry no `cap_dac_override`. Without the chmod its
PPPoE client could not open `/dev/ppp` after PADS: it sent PADT and redialed,
and on Ze's own image `pppoe-client-lcp-renegotiation` timed out in its
concentrator's LCP read. Every IPsec Child SA that receives UDP-encapsulated
ESP opens `/dev/net/tun` to create the `ze-espform` device (`openESPFormTUN`),
and the install is refused when that open fails. The chmod runs only where
`/workspace` is the 9p share, so it never widens a device on a machine that
outlives the run.
<!-- source: internal/le/test/qemu/netns.go -- netnsCapMountScript -->
<!-- source: internal/le/test/qemu/netns.go -- netnsDevNodes, netnsDevNodesScript -->
<!-- source: internal/le/test/qemu/netns.go -- netnsHarnessSource -->
<!-- source: internal/test/runner/caps.go -- netnsDaemonCapBits, NetnsDaemonSetcap -->
<!-- source: internal/test/runner/caps_fixture.go -- FixtureProcessControlCapability, NetnsFixtureSetcap -->
<!-- source: internal/test/runner/caps_fixture_inspection.go -- FixtureProcessInspectionCapability -->
<!-- test: internal/le/test/qemu/netns_fixture_capabilities_test.go -- TestNamespacePreparationKeepsFixtureProcessControlOffDaemons -->
<!-- test: internal/le/test/qemu/netns_fixture_inspection_test.go -- TestNamespacePreparationMakesInspectionIndependentOfAccounts -->
<!-- source: internal/test/runner/record_parse_option.go -- applyNetnsDaemonCapsGate -->
<!-- source: internal/core/privilege/capabilities.go -- DaemonCapabilities -->
<!-- test: internal/le/test/qemu/netns_alltests_test.go -- TestTheCapabilityCopiesAreMadeBeforeTheFirstSuite, TestTheCapabilityCopiesLandOnATmpfsThatHonorsFileCapabilities, TestTheCapabilityCopiesCarryTheHarnessTheDroppedDaemonExecs, TestThePreparationLetsTheDroppedDaemonOpenDevPPP -->
<!-- source: internal/le/test/qemu/netns_linux.go -- prepareNetnsBinaries -->
<!-- source: internal/le/test/qemu/netns_linux.go -- prepareNetnsState -->
<!-- source: internal/le/test/qemu/netns.go -- allTestsRun.prepareNamespace -->

Routing a suite also un-skips its `option=netns-link` tests. Outside this mode
the runner SKIPS them (`applyNetnsLinkGate`, `internal/test/runner/caps.go`),
so eight `test/ospf` and three `test/ospfv3` tests ran in no VM phase at all.
The `plugin` suite stays in the guest root namespace during whole-suite runs.
Its ASPA `netns-link` cases are therefore skipped in that mode.
<!-- source: internal/le/test/qemu/alltests.go -- vmSuites -->

A one-CI selection uses the runner's CI parser. A declared `option=netns-link`
moves only that selected test into a per-test namespace, with its tools checked
and binaries prepared. An option inside an embedded file is not a declaration.
A BPF requirement without a link keeps the guest root namespace.
<!-- source: internal/le/test/qemu/alltests.go -- allTestsRun.executeOne -->
<!-- source: internal/test/runner/record_parse.go -- ParseCIFile -->
<!-- source: internal/le/test/qemu/netns.go -- allTestsRun.verifyNamespaces, allTestsRun.prepareNamespace -->
<!-- test: internal/le/test/qemu/alltests_one_netns_test.go -- TestOnePluginSelectionUsesCanonicalNamespaceMetadata, TestOnePluginSelectionVerifiesItsOwnNamespaceTools -->

The separate `netns suites plugin` selection runs the seven ASPA validation and policy
cases with their declared `eth1` address, `10.0.0.254/24`. Their received
NEXT_HOP, `10.0.0.1`, is then nonlocal, on-link and syntactically valid.
The selection also includes `adj-rib-in-replay-rfc2545-next-hop`. That carrier
owns three distinct namespaces for the speaker, recipient and next-hop entity,
connected by real veth/bridge links; it no longer requests a dummy `netns-link`.
The fixture checks address ownership and attachment before traffic. Both live
forwarding and replay must retain the complete global/link-local pair on that
common subnet. The fixture owns shutdown after both recipient assertions.
It also includes `redistribute-chain-static-ospf`, which declares an `eth1`
address of `10.0.0.1/24` because OSPFv2 opens no interface without an IPv4
address; `test qemu all` skips it in the root namespace like the ASPA cases.
It also includes `ddos-detect-internal-blind`, whose detector must see no
received traffic for one baseline window: only a fresh namespace holding lo
and a dummy `eth1` receives nothing, so it too is skipped in the root
namespace during whole-suite runs.
<!-- source: internal/le/test/qemu/netns_linux.go -- netnsSelections -->
The selector does not move the whole plugin suite into namespaces.

`./le test qemu netns suites <names>` is the same launcher over a named subset,
and it also asserts the guest root nft ruleset is unchanged by the run. It is
the tight loop; `test qemu all` retains the whole-suite namespace table above.
It copies `ze`, `ze-stripped` and itself, as `le`, into its guest-local
binary directory (`netnsCapDir`) with the daemon and fixture grants described
above, and runs the
suite through that copy. The daemon runs as uid 1000, and a `.ci` whose plugin
block runs `le test fixture ...` makes it exec `le` by name: a link into the
checkout failed with `le: Permission denied`, because the 9p root keeps the
host owner's 0700 mode, and such a fixture also needs the capabilities for its
AF_PACKET socket, which a dropped user receives only from the file.
<!-- source: internal/le/test/qemu/netns_linux.go -- prepareNetnsBinaries, runNetnsSuite -->
<!-- source: internal/le/test/qemu/netns.go -- netnsHarnessSource -->

The plugin subset is scheduled in `qemu-nightly.yml`'s
`runtime-kernel-labs` job, alongside the PPPoE proof. Run the same action inside
the runtime-kernel guest:

```bash
./le test qemu docker-lab kernel tmp/kernel/build/vmlinuz harness \
  timeout 1200s command 'ZE_QEMU_BIN=bin/ze-linux-amd64 ZE_QEMU_STRIPPED_BIN=bin/ze-stripped-linux-amd64 tmp/qemu/linux-amd64/le test qemu netns suites plugin'
```

This uses the architecture-qualified guest binaries selected by the existing
`ZE_QEMU_BIN` and `ZE_QEMU_STRIPPED_BIN` overrides or their defaults. The
harness is the guest le that runs the action: `docker-lab harness` builds it
with `internal/le/linuxle` for `QEMU_GOARCH` or the host architecture at
`tmp/qemu/linux-<guest arch>/le` (`buildGuestLe`) before it boots the guest, and
the launcher runs `le test <suite>` through a link to it (`guestLeLink`). No
variable names the harness. A native prerequisite skip is not evidence for this subset.

<!-- source: internal/le/test/qemu/netns.go -- the namespace table's producer and the capability preparation -->
<!-- source: internal/le/test/qemu/alltests.go -- vmSuites, the Namespace of each row -->
<!-- source: internal/le/test/qemu/netns_linux.go -- netnsSelections, runNetnsSuite -->
<!-- source: .github/workflows/qemu-nightly.yml -- runtime-kernel-labs ASPA plugin namespace subset -->

### A run says what it planned, what it reached, and how many tests ran

The report names its whole population before the first child (`plannedPhases`),
so a run that stops can be told from a run that answered. `Unreached` is the
planned phases with no result, the summary names them, and a report carrying
one never prints `ALL PHASES PASSED`.

Each functional suite also carries how many tests it EXECUTED, read from the
runner's own summary line (`suiteTally`, `alltests_tally.go`). A suite that
prints no such line executed nothing and is a failure whatever it exited with:
the `.ci` runner answers 0 for an empty selection, so a suite whose directory
moved would otherwise report success.

A process whose transport is cut reports nothing at all, which is why the
population is printed at the START of the run as well.

<!-- source: internal/le/test/qemu/alltests_report.go -- Planned, Unreached, Text -->

### How `option=needs-linux` behaves on each host

| Host | Behavior |
|------|----------|
| `GOOS != linux` | The runner sets `SkipReason` and the test reports SKIP, never FAIL. `./le verify worktree` and `./le test functional gating` stay green on darwin without running the test |
| `GOOS == linux`, a `caps=` capability the process does not hold | The same: SKIP, with the missing capability in the reason |
| `GOOS == linux`, inside the VM | The option is inert, so the same `.ci` test runs for real against the Linux kernel |

Each skip reason ends with the host command that runs that test in the guest,
`./le test qemu ci test <path>`, never `test qemu all`, which is a guest action
and refuses on the host that printed the reason. A declaration with no `.ci`
behind it (an `.et` or `.wb` admission), or a `.ci` outside the checkout's
`test/` tree, gets the command with a `test/<dir>/<name>.ci` placeholder.

<!-- source: internal/test/runner/record_parse_option.go -- parseOptionNeedsLinux -->
<!-- source: internal/test/runner/guest_route.go -- guestRouteAdvice -->

## A run boots ze's kernel when it is given one

The VM runs Alpine userland, and it runs **ze's own runtime kernel when the
`kernel <path>` parameter names one**. Without that parameter `Run.kernelPath`
answers the empty string, QEMU boots the Alpine ISO's own kernel, and
`Run.assertRuntimeKernel` never runs: it is reached only when `Plan.Kernel` is
set. So a run with no `kernel` argument proves nothing about the kernel an
operator gets, and every recipe on this page passes one.

The host action `./le test qemu run` owns the Alpine cache, the QEMU lifecycle, both
9p shares, bounded SSH waits, package installation, and cleanup. When the
parameter is there, the guest release check refuses a boot whose `uname -r`
disagrees with `internal/appliance/kernel.version`: it accepts that version, a
longer version of it, and either with the runtime kernel's `-ze` suffix
(`docs/architecture/appliance/kernel-profiles.md`).

Every required checkout/cache 9p mount must succeed before the setup downloads
or unpacks Go or runs the patched-tree generator. Optional module, conntrack and
Git setup alternatives are grouped inside the shell's AND chain: their OR
branches cannot recover an earlier mount failure. A failed required mount stops
setup, rather than continuing against an empty or dangling workspace.
This shell contract does not attribute the historical cache-symlink xattr
`ELOOP` to BuildKit or the 9p server, and no mount security policy changes on that
unconfirmed cause.
<!-- source: internal/le/test/qemu/run.go -- setupCommand, virtfsArgs, cacheShare -->

With a `kernel`, the command line `Run.qemuArgs` appends names the serial
console of the machine, `ttyS0` on amd64 and `ttyAMA0` on arm64. Alpine's
init spawns the login getty on that device, so a wrong name boots a guest
that never shows `login:`. The run keeps the guest's serial output in
`tmp/qemu/serial-*.log` until the run passes. A boot that shows no login
prompt names that file, and names the boot timeout as the cause unless QEMU
itself exited, when it quotes QEMU's stderr instead (`bootFailure`,
`internal/le/test/qemu/run_exec.go`).

The native host action cross-compiles a Linux `cmd/ze` personality with the
`ze_le` tag before boot. That guest binary runs the selected action. The full
Linux suite is:

```text
./le test qemu guest-binaries
mkdir -p tmp/qemu && cp -RL "$(./le go stdpatch goroot)" tmp/qemu/_guest-go
./le test qemu docker-lab kernel <vmlinuz> harness timeout 5400s \
  command 'PATH=/workspace/tmp/qemu/_guest-go/bin:$PATH GOROOT=/workspace/tmp/qemu/_guest-go GOTOOLCHAIN=local GOCACHE=/workspace/tmp/qemu/go-cache GOMODCACHE=/workspace/tmp/qemu/gomodcache ZE_BIN=tmp/qemu/linux-<arch>/bin/ze ZE_STRIPPED_BIN=tmp/qemu/linux-<arch>/bin/ze-stripped ZE_STRIPPED_FLOW_BIN=tmp/qemu/linux-<arch>/bin/ze-stripped-flow <guest-le-binary> le test qemu all'
```

The whole run compiles Go in the guest (the unit, installer and integration
phases), and Ze's image carries no Go, so the command names the patched tree
copied into the checkout and caches on the share. The tree to copy is the
patched standard library that `./le go stdpatch goroot` prints, the same tree
the nightly selects before it copies `$GOROOT`
(`docs/contributing/patching-the-standard-library.md`); `-L` follows its links
into the real GOROOT, which no share reaches. Why each assignment is there is
the Go paragraph under
[One command in Ze's own system](#one-command-in-zes-own-system).
<!-- source: .github/workflows/qemu-nightly.yml -- the needs-linux job's guest command -->

The extra `le` after the guest binary is intentional. The cross-compiled file
has an architecture-qualified basename, so `cmd/ze` treats it as a Ze
personality; the `ze_le` crossing selects the same `qemu all` action the
standalone root launcher exposes. Guest-side VRRP, PPPoE, network-namespace, and
full-suite work is Go in `internal/le/test/qemu`, not an interpreted guest driver.

A shared checkout hands the guest a symlink as a symlink, so a `tmp/` path that
`./le scratch migrate` relocated out of the tree dangles inside the guest until
its target is mounted there. `tmplink.Shares` walks the two layouts the
migration leaves: `tmp/` itself a symlink, or a real `tmp/` whose migratable
children (the `tmplink.Migratable` allowlist) are each a symlink. Every link found
becomes one 9p share, tagged `zescratch` for `tmp/` or `zescratch-<child>`, that
the guest mounts at the link's own absolute target before it touches
`/workspace/tmp`. `Run.scratchShares` adds them to the `./le test qemu run` argv and
setup line, and `guestSetup` plus `qemuArgs` in the kernel builder do the same
for `ze appliance kernel --builder qemu`, whose worker binary and runtime output
both sit under `/workspace/tmp`.

`./le test qemu run` also exports the checkout's `cache` link, tagged
`zescratch-cache`, when it is a symlink, which every le run on the host makes it
(`ensureCacheLink`, `internal/le/register.go`; `cacheShare` in
`internal/le/test/qemu/run.go`). The guest builds into the host's one Go
cache through it: the setup command and the keep-alive profile export
`GOCACHE=/workspace/cache/go-cache` (`setupCommand`, `keepAliveProfile`), and
every le-driven Go build inside the guest sets `GOCACHE` to `cache/go-cache`
too. The guest keeps no Go cache of its own under `tmp/qemu/`. Without the
share a lab that cross-compiles inside the guest meets the link dangling and
stops with `mkdir /workspace/cache: file exists`; the `le` launcher alone then
falls back to `tmp/go-cache` (`gotoolchain.BootstrapCache`). The kernel builder
runs no le-driven build in its guest and takes no cache share.

A link that dangles on the host itself (its target directory was removed) is
refused before the run creates its cache directories or builds its shares:
`danglingScratchLink` checks `tmp/`, each `tmplink.Migratable` child of a real
`tmp/`, and `cache`, and the refusal names the link, its missing target, and
`mkdir -p '<target>'` as the fix. Neither `./le scratch ensure` nor
`./le scratch migrate` creates a missing child target, so the directory is the
fix.
<!-- source: internal/le/test/qemu/run.go -- setupCommand, ensureScratch -->
<!-- source: internal/le/test/qemu/scratchlink.go -- danglingScratchLink, linkTargetMissing, danglingLinkRefusal -->
<!-- source: internal/le/test/qemu/run_exec.go -- keepAliveProfile -->
<!-- source: internal/le/go/toolchain/gotoolchain.go -- BootstrapCache -->
<!-- source: internal/core/tmplink/tmplink.go -- Shares -->
<!-- source: internal/le/test/qemu/run.go -- cacheShare -->
<!-- source: internal/le/test/qemu/run.go -- Run.scratchShares -->
<!-- source: internal/appliance/kernelbuilder/recipe.go -- guestSetup -->

```text
host                                      QEMU Alpine VM
────                                      ──────────────
./le test qemu run kernel <vmlinuz> ...
  ├─ cross-compile cmd/ze and the native guest runner
  └─ boot the named kernel and run ONE command over SSH
       ├─ boot the named kernel             → verify uname -r
       ├─ mount checkout and tmp target      → /workspace
       ├─ install declared packages
       └─ SSH native guest command           → le test qemu all   (in the guest)
                                                ├─ per-test namespace preparation
                                                ├─ functional suites
                                                ├─ Linux unit pass
                                                ├─ installer initrd tests
                                                └─ integration-tagged tests
```

`test qemu all` is on the GUEST side of that diagram. It is an action of the same
`le test qemu` table, and typing it on the host answers `qemu: the repository is not
mounted: /workspace`.

The runtime kernel itself is built by `ze appliance kernel`, which writes
`tmp/kernel/build/vmlinuz` (`runtimeKernelOutputDir`,
`internal/appliance/cmd_kernel.go`). The build is cache-backed under
`~/.cache/ze` and only runs when the kernel key changes.

<!-- source: internal/le/test/qemu/run.go -- Run, Plan -->
<!-- source: internal/le/test/qemu/actions.go -- Answer -->
<!-- source: internal/le/test/qemu/alltests.go -- AllTestsRun.Run -->

The installer phase runs `go test -tags 'ze_core ze_installer'` over
`./internal/install/...`. No other phase compiles those files. The tag is a
personality, not a feature the manifest declares. The unit pass therefore
excludes every file behind it. On a host that is not Linux,
`./le test unit installer` can only type-check them, so this virtual machine
is where they run.

## Docker labs on the docker-host image

A Docker lab that runs Ze needs a Docker daemon whose kernel carries every
feature Ze enrolls (`docs/architecture/testing/interop.md`, "The Docker host
kernel check"), on Ze's own system (`ai/META-DECISIONS.md`). On macOS the `ze` Colima
profile is that daemon (`docs/architecture/appliance/docker-host.md`). The QEMU
route serves the scheduled nightly, whose hosted runner kernel is too old, and
is the Mac's second route (owner D-6). Both architectures use one action:

```text
./le test qemu docker-lab lab "test integration interop-ipsec" timeout 3600s
```

The guest is Ze's docker-host image for the host's architecture, the same image
the `ze` Colima profile boots (`setupdockerhost.EnsureImage`: built, or taken
from `~/.cache/ze/docker-host/<arch>/<key>/` when the inputs' key holds it,
under the setup lock). No Alpine ISO or initramfs is read, no package is
installed. In the Docker lab branch nothing of le's runs in the guest.
The action demands HVF or KVM, because a lab under TCG measures the emulator.
The Docker lab branch then:

1. cross-builds the linux `ze` the kernel check probes with
   (`tmp/qemu/linux-<arch>/ze`, daemon feature tags, through job admission);
2. creates a per-run directory `tmp/qemu/docker-lab-<n>/` holding an ed25519
   key `ssh-keygen` makes for the run and the cidata volume, and refuses when
   the forwarded socket's path there would be longer than the 103 bytes a unix
   socket path holds;
3. writes the cidata volume: an ISO 9660 image labelled `cidata` holding
   Lima's `lima.env`, `meta-data`, `user-data` and `boot.sh`, the files the
   image's boot reads (`dockerhost.ParseCidata`). The user is `ze` (uid 1001,
   home `/home/ze.linux`), its one key is the run's, and `user-data` mounts the
   checkout over 9p at the checkout's own absolute path, and each directory the
   checkout's `tmp` and `cache` links lead out of it at its own path, so a bind
   mount the host le names (the kernel probe mounts the linux `ze` by its host
   path) resolves in the guest (spec R-7). When a link's target text is not
   that resolved path, because a directory on the way is itself a symlink
   (`cache` names `~/.cache/ze/` and `~/.cache` links elsewhere), the directory
   is exported once more under its own tag and mounted at the path the link
   names, so the link resolves in the guest too: without it the harness
   setup's `mkdir` under `/workspace/cache` failed before the command ran
   (`dockerLabShares`). The image boots a read-only root,
   so the guest can create a mount point only strictly under `/mnt`, `/Users`
   or `/home` (`dockerhost.ShareMountRoots`). A share outside them is refused
   before the VM boots, with the directories named and the fix (move the
   checkout or the link's target under one of them) (`refuseUnmountableShare`).
   Covering the share's top directory with a tmpfs was rejected: over `/opt`
   or `/var` it would hide what the image carries there. `boot.sh` starts `containerd.service`
   and `docker.service` through the systemctl shim, what Colima's provisioning
   does under Lima. le writes the ISO itself (`cidataISO`): Lima's writer,
   `go-diskfs`, is not vendored, and a host tool (`hdiutil`, `xorriso`) would be
   one more dependency per OS (spec D-10);
4. boots QEMU with the image as a virtio disk opened `snapshot=on`, so the
   cached image is never written and every run starts from the built state,
   then the cidata volume as the last virtio disk; arm64 boots the edk2 UEFI
   firmware, amd64 SeaBIOS. The guest's console (`hvc0`) is written to
   a uniquely reserved `tmp/qemu/docker-lab-console-<random>.log`, so a later
   run reusing the SSH port cannot truncate an earlier console; its first program line is
   `ze-docker-host: kernel ` and `/proc/version`;
5. waits for the guest's sshd as `ze` on the forwarded port, up to the boot
   timeout, then at most 120 seconds for `docker info` in the guest. When
   the guest never answers, the error carries what QEMU wrote to
   `qemu-errors.log` (a forwarding rule it could not bind, for one) and names
   the console, because the run directory holding that log is removed when the
   run returns (`dockerLabBootFailure`);
6. forwards the guest's `/var/run/docker.sock` with `ssh -N -L` to
   `tmp/qemu/docker-lab-<n>/docker.sock`, created mode 0600
   (`StreamLocalBindMask=0177`; any other mode refuses the run);
7. runs, on the host, `./le setup docker kernel check ze <linux ze>` with
   `DOCKER_HOST` naming that socket, and, when it passes and a `lab` is named,
   `./le <lab>` under the same `DOCKER_HOST` and under `env` with the lab's
   assignments;
8. after the lab, lists over ssh what the guest dockerd still holds of it:
   containers and networks carrying the `ze.lab` label, and the run-owned
   `ze-interop-retained:` image tags (the DUT image kept under its cache tag is
   allowed to stay). The line reads `docker-lab: left behind: none`, or the
   list, or `unknown` when dockerd does not answer the listing;
9. stops the ssh forward, then the VM, and removes the per-run directory.

The exit status is 1 for an interrupted run, else the check's when it failed,
else the lab's.

The action takes its signal root before its first child, so a SIGINT, SIGTERM
or SIGHUP to le ends whichever phase it lands in: the image build, the linux
`ze` build (through `Admission.RunContext`, which ends the wait for a slot and
sends a running build SIGTERM; a build the cancel stopped says so and names no
exit code), the boot, or the host le. QEMU, the ssh clients and the host le
each lead a process group of their own, so a Ctrl-C at the terminal reaches
this le alone. On the cancel, le sends the host le's group one SIGINT, the
interrupt a terminal would have sent; the lab runs its own teardown under its
own signal root and exits. le waits for it up to five minutes, then kills its
group. Only then does le list the leftovers, and only after that does the VM
stop (spec AC-10).
<!-- source: internal/le/test/qemu/dockerlab.go -- runDockerLabHere, runDockerLab, Run.dockerLab, Run.dockerLabPrepare, dockerLabVM.argv, dockerLabGuest.cidataFiles, Run.runHostLe, dockerLabSession.forward, dockerLabLeftoverListing, BuildLinuxZe, refuseUnmountableShare, Run.dockerLabShares -->
<!-- source: internal/le/test/qemu/cidata.go -- cidataISO -->
<!-- source: internal/le/setup/docker/host/actions.go -- EnsureImage -->
<!-- source: internal/le/job/job.go -- Admission.RunContext -->
<!-- test: internal/le/job/runcontext_test.go TestRunContextEndsTheWaitForASlot, TestRunContextStartsNoChildUnderADoneRoot, TestRunContextStopsARunningChildWithSIGTERM, TestRunContextEndsTheFollowOfASharedRun, TestRunContextNotesAStopRatherThanItsError -->
<!-- test: internal/le/test/qemu/dockerlab_root_test.go TestDockerLabBuildsUnderTheVerbRoot, TestDockerLabBuildsSayTheyWereStopped -->
<!-- test: internal/le/test/qemu/dockerlab_leftover_test.go TestDockerLabReportsWhatTheGuestStillHolds -->
<!-- test: internal/le/test/qemu/dockerlab_test.go TestDockerLabBootsTheDockerHostImage, TestDockerLabCidataPassesTheGuestParser, TestDockerLabHostLeTargetsTheForwardedSocket, TestDockerLabForwardsOneInterruptThenStopsTheVM -->
<!-- test: internal/le/test/qemu/dockerlab_share_test.go TestDockerLabMountsTheCacheWhereItsLinkPoints -->
<!-- test: internal/le/test/qemu/cidata_test.go TestDockerLabWritesCidata, TestCidataRefusesANameItCannotRecord -->

A suite's one-scenario selector travels as `env`, a list of `NAME=value`
assignments the lab alone runs under (the check does not see them). `env`
without a `lab` is refused:

```text
./le test qemu docker-lab lab "test integration interop-ipsec" env "IPSEC_INTEROP_SCENARIO=mobike-initiator"
```

With no `lab` the run stops after the check, which is what
`TestDockerLabGuestBootsZeKernel` (build tag `integration`, host only) runs to
prove the image boots, its Docker answers over the forwarded socket and the
kernel passes.
<!-- source: internal/le/test/qemu/dockerlab.go -- dockerLabEnvironment -->

### One command in Ze's own system

**Validation status, 2026-11-05:** a native run of
`TestDockerLabCommandRunsInZeSystem` booted Ze's image with runtime kernel
7.3.0-ze, checked PID 1 and the squashfs root, and read and wrote exact private
scratch bytes as the checkout owner's ordinary UID. This proves the command
carrier. It does not establish the fixtures' runtime results or an RFC proof.

```text
./le test qemu docker-lab kernel "<runtime-tree>/vmlinuz" command "uname -r" timeout 120s
```

`command` and `lab` are mutually exclusive. A command requires an explicit
kernel matching the immutable runtime kernel tree resolved for the image:
arbitrary custom kernels and unknown module sets are refused. le compares the
requested bytes with that tree and with the image's boot kernel. Before starting
QEMU, it streams the supplied payload into a private run-owned file and verifies
the copied bytes against that same SHA256. A changed payload refuses before
boot; the cache never substitutes for the requested input. QEMU's `-kernel`
names this private payload, held until the VM stops, so rewriting the shared
requested path cannot change what QEMU consumes. The image's own FAT boot command line
supplies its root PARTUUID; le does not guess a `/dev/vda2` root or silently boot
the embedded kernel instead. The report records requested and selected kernel
paths (original request and actual private payload), the SHA256 identity and the
module release. Before execution, le checks
the release, `/gokrazy/init` as PID 1, a squashfs root, runtime modules, no
Docker socket, and a cgroup2 hierarchy mounted. gokrazy's init mounts cgroup2 at
`/sys/fs/cgroup` during boot and only logs a failure, so the check makes its
absence a named refusal rather than a red in a command that attaches a cgroup
program. A refusal names the failed check and includes its output.
The PID 1 executable probe uses `sudo -n readlink /proc/1/exe`: PID 1 belongs
to root, so procfs's ptrace credential check refuses its executable link to
the ordinary cidata user. SSH and the requested command still run as `ze`;
only that identity primitive uses sudo, and its exact expected value remains
`/gokrazy/init`.
<!-- source: internal/le/test/qemu/dockerlab_command.go -- Run.dockerLabCommand -->
<!-- test: internal/le/test/qemu/dockerlab_identity_test.go -- TestDockerLabCommandNamesTheFailedIdentity -->
<!-- test: internal/le/test/qemu/dockerlab_identity_test.go -- TestDockerLabCommandReadsRootPID1ThroughSudo -->

The command branch permits TCG when hardware acceleration is unavailable.
Use it for correctness tests only. Docker labs still require HVF or KVM.
<!-- source: internal/le/test/qemu/dockerlab.go -- runDockerLab -->
<!-- source: internal/le/test/qemu/run.go -- Run.acceleration -->

This branch reuses the image cache, per-run SSH key, snapshot disk and absolute
checkout, tmp and cache mounts. The cidata user `ze` has the host owner's UID,
so mode-0600 scratch can be read and written without sudo or a permissions
change. Normal Docker labs keep UID1001. Command boot starts neither containerd
nor dockerd, forwards no Docker socket, performs no Docker kernel check and
does not build a Linux `ze` just for such a check. No package manager or guest
Go build is provided: host-built binaries can be copied to the guest user's
writable home and executed there.

The guest command receives the existing LABEL-only job-parent stand-in, not the
host registry's PID-bearing entry. Its process group receives SIGINT on timeout
or host interruption; le waits for the command's own teardown, then joins its
SSH session before stopping the VM. The command's exit status is the action's
verdict. A successful boot alone is not a successful command or an RFC proof.
<!-- source: internal/le/test/qemu/dockerlab.go -- runDockerLab, dockerLabGuest.cidataFiles, dockerLabVM.argv -->
<!-- source: internal/le/test/qemu/dockerlab_command.go -- Run.dockerLabCommand, dockerLabCommandKernel, dockerLabImageBoot -->
<!-- test: internal/le/test/qemu/dockerlab_command_integration_test.go TestDockerLabCommandRunsInZeSystem -->
<!-- test: internal/le/test/qemu/dockerlab_command_test.go TestDockerLabCommandRefusesIncompatibleModes, TestDockerLabCommandKernelRejectsDifferentBytes, TestDockerLabCommandKernelRefusesUnknownModules, TestDockerLabCommandRefusesMismatchedImage, TestDockerLabCommandUsesImageRootIdentity, TestDockerLabCommandCidataUsesOwnerWithoutDocker, TestDockerLabCommandTransportKeepsUnprivilegedInterrupt -->

The `harness` switch runs a command the way `test qemu all` needs it, and no
caller spells any of it. Before boot le builds the guest le at
`tmp/qemu/linux-<arch>/le`, after the caller's own builds, so a discrimination
overlay never reaches it. In the guest the setup binds the checkout at
`/workspace`, a directory the image carries empty because its root is read-only,
mounts a guest-only tmpfs over `cache/stdpatch`, and enters `/workspace`. The
command then runs through `sudo -n env`, because sudo resets the environment,
with the job stand-in, `HOME` and `TMPDIR` on the guest's `/tmp`, guest-local
`GOCACHE` and `GOMODCACHE`, `GOFLAGS=-buildvcs=false` and `CGO_ENABLED=0`. The
command's own assignments win. The session stays the ordinary `ze` user, and
the forwarded SIGINT goes through `sudo -n kill`: the kernel refuses that user a
signal to a root process, and `kill` still answers success once the shell alone
receives it.

The `root` switch runs the command through the same `sudo -n env` wrapper and
the same `sudo -n kill` interrupt, and does nothing else: no guest le is built,
and nothing is bound at `/workspace`. `harness` already implies it. Its caller
is `le rfc record` for a guest-placed Go unit whose test declares it needs
root (`docs/contributing/rfc-conformance-gates.md`, the guest unit paragraph).
<!-- source: internal/le/test/qemu/run.go -- RunOptions.guestCommandAsRoot -->
<!-- test: internal/le/test/qemu/dockerlab_harness_test.go TestDockerLabRootRunsTheCommandAsRootWithoutTheHarness, TestDockerLabRootIsOnlyValidWithCommand -->

Ze's image carries no Go and no git, and `go test` runs in three phases of a
whole `test qemu all`. A whole run therefore names a Go tree the guest can read
and a cache on the share. The nightly copies the patched tree with `cp -RL` into
`tmp/qemu/_guest-go`, whose `_` keeps it out of the unit phase's `./...`, and
assigns `PATH` with that tree's `bin` first, `GOROOT`, `GOTOOLCHAIN=local`,
`GOCACHE` and `GOMODCACHE` in its command. `PATH` is what makes it work: le and
the phases find `go` through `exec.LookPath`, and `sudo` reset the variable, so the
command's own shell expands the assignment. A selected `.ci` compiles nothing and needs none of this. A test that shells out
to a tool the image lacks (the image ships `ip`, `nft`, `setcap`, `getcap`, sudo,
iptables and busybox) fails there: the fix is that test's dependency or the image,
never a move back to another guest.
<!-- source: internal/le/test/qemu/dockerlab_command.go -- dockerLabHarnessSetup, dockerLabRootCommand, runDockerLabCommand, GuestRuntimeKernel -->
<!-- source: internal/le/test/qemu/dockerlab.go -- runDockerLab -->
<!-- source: internal/le/test/qemu/run_admit.go -- admitQEMURun, dockerLabArgv -->
<!-- source: internal/le/test/qemu/run_guestcommand.go -- guestRootInterruptRemote, Run.interruptGuest -->
<!-- test: internal/le/test/qemu/dockerlab_harness_test.go TestDockerLabHarnessIsOnlyValidWithCommand, TestDockerLabHarnessSetupMountsTheWorkspace, TestDockerLabHarnessRunsTheCommandAsRootWithItsEnvironment, TestDockerLabHarnessInterruptReachesRootCommand, TestDockerLabHarnessActionPreparesTheGuest, TestDockerLabHarnessHoldsAHostSlot, TestDockerLabHarnessBootMountsAndInterruptsAsRoot -->

The image's cache key holds the SHA-256 of the runtime kernel's `vmlinuz`, the
same bytes this branch compares, so the "rebuild the image" refusal has a fix: a
runtime entry rebuilt under its old name holds new bytes and gets a new image.
<!-- source: internal/le/setup/docker/host/build.go -- builder.build -->
<!-- test: internal/le/setup/docker/host/build_kernel_key_test.go TestDockerHostBuildKeysTheImageByKernelBytes -->

### VPN facilities without module loading

`./le test qemu vpn-kernel timeout 30m` uses the native-architecture
docker-host image and the shared hardware-accelerated guest runner. It accepts
no image, kernel or architecture override. Before boot, it checks the emitted
config from the image's runtime-kernel cache resolver against both requirement
manifests.

Guest preparation disables module loading, refuses an already-loaded module,
and removes `/sbin/modprobe` in the disposable snapshot before Docker starts.
It does not modify the cached image or the parent host. The inner
`test deployment vpn-kernel` runs the five ESP combinations against
strongSwan and a two-peer WireGuard handshake/received-traffic proof.
The WireGuard checker also reads the guest host's module state through its
read-only root mount. A scenario or resource-cleanup failure fails the
aggregate verdict.

The fixed preparation block traces its commands and observed values to the
guest console, so a refused assertion identifies the state it rejected.
Both module-state reads must succeed before their values are tested; a failed
read is not an empty module list. Tracing stops before the service-provisioning
body, and the module guards and readiness deadline are unchanged.

This is not proof about the default appliance image, and an amd64/KVM run is
not arm64/HVF evidence. Read those provenance claims from the outer guest
report and the separate appliance-image check.
<!-- source: internal/le/test/qemu/vpnproof.go -- runVPNKernelHere, vpnKernelConfig, vpnGuestPreparation -->
<!-- source: internal/le/test/deployment/vpnproof.go -- runVPNKernelAt -->
<!-- source: internal/le/test/deployment/wireguardproof.go -- runWireGuardProof -->

## Stopping a guest command

A Ctrl-C on a run that has reached its guest command (`run`,
`ipsec-mobike`, the stress harness) stops that command the way a terminal stops a command
on a host: its process group in the guest receives SIGINT, and the command runs
its own teardown. A lab run through `le` in the guest is under `le`'s signal
root there, so it removes its containers, networks and image tags and exits 1,
exactly as on a Linux host.

Three things make that possible:

1. QEMU and every ssh client a run starts, the SSH readiness probe included,
   lead a process group of their own (`Setpgid`). The terminal's Ctrl-C signals
   its foreground group, so it reaches `le` alone, and `le` decides the order:
   the guest first, the VM last. Without this, QEMU took the same SIGINT and
   exited at once, under the guest command.
2. The guest command's shell first traps SIGINT with `trap 'exit $?' INT`, then
   writes its process group, field 5 of `/proc/$$/stat`, to
   `/tmp/ze-guest-command.pgid`. A shell that catches the signal waits for its
   command, then runs the trap, which exits with that command's status: the
   host reads the guest le's exit 1, not the signal, and no later step of the
   line runs after the interrupted one. An EXIT trap the command set still runs.
   Busybox ash 1.36 was checked by hand for all three on 2026-10-21. The setup
   and the command are joined as `setup && { command }`, so an interrupted
   setup runs no step of the command.
3. On the first SIGINT, SIGTERM or SIGHUP, or when the command's `timeout`
   passes, `le` opens a second ssh session that runs `kill -INT -$(cat
   <pgid file>)` (busybox's `kill` refuses `--`). A command that ended in the
   same instant gets no interrupt: its exit status is the answer. The command's
   own ssh session is left open: a guest `le` whose session ends dies of
   SIGPIPE on its next write and never tears down.

`le` then waits for the guest command to end, streaming its output, for at most
two minutes, and stops the VM after. It always answers an error naming what
happened, and an interrupted run always exits non-zero. The report's verdict is
FAIL with the guest's exit code only when the guest command ended; otherwise it
stays INVALID, because no exit code was heard:

| Outcome | What `le` says |
|---------|----------------|
| The guest command ended | it finished its own teardown and exited `<code>`, and the VM was then stopped |
| The command's ssh answered 255 | the guest command's ssh failed (exit 255) after the forwarded SIGINT, so le never heard whether its teardown finished |
| Two minutes passed | it did not finish its teardown, so the VM was stopped under it |
| A second Ctrl-C or SIGTERM | the second interrupt stopped the wait, so the VM was stopped under it |
| The interrupt could not be sent (no group file yet, ssh failed) | le could not forward SIGINT, so the VM was stopped and the teardown never ran |

The second Ctrl-C is the operator's way out of a teardown that hangs. A second
signal that lands in the instant between the root's cancel and the wait's
subscription is taken by the root and changes nothing; the two-minute bound
still ends the wait. When the root ends the run, `le test qemu run` prints
that error before it exits with the signal's code.

### The SSH forward's host port

Every QEMU forward this package starts binds `127.0.0.1`: the SSH forward of
`run`, `docker-lab`, `installer`, hugepages, MPLS boot, the hostname proof and
crash capture, the `installer` and hostname-proof web forwards, and the
crash-capture HTTP forward and monitor.
Every client dials `127.0.0.1` too, and nothing reaches a forward from off the
host. One address is what keeps two VMs off one port. Two forwards on one
address collide at QEMU's bind, which is loud. A wildcard forward beside a
loopback one binds on macOS, because slirp sets `SO_REUSEADDR`, and BSD's
most-specific-bind lookup then sends every loopback ssh to the loopback VM,
so a wildcard `run` would reach a `docker-lab` VM.

A forward with no fixed port (`ssh-port`, `ze.qemu.ssh.port`,
`ze.install.ssh.port`) takes the first port in 2223..2322 that two checks pass.
The band starts above 2222 because 2222 is the SSH port `ze appliance run`
forwards by default and Ze's own CLI SSH default, so a le VM never takes the
port an operator types `ssh -p 2222 localhost` to reach.
The first is the per-port lock the functional runner leases test ports through
(`/tmp/le-port-locks-<euid>`, a fixed path whatever `TMPDIR` says, so root's
locks sit apart in `/tmp/le-port-locks-0`). le takes the lock and holds it
until the process exits, so a concurrent le run skips the port in the window
before this run's QEMU binds it, and so does a second forward of the same run:
the `installer` SSH and web forwards once got the same port. The lock
covers the VM only while its le lives. A `run` or `docker-lab` VM dies with its
le on Linux and is stopped by the next run's reap on macOS (below). The
`installer`, hugepages, crash-capture, MPLS boot and hostname-proof VMs carry neither
mechanism, so a le killed outright leaves them running on their port with no
lock. The second check covers them: it binds the port on `127.0.0.1` and on the
wildcard, IPv4, with `SO_REUSEADDR` cleared, and so finds a port another
program holds on any address, including a VM whose le died and took its lock
with it. With the option set, as Go sets it by default, macOS lets the probe
bind beside a listening holder.

When every port in the band fails a check, the error names the fixed-port
setting of each action that reaches it (`ze.qemu.ssh.port` for `run` and
`docker-lab`, `ze.install.ssh.port` for `installer`, `ze.vpp.hp.ssh.port` for
hugepages, MPLS boot and the hostname proof; crash capture and the hostname
proof's web forward have none), and `lsof +D
/tmp/le-port-locks-<euid>` to name the le processes holding reservations.
<!-- source: internal/le/test/qemu/hostport.go -- forwardHost, reserveForwardPort, reserveForwardPortIn, forwardPortFree -->
<!-- source: internal/test/runner/ports.go -- LockPort -->
<!-- test: internal/le/test/qemu/run_port_test.go TestForwardPortRefusesAHeldForward, TestForwardPortRefusesABoundLoopbackForward, TestForwardPortAnswersAFreePort, TestForwardPortsInOneRunAreDistinct, TestForwardPortSkipsAReservedPort -->

### A VM never outlives its le

Leading its own group takes QEMU out of the terminal's Ctrl-C, so a `le`
killed outright (SIGKILL, a crash) would leave its VM running, holding its
cores and its SSH port. Two mechanisms close that:

| Host | Mechanism |
|------|-----------|
| Linux | QEMU and every ssh client start with `Pdeathsig` SIGKILL (`childProcAttr`), so the kernel kills them when `le` dies |
| Every host, and the only one on macOS | After QEMU starts, the run writes `tmp/qemu/vm-record-<qemu pid>.json`, staged under another name and renamed into place so no reap reads half of it, with the QEMU pid and start time and le's pid and start time, and removes it once the VM is stopped. Before it boots, every run reads the records and stops a QEMU only when the recorded `le` is gone (no process at its pid, or one that started at another time) AND the QEMU pid still names the process that started at the recorded time. A record whose `le` still runs is another session's live VM, and is left alone |
| Any other host (FreeBSD, OpenBSD) | None: `le` has no process start-time reader there (`processStartTime`), so a run stops the QEMU it just started and fails, naming Linux or macOS as the hosts to run on, rather than leave a VM nothing would stop |

The reap prints `stopped QEMU pid <pid>: le pid <pid>, which started it, is
gone`. A record it cannot judge is kept and named. An unreadable record is
kept and named while it is younger than ten minutes, and dropped after. A
closed terminal (SIGHUP) is handled like SIGTERM by every qemu root. After the
hangup the guest's output can no longer reach the terminal, so `le` stops
writing it there and keeps reading it from the guest (`terminalWriter`): the
guest ssh session survives, and the guest's teardown runs. Once the command ends, `le` says once, on stderr,
that its output stream stopped accepting writes, when, and why, and that the
run's captured output is intact: the same holds for an ENOSPC on a job log. A
staged record a killed `le` left before its rename is removed by the reap once
it is older than ten minutes. The VMs that `installer`, the hugepages proof,
crash capture, the MPLS boot and the hostname proof start outside `Run` carry neither mechanism.
<!-- source: internal/le/test/qemu/vmrecord.go -- recordVM, reapOrphanVMs, reapOne, reapUnreadable, removeStaleStaged -->
<!-- source: internal/le/test/qemu/vmrecord_other.go -- processStartTime -->
<!-- source: internal/le/test/qemu/run.go -- terminalWriter, runHostCommand -->
<!-- source: internal/le/test/qemu/run_procattr_linux.go -- childProcAttr -->
<!-- test: internal/le/test/qemu/vmrecord_test.go TestReapStopsOnlyAQEMUWhoseLeIsGone, TestReapKeepsARecordItCannotJudge, TestReapKeepsAYoungUnreadableRecord, TestReapRemovesAnOldStagedRecord, TestRecordVMLeavesOnlyTheRecord, TestExecutePlanRecordsTheVMWhileItRuns -->
<!-- source: internal/le/test/qemu/run_guestcommand.go -- guestCommandBody, guestInterruptRemote, runGuestCommand, stopGuestCommand, interruptGuest -->
<!-- source: internal/le/test/qemu/run.go -- commandSpec.OwnGroup, notifyInterrupts, runHostCommand -->
<!-- source: internal/le/test/qemu/run_exec.go -- executePlan, sshSession -->
<!-- test: internal/le/test/qemu/run_guestcommand_test.go TestGuestCommandInterruptReachesTheGuestBeforeTheVMStops, TestGuestCommandTeardownPastTheGraceStopsTheVM, TestGuestCommandSecondInterruptStopsTheVM, TestGuestCommandUndeliveredInterruptSaysSo, TestGuestCommandLineKeepsTheCommandsStatus, TestGuestCommandTimeoutForwardsTheInterrupt, TestGuestCommandSSHFailureAfterTheForwardSaysSSH, TestHostCommandOutlivesADeadTerminal, TestHostCommandOwnGroupLeadsItsOwnProcessGroup -->
<!-- test: internal/le/test/qemu/run_guestcommand_linux_test.go TestGuestInterruptLineSignalsTheRecordedGroup, TestChildProcAttrDiesWithLe -->

## Asking the appliance over SSH

The hugepages proof, crash capture, the MPLS boot and the hostname proof ask
the appliance their questions through one `sshpass` call, with the seeded
`admin` password. That ssh turns public-key authentication off
(`PubkeyAuthentication=no`), never asks the ssh-agent (`IdentityAgent=none`,
`IdentitiesOnly=yes`) and offers only the password
(`PreferredAuthentications=password`). Without those options, an agent holding
many keys made OpenSSH offer each one first, and Ze's SSH server, which allows
six attempts, ended the session with `Too many authentication failures` before
the password was typed.

A failed attempt is reported with the final line of ssh's stderr, preceded by
up to three earlier lines that say why the server ended the session (a
`disconnect`, `denied` or `authentication fail` line). OpenSSH prints that
cause before its closing `Disconnected from 127.0.0.1 port <port>`, so a
report that kept only the last line read as an appliance that dropped the
session.
<!-- source: internal/le/test/qemu/boot.go -- sshpassArgv, sshFailure, withSessionEndCause, namesSessionEnd -->
<!-- test: internal/le/test/qemu/boot_test.go TestTheApplianceSSHOffersOnlyThePassword, TestAnSSHFailureKeepsTheLineThatSaysWhy -->

## Stopping the hugepages proof

`./le test qemu vpp-hugepages` establishes its signal root before the host build.
SIGINT, SIGTERM and SIGHUP cancel the host compiler, appliance initialization or
image build, QEMU, and the active SSH query or retry wait. Every external command
uses the shared process-group owner; the action joins that owner before removing
its work directory and starts no later phase after cancellation. An interrupted
action returns no report payload for the renderer; completed pass, fail and skip
verdicts retain their reports. Normal completion preserves QEMU's cooperative
SIGINT-first shutdown and existing ten-second grace before group termination,
using the same identity-safe owner and sole reaper. Cancellation or an error
instead closes the owned group immediately. Both paths join it before returning.

The host compiler remains a native-host build, and no new admission queue is
introduced by this action. Process-fixture cancellation coverage is not evidence
that an appliance image built, a kernel reserved hugepages, or a VM booted. Those
claims still require the actual image-and-boot invocation on the intended host.
MPLS boot and crash capture retain their existing invocation roots; their shared
leaf call-site migrations do not establish a cancellation contract for those
actions.
<!-- source: internal/le/test/qemu/actions.go -- runHugepagesHere, runHugepages -->
<!-- source: internal/le/test/qemu/hugepages.go -- Hugepages.Run, buildHostZe, appliance -->
<!-- source: internal/le/test/qemu/boot.go -- Hugepages.boot, ask, ssh -->

## Appliance first-boot storage import

The host-side `installer` action boots a fresh appliance through the HTTP
installer, then checks its first runtime boot. It requires a logged explicit
seed import, a `/perm/ze/database/` tree with every seed key's original bytes,
and a retired `database.zefs.replaced-*` with no live `database.zefs` left.
SSH must accept the seeded credentials, and the web listener must present the
seeded TLS certificate. These checks use the services and the installed disk;
they do not depend on an offline command being callable through SSH.

Before runtime boot, the action writes a partial frame beneath
`database.import-tmp-interrupted/` on the installed disk. Successful import
then proves that abandoned staging cannot become the live store. This scenario
covers restart before publication. The storage package's interruption tests
cover the later boundary between tree publication and seed retirement.

```bash
ZE_INSTALL_KERNEL=$PWD/build/kernel/Image \
ZE_INSTALL_ARCH=amd64 ZE_INSTALL_KEEP=1 \
./le --name storage-proof test qemu installer
```

The kernel must match the target architecture and include the module-free
installer's virtio, network and ext4 support. The host needs Go, QEMU and
e2fsprogs (`mkfs.ext4`, `debugfs`, `e2fsck`); a source image build also needs its
kernel-builder runtime. Linux KVM access is described above. arm64 needs the
QEMU UEFI firmware, selectable with `ZE_INSTALL_AARCH64_BIOS`. Host tooling is
built for the host, independently of the target architecture.

`ZE_INSTALL_IMAGE` and `ZE_INSTALL_ZEFS` can name a fresh prebuilt image and its
matching seed artifact. That image must enable SSH on port 22 and HTTPS on port
8080 with the seed's `meta/web/cert` and `meta/web/key`, as the default appliance
template does. `ZE_INSTALL_SSH_USER` and `ZE_INSTALL_SSH_PASS` must match its
credentials. The check uses native Go SSH and TLS clients and needs neither
`sshpass` nor `uv`. `ZE_INSTALL_KEEP=1` retains the disk, serial log and extracted
store for diagnosis. A skipped action is not storage-import evidence.

<!-- source: internal/le/test/qemu/install_boot.go -- executeHTTP, bootTargetSSH -->
<!-- source: internal/le/test/qemu/install_storage.go -- seedInterruptedImport, assertImportedSeed, installSeedTLS -->
<!-- source: internal/le/test/qemu/install_build.go -- buildHostZeEnv, buildImage -->

## Appliance image update and recovery

`./le test qemu appliance-ota` is a manual deployment proof. It builds two named
images for the host's architecture, boots the first disk, and publishes a host
name through Ze's authenticated HTTPS configuration upload API. The upload
validates and commits the complete configuration; its port 8080 forward is
bound to host loopback, alongside the SSH and updater forwards. It then uploads
the second image through the real `ze appliance push --testboot` command and
requires the authenticated gokrazy status to name a different runtime build.
The persistent partition UUID, Ze login and committed host name must survive.
Finally it reboots without switching slots and checks the original runtime and
the same persistent configuration again. Testboot retains the original root as
the next boot selection; calling the updater's switch endpoint would instead
promote the tested root.
<!-- source: internal/le/test/qemu/install_ota.go -- executeApplianceOTA, otaGuest.await, otaGuest.assertHost -->

After update and recovery, the proof queries the authenticated daemon with
`show health | json`. It requires the live plugin and interface probes and
rejects missing, malformed or inconsistent reports. Unexpected degraded or down
components fail the proof. Only the exact inactive L2TP and IPsec outcomes are
permitted when the parsed seed omits their configuration. This checks the
seed's configured services; it does not prove unconfigured routing or VPN
traffic. Valid unhealthy reports are retried within `ZE_INSTALL_BOOT_TIMEOUT`;
command and malformed-data failures stop immediately.
<!-- source: internal/le/test/qemu/install_ota.go -- otaGuest.assertHealth, validateOTAHealth, otaInactiveHealth -->

Status requests have a ten-second ceiling. Configuration publication has a
two-minute ceiling to allow synchronous reload, and recovery reboot has a
five-minute ceiling to allow gokrazy's two 100-second service shutdown waits
and filesystem unmount. Every phase also obeys the parent context. SSH probes
close their connection when that context ends and join their cancellation
watcher before returning. The image's configured Ze username is set from
`ZE_INSTALL_SSH_USER` before credential assembly, alongside the proof password.
<!-- source: internal/le/test/qemu/install_ota.go -- uploadConfig, post, ssh -->
<!-- source: internal/le/test/qemu/install_build.go -- setApplianceArch, applianceEnv -->

```bash
ZE_INSTALL_QEMU_ACCEL=kvm ZE_INSTALL_IMAGE_SIZE=1610612736 \
ZE_INSTALL_KEEP=1 ./le --name appliance-ota test qemu appliance-ota
```

The proof boots the images' own runtime kernel and needs no separate installer
kernel. It needs the named appliance build prerequisites, QEMU and native KVM
or HVF access. It builds its own credentials and images; unset
`ZE_INSTALL_IMAGE`. `ZE_INSTALL_KEEP=1` retains the target disk and serial log
for diagnosis. A build or boot failure is a failed proof. The crash-capture
`ota-unaffected` action below checks reboot only and is not image-update
evidence.
<!-- source: internal/le/test/qemu/install_ota.go -- executeApplianceOTA, launchOTAGuest -->
<!-- source: internal/le/test/qemu/actions.go -- runApplianceOTAHere -->

## Appliance SSH configuration publication

The action passed twice on 2026-11-13 on a native amd64 host under KVM (QEMU
8.2.2, guest kernel 7.3.0-ze), each run building its own image and host
driver: three joined boots of one disk, publication by the real host driver,
retention of the hostname and of both logins across a fresh boot, and the
three refusals. It has not been run on arm64. The host driver and image it
proved were built from the tree of commit `f95746bc27` with that checkout's
uncommitted files, not from a clean commit, so it is not release-artifact
evidence.

`./le test qemu appliance-config-push` builds one current native appliance image
and drives its matching host driver's `appliance config-push` command. The
proof supplies the named SSH key to `appliance init --config` before secrets
are created. A private agent, child-only HOME and SSH_AUTH_SOCK, and a host pin
read from the stopped owned disk isolate its login from the operator's files.
One allocated SSH port is used by both the image listener and its loopback
QEMU forward, so the complete preview changes only the requested hostname.
<!-- source: internal/le/test/qemu/install_config_push.go -- executeApplianceConfigPush, configPushSeed, configPushPublish -->
<!-- source: internal/le/test/qemu/install_config_push_agent.go -- configPushIdentity, configPushAgent.stop -->

Three fresh QEMU processes boot the same disk with `-no-reboot`. Authenticated
gokrazy reboot must exit and join each process before read-only persistent
partition inspection. The first inspection supplies the host pin; later ones
require the active configuration pointer and resolve its selected version,
rather than accepting a stale active-file mirror. The stored configuration is
read under the name the daemon's own resolver gives it, which for a named
appliance is its instance name and not `ze.conf`. Each boot ends with a reboot
that asks for a machine reset (`kexec=off`): gokrazy's default reboot is a
kexec, which resets nothing, so `-no-reboot` would never exit. Fresh pinned
password and key logins check live configuration and retention after
publication and reboot.
Malformed receiver input, an authenticated observer denied configuration edits,
and an incorrect host pin must all refuse without a success receipt and leave
the complete live and selected persistent configuration unchanged.
<!-- source: internal/le/test/qemu/install_config_push.go -- configPushBootstrap, configPushReboot, configPushRefusals -->
<!-- source: internal/le/test/qemu/install_config_push_storage.go -- configPushDisk -->
<!-- source: internal/le/test/qemu/install_config_push_ssh.go -- configPushLive, configPushMalformed, configPushClient -->

The live configuration is the daemon's answer to `show config running | json`,
the one command that answers what the daemon applied. The proof compares that
answer only with other answers of the same command, so it holds no copy of the
daemon's projection or secret masking. The answer after the push is kept. The
stopped-disk read proves the stored source equals the preview, and the next
boot, which loads that stored source, must give the same answer. Each refusal
is followed by the same comparison against the answer taken before it.
<!-- source: internal/le/test/qemu/install_config_push_ssh.go -- configPushRunningQuery, configPushRunningEqual -->

A refusal counts only when the device itself answered it. The malformed control
sends its frame over a raw SSH session channel, as the host driver does, and
requires the device's own nonzero exit status: a client-side error is a failed
proof, never a refusal. The observer must first log in with its key and read
the accepted hostname, so that a login that fails cannot pass as an edit
refusal. The two controls that go through the host driver also require the
cause it prints: a refusal by the device to an authenticated session for the
edit denial, and a host key mismatch for the wrong pin. A nonzero exit alone
is also what a failed login or an unreachable device produces.
<!-- source: internal/le/test/qemu/install_config_push_ssh.go -- configPushExchange, configPushHostname -->
<!-- source: internal/le/test/qemu/install_config_push.go -- configPushRefused, configPushCauseDevice, configPushCausePin -->
<!-- test: internal/le/test/qemu/install_config_push_protocol_test.go -- TestApplianceConfigPushObserverLoginRequired -->

The action requires native KVM or HVF, refuses `ZE_INSTALL_IMAGE`, and uses the
named image's own runtime kernel. `ZE_INSTALL_KEEP=1` retains the disk and each
boot's serial log, but the private agent identity is always stopped and removed.
This is a live configuration publication proof on a copied raw image. The
separate installer actions prove ISO installation; the orchestration unit
fixture does not establish a VM run, automatic live rollback or routing traffic.
<!-- source: internal/le/test/qemu/actions.go -- runApplianceConfigPushHere -->
<!-- test: internal/le/test/qemu/install_config_push_protocol_test.go -- TestApplianceConfigPushNativeSequence, TestApplianceConfigPushFailureJoinsOwnedResources -->

## Appliance kernel crash capture

Two host-side actions prove kernel crash capture
(`docs/architecture/diagnostics/crash-capture.md`) on a booted appliance. Both
build an image whose `appliance.json` carries `image.crash-dump` with a 16 MB
reserve and whose seed adds `set system crash-dump enabled true`, boot it, and
read `show crashes | json` until readiness reports `armed`, `pstore-available`
and a writable `/perm/ze/crash`. Each then reboots the machine its own way.

| Action | Reboot | Passes when the next boot |
|--------|--------|---------------------------|
| `crash-capture-panic-harvest` | The seed sets `kernel.unknown_nmi_panic` and `kernel.panic`; the proof sends `nmi` to a QEMU monitor on its own TCP port after the monitor's first `(qemu) ` prompt, and keeps the connection open until the next prompt, because the monitor drops a line it has not run when the connection closes; the kernel then panics and warm-resets | Lists a kernel-kind artifact whose text carries the `Kernel panic` line |
| `crash-capture-ota-unaffected` | A POST to gokrazy's `/reboot` over a forwarded port 80, the kexec reboot `gok update` ends with, authenticated with the password gok resolved at build time | Is armed again and lists no kernel artifact |

A new boot is told from the old one by `boot-time-unix` in
`show host kernel | json`. Both actions are amd64 only, because an unknown NMI
panics only on x86 and gokrazy kexecs only on amd64; another architecture
answers SKIP. The functional wrappers are
`test/appliance/crash-capture-panic-harvest-qemu.ci` and
`test/appliance/crash-capture-ota-unaffected-qemu.ci`.

<!-- source: internal/le/test/qemu/crashcapture.go -- CrashCapture, injectNMI, otaReboot, gokrazyPassword -->

## Writing Integration Tests

### Which test each Linux-only change needs

| You wrote | You need |
|-----------|----------|
| `//go:build linux` source file | A matching `*_integration_linux_test.go` |
| termios / serial port code | A PTY-pair test (`creack/pty`, vendored) |
| netlink / interface code | A network namespace plus veth or dummy test |
| nftables / firewall code | A network namespace plus nft test |
| sysctl / kernel tuning | A procfs read test (a write may need `t.Skip`) |
| Any new Linux-only package | An entry in `integrationPackages`, `internal/le/test/qemu/alltests.go` |
| A Docker interop lab needing host-kernel features | A native `./le test qemu <feature>` action beside the Docker action |

### Build Tags

Two patterns, choose based on what the test needs:

| Build tag | Use when | Example |
|-----------|----------|---------|
| `//go:build linux` | Test imports linux-only types, or needs kernel privilege that a user namespace of its own gives it (`userns.Enter`, below): links, routes, XFRM, nftables, raw sockets, named network namespaces | `host/cpu_linux_test.go`, `core/routewatch/integration_linux_test.go` |
| `//go:build integration && linux` | Test needs a host grant (`/dev/ppp`), a binary `./le setup` does not install (`nft`), an explicitly supplied `ze` executable, or a privilege only the initial user namespace holds (`SO_RCVBUFFORCE`, an nftables payload write, BPF memlock) | `l2tp/bridge_integration_linux_test.go` |
| `//go:build docker` | Test starts a real server container through `labtest.Start`, on the host's Docker; never together with `integration`, because the guest has no Docker ("The docker population") | `interoplab/labtest/labtest_docker_test.go` |

Tests tagged `integration && linux` run through the applicable `./le test qemu`
action, which supplies `-tags integration`. Tests tagged only `linux` also run
in native unit groups on a Linux host.

A kernel-touching test, including one carrying an `RFC requirement:` tag, uses
`linux` and calls `userns.Enter`, so every Linux unit run executes it. It keeps
`integration && linux` only for a need from the second row above, and the
reason belongs in the file's header comment. Both tags still run in the guest, because `integration &&
linux` compiles every `linux` file. The native discrimination recorder tries the
host's actual build tags first, then a Linux guest with `integration`.
A guest observation requires Ze's runtime kernel through the `kernel`
keyword; a host observation rejects that keyword. See
[RFC discrimination placement](../../contributing/rfc-conformance-gates.md).

For tests needing only their own network, a fixed port or an address on `lo`,
`userns.Enter` (`internal/test/userns`) re-executes the test in a new user,
network and mount namespace, where the test is root over namespaces of its
own, and fails on every startup error, including namespace denial. An
unprivileged parent maps only its own uid and gid into the child; a root parent
(`sudo ./le test integration`) maps every id to itself (`idMappings`), so a test
binary under a `0750` home another user owns stays reachable to the child's root. Go reports
clone and exec errors through the same channel, so `Enter` cannot safely
classify an errno as a namespace refusal. A failed child's headline comes
from its own output, and the full output follows it. When the child never
started the test, the headline retains the error and names `./le setup install`
and its `userns-unrestricted` check as the fix when namespace policy caused it.
When the namespace setup failed, the headline is the failed step and its errno.
On an unprivileged run where
`kernel.apparmor_restrict_unprivileged_userns` reads 1, the guidance names
that AppArmor restriction and the `sysctl` that lifts it: the kernel then
creates the namespace without privilege, so the first mount fails with
`permission denied`. When the test body failed, the headline names each
failed test and subtest and does not blame the namespace. `Enter` is the first
statement of a top-level test, and such a test never calls `t.Parallel`,
because the child re-runs that one named test in a fresh process. A well-known
port bound there never collides with a ze running on the host or with a
parallel run of the same package.
<!-- source: internal/test/userns/userns_linux.go -- Enter, childFailure, usernsPolicyHint, idMappings -->

The child runs with `-test.run '^<Name>$' -test.v -test.count=1`, and the
parent reads the child's own top-level result line, `--- PASS: <Name> (` or
`--- SKIP: <Name> (` at the start of a line. A child that exits 0 with neither
line ran no test (a name the binary does not hold selects nothing and still
exits 0), and the parent then fails rather than pass on nothing. A subtest's
result line is indented and does not count: a skipped subtest neither skips nor
passes the test it belongs to.
A skip reaches the parent only when the child exits successfully and reports
that the selected top-level test skipped.
<!-- source: internal/test/userns/userns_linux.go -- Enter, childResult -->

Every flag the parent was given reaches the child with its value, read from
`flag.CommandLine` through `flag.Visit`, so a test's own flag
(`-flowexport-ze-path`, `-rsvp-producer-daemon`) reaches the body, and a
coverage run's `-test.gocoverdir` lets `./le rfc record` see the code the child
ran. `childFlagsWithheld` names the flags withheld, each with its reason: the
ones Enter sets itself (`-test.run`, `-test.v`, `-test.count`), the ones that
change what the child runs (`-test.cpu`, `-test.bench`, `-test.fuzz`,
`-test.fuzzworker`, `-test.list`), and the files the parent writes on exit
(`-test.testlogfile`, `-test.coverprofile` and the profile and trace flags).
<!-- source: internal/test/userns/userns_linux.go -- childCommand, childArgs, childFlagsWithheld, childResult -->

Most kernel-touching tests use it, among them the BFD wire tests
(`internal/component/bfd/rfc5881_wire_port_linux_test.go`), the interface,
IKE dataplane, FIB, firewall, VRRP, OSPF and IS-IS kernel tests, and the PPP
tests once the host grants `/dev/ppp` (`ppp-access`,
[developer setup](../../guide/developer-setup.md)).
`Enter` brings up the literal kernel-created `lo` inside that new namespace.
This is fixture topology, not an operator-selected logical interface, so the
direct lookup in `loopbackUp` is an explicit interface-resolution gate exemption.
<!-- source: internal/test/userns/userns_linux.go -- Enter, loopbackUp -->

Before the test body runs, the child makes its mount tree recursively private
and mounts a tmpfs on `/run/netns`, so a named network namespace
(`netns.NewNamed`, `ip netns add`, `ip netns exec`) works without privilege
and never appears on the host. Where the host has no `/run/netns`, the tmpfs
goes on `/run` and the child creates `/run/netns` in it, which hides the rest
of the host's `/run` from that test. The parent passes the device number of
the host's `/run/netns` to the child, and the child fails the test while its
own `/run/netns` still sits on that device. A mount step that fails fails the
test with the step and the errno, never a skip.
<!-- source: internal/test/userns/userns_linux.go -- childSetup, mountNetnsDir, netnsDevice -->

Off Linux, `Enter` answers true at once and the test runs in place, so a
portable test (the traceroute command tests) can call it without a build tag.
<!-- source: internal/test/userns/userns_other.go -- Enter -->

### File Naming

```
<feature>_linux_test.go                    # linux-only types, no kernel caps
<feature>_integration_linux_test.go        # kernel caps needed (QEMU only)
```

### Virtual Substitutes for Hardware

Never require physical hardware. Use kernel virtual devices:

| You need | Use instead | How |
|----------|-------------|-----|
| Serial port (`/dev/ttyS*`) | PTY pair | `pty.Open()` from `github.com/creack/pty` (vendored) |
| Network interface | dummy or veth | `ip link add ze0 type dummy` in a network namespace |
| Firewall table | nftables in netns | `nft add table ip ze_test` in an isolated namespace |
| Kernel routes | netlink in netns | `route.Add(...)` inside `netns.NewNamed(...)` |
| Block device | loop device on a tmpfs file | `losetup` |

A focused VM run that needs an extra Alpine package, such as `strace` or
`util-linux`, passes it after the `packages` keyword to `./le test qemu run`.

### Failing ONE Socket Under a Daemon

A test that asserts what an operator's monitoring shows while a socket fails
needs that socket to fail for a whole window, not once. No ordinary network
condition supplies one: an interface that goes down delivers silence, and a
device-bound `AF_PACKET` socket is told `ENETDOWN` once by `packet_notifier`
before it reverts to blocking.

`le test fail-syscall` supplies one. It installs a classic seccomp filter that
answers a chosen errno for a chosen syscall, then `execve`s the command, so no
tracer is in the path and the refused call is charged to the daemon's own CPU
time.

```
cmd=background:seq=1:exec=le test fail-syscall syscall recvfrom errno ENETDOWN length 1500 -- ze -:stdin=config
```

| Keyword | Meaning |
|---------|---------|
| `syscall` | which call fails, by name. `recvfrom` and `recvmsg` are what it knows, and a name it does not know is refused |
| `errno` | what the call answers, by name. Pick one the loop under test does NOT classify as a closed socket: `readDiscoveryFrame` maps `EBADF` and `EINVAL` onto `errSocketClosed` and LEAVES the loop, so either would prove nothing |
| `length` | selects ONE socket by the byte count its reader asks for, which is a per-loop constant. Optional; without it every descriptor in the process fails |

Three constraints the caller has to know:

- **`length` reads `args[2]`, which is a read length only for some calls.**
  `recvfrom(fd, buf, len, flags, ...)` carries the length there, so the keyword
  selects one socket. `recvmsg(fd, msghdr, flags)` carries the FLAGS there, and
  the launcher refuses the combination rather than compare against a flag word.
- **Arming is loud.** The launcher makes the selected call once itself, plus one
  call of a length it must NOT select, and refuses to `execve` anything if
  either answer is wrong. A filter that installed and selects nothing would
  otherwise leave the daemon healthy while the test reported an armed window.
- **Wrapped stdin daemons receive isolated storage.** The runner recognizes
  `le test fail-syscall ... -- ze -` and gives it a stable per-daemon config
  folder. The wrapper still needs its own readiness probe.

`CONFIG_SECCOMP` and `CONFIG_SECCOMP_FILTER` are pinned in
`gokrazy/kernel/runtime.require`, so a defconfig that stopped setting them fails
the kernel build rather than turning the scenario into a quiet pass.

<!-- source: internal/test/failsyscall/failsyscall_linux.go -- arm, buildFilter, proveArmed -->
<!-- test: test/l2tp/subscriber-reader-failing-socket.ci -- the first caller, with the measured numbers in its header -->

### Network Namespace Isolation

Tests that create interfaces or routes MUST use a dedicated network namespace
to avoid interfering with other tests or the VM's network. See
`internal/component/iface/integration_helpers_linux_test.go` for the
`withNetNS` helper pattern.

### Dataplane Counters Need a Real Remote Peer

<!-- source: ai/rules/platform-linux.md -- Dataplane counters need a real remote peer -->

A test that asserts on a kernel counter sitting behind state written for a
remote peer, such as `ip xfrm` bytes, sends its traffic to a real remote peer
and pairs the assertion with a run known to move the counter. A VM addressing
its own address matches no policy that names a peer, so no security association
encrypts the packet and the counter stays at zero. The reading is then zero for
a working path and zero for a broken one. Two namespaces, two VMs, or two
containers are what make the counter readable.

The selector is the reason, not the interface. A plain nftables rule counter in
an input or output chain does advance for a self-addressed packet, so this
constraint does not reach it.

### Graceful Degradation

Use `t.Skip()` when a capability is missing, not `t.Fatal()`:

```go
master, slave, err := pty.Open()
if err != nil {
    t.Skipf("cannot open pty: %v", err)
}
```

This keeps the same test file usable in environments with different
capabilities.

The one exception is a missing `docker` binary under `ZE_REQUIRE_DOCKER=1`,
which `./le test integration docker-peers` and `live-rpki` set: the test fails,
because a run that skipped every container would report a pass it never earned.
<!-- source: internal/le/interoplab/labtest/labtest.go -- Start, dockerRequired -->

### Registering a New Package

Add the package to `integrationPackages` in
`internal/le/test/qemu/alltests.go`. If it needs an Alpine package, add that package
to the owning native action. Do not add a guest script or a second QEMU
lifecycle.

For a distinct guest proof, add one action to `internal/le/test/qemu/actions.go` and
keep the action callable from Go. The host recipe invokes it through:

```text
./le test qemu run kernel <vmlinuz> packages "<packages>" \
  command '<guest-le-binary> le test qemu <action>'
```

The host prepares the binary and kernel; the guest action owns only the proof.

## VM Environment

The guest is an Alpine live system with no systemd. It provides:

| Feature | Available | Notes |
|---------|-----------|-------|
| Root access | Yes | All capabilities |
| PTY pairs | Yes | `/dev/ptmx` |
| Network namespaces | Yes | `ip netns` |
| nftables | Yes | Installed through the `packages` keyword of `./le test qemu run` |
| Go toolchain | Yes | Downloaded and cached under `tmp/qemu/` |
| Repository | Yes | Mounted read-write over virtio-9p at `/workspace` |
| Kernel modules | **No** | See below |
| systemd | **No** | Alpine uses OpenRC, or the test skips |
| Physical serial ports | **No** | Use PTY pairs |
| Multiple physical NICs | **No** | Use veth pairs |
| GPU or display | **No** | Every run is headless |
| Persistent state | **No** | Boots fresh from the ISO each run |

### No module loads in the guest

The VM pairs Ze's runtime kernel with Alpine's initramfs and Alpine's
`/lib/modules`, which are built for the ISO's own release. No module loads at
all: not Alpine's, and not one built from Ze's kernel tree. Every symbol a QEMU
run needs therefore has to be `=y` in `gokrazy/kernel/*.config`, and the
matching `gokrazy/kernel/*.require` manifest is what makes a silent demotion to
`=m` fail the build instead of the test. `CONFIG_PPP`, `CONFIG_L2TP`,
`CONFIG_PPPOE`, `CONFIG_VLAN_8021Q`, `CONFIG_DUMMY` and the qdisc set are all
`=y` for this reason. So is `CONFIG_XFRM_INTERFACE`, the device `set interface
xfrm` creates and a `vti bind` IPsec peer installs its Child SA against: the
first QEMU run of `test/plugin/show-mtu-oversized-tunnels.ci` found it unset,
and the shipped kernel answered `link add: operation not supported`.

`Run.setupCommand` still issues `modprobe` for ppp, l2tp and netfilter modules,
each with `|| true`. Those lines are best-effort and load nothing on a
`kernel`-supplied boot. They are not the reason those features work.

<!-- source: internal/le/test/qemu/run.go -- Run.setupCommand, the best-effort modprobe list -->
<!-- source: gokrazy/kernel/runtime.config -- "no module of this kernel can load" -->
<!-- source: gokrazy/kernel/runtime.require -- the symbols a =m answer must fail -->

## Interop labs need a QEMU path too

A Linux-only interop lab that runs as Docker containers and depends on
host-kernel features (L2TP, PPPoE, netfilter) runs on neither macOS nor a
plain CI runner by itself: Docker Desktop's VM lacks the kernel modules, and
the Alpine QEMU VM of `./le test qemu run` has no Docker (the docker-host image
of `./le test qemu docker-lab` has, see "Docker labs on the docker-host
image"). Each such lab therefore ships two native
actions.

| Lab | Docker action | QEMU action | Native producer |
|-----|---------------|-------------|-----------------|
| L2TP (Ze LNS against xl2tpd) | `./le test deployment docker-l2tp-ppp` | `./le test deployment gokrazy-l2tp-ppp` | `internal/le/test/deployment` |
| PPPoE (Ze client against accel-ppp) | `./le test deployment docker-pppoe-accel` | `./le test qemu pppoe-accel` | `internal/le/test/qemu/pppoe_accel_linux.go` |
| VRRP (Ze against keepalived) | `./le test integration interop`, scenario `vrrp-mastership-keepalived` | `./le test qemu vrrp-keepalived` | `internal/le/test/qemu/vrrp_keepalived_linux.go` |
| VRRPv2 owner discard (Ze owner against a conflicting keepalived) | `./le test integration interop`, scenario `vrrp-v2-owner-keepalived` | `./le test qemu vrrp-keepalived scenarios v2-owner-keeps-the-address` | `internal/le/test/qemu/vrrp_owner_linux.go` |
| MOBIKE (Ze initiator and responder against strongSwan) | `./le test integration interop-ipsec`, scenarios `mobike-initiator` and `mobike-responder` | `./le test qemu ipsec-mobike kernel <vmlinuz>` | `internal/le/interoplab/ipsec/mobike_netns_linux.go` |

<!-- source: internal/le/test/deployment/actions.go -- gokrazy-l2tp-ppp, docker-l2tp-ppp, docker-pppoe-accel -->
<!-- source: internal/le/test/qemu/actions.go -- pppoe-accel, vrrp-keepalived -->

The L2TP appliance proof creates a private host bridge between the QEMU TAP
and the xl2tpd namespace. When UFW is active, setup permits host INPUT on
that bridge for DHCP and forwarding with both the incoming and outgoing
interface restricted to that same bridge. Bridge netfilter can otherwise
drop L2TP UDP traffic even when the readiness ping succeeds. Setup refuses
if it cannot inspect UFW or install either rule. Cleanup removes both rules;
the proof does not disable UFW or change global bridge-netfilter settings.
<!-- source: internal/le/test/deployment/gokrazylab.go -- allowBridgeThroughFirewall, clearBridgeFromFirewall -->

The appliance proof's signal context also reaches kernel-cache resolution,
host-driver compilation, database initialization, image construction, lab setup,
QEMU and its LAC. Each external command has the shared runner process-group
owner, which joins owned descendants before the phase returns. The lab keeps
dnsmasq in the foreground under that owner rather than treating a pid file as
authority to signal a process. Cancellation is an interrupted proof, not a
failed PPP verdict. Cleanup joins its DHCP owner and removes only this lab's
network and firewall objects on an uncanceled context.
<!-- source: internal/le/test/deployment/gokrazyimage.go -- gokrazyImage, prepareProofDatabase, buildGokrazyImage -->
<!-- source: internal/le/test/deployment/gokrazyl2tp.go -- proveAppliance, observe -->
<!-- source: internal/le/test/deployment/gokrazylab.go -- setup, startDNSMasq, remove -->

The MOBIKE action is a manual runtime-kernel proof, not a merge gate or a
scheduled job. It runs both existing movement scenarios, without a selector:

```text
./le --name mobike-proof test qemu ipsec-mobike kernel <runtime-vmlinuz> timeout 1200s
```

The host builds a static Linux daemon and the native guest runner through build
admission, with tags derived from `feature-gates.txt`. Both artifacts live under
the invoking session's scratch directory until the VM run ends; host tools are
not cross-compiled. The existing QEMU harness boots the supplied runtime kernel,
checks its release, installs Alpine's `strongswan`, `iproute2`, `iputils` and
`util-linux` packages, and invokes
`le test deployment ipsec-mobike daemon <guest-daemon-path>` in the guest.
No Docker daemon or guest Go build is involved.

The deployment action also accepts explicit `charon <path>` and `swanctl <path>`;
its defaults are Alpine's `/usr/lib/strongswan/charon` and `/usr/sbin/swanctl`.
The adapter creates private namespaces and process directories and reuses the
Docker scenarios' MOBIKE checks. A pass requires unchanged IKE and Child SA
identities, new installed endpoints with no stale states, and encrypted traffic
in both directions before and after the old address is removed. A new SA or an
unprotected ping cannot satisfy those checks.
<!-- source: internal/le/test/qemu/mobike.go -- runIPsecMOBIKEHere, buildMOBIKEGuests -->
<!-- source: internal/le/test/deployment/mobike.go -- runIPsecMOBIKEHere -->
<!-- source: internal/le/interoplab/ipsec/mobike_netns_linux.go -- RunMOBIKENetns -->
<!-- source: internal/le/interoplab/ipsec/mobike.go -- checkMOBIKE -->

The native PPPoE and VRRP guest labs create their scenario work directories
under the guest's `/tmp`, where the invoking user owns them, and write Ze's
explicit input to `<scenario-work>/ze/ze.conf`. `ZE_CONFIG_DIR` selects that same
private directory. This avoids a root daemon opening storage beneath a checkout
owned by the host user. The database tree is created beside the input, and a
VRRP restart reads the explicit file again while retaining the scenario's tree.
Retained failure artifacts live in the guest and last for that guest's lifetime.
<!-- source: internal/le/test/qemu/pppoe_accel_linux.go -- runPPPoEAccelGuest -->
<!-- source: internal/le/test/qemu/vrrp_keepalived_linux.go -- startZe -->

`./le test qemu vrrp-keepalived` runs five scenarios, selectable with
`scenarios=<csv>`.

| Scenario | What it proves |
|----------|----------------|
| `QS-1` | Ze wins the election against keepalived and the advertisement fields match the RFC. |
| `QS-2` | Ze dies, keepalived promotes within the master-down band and sends a gratuitous ARP; Ze returns and preempts. |
| `QS-3` | The skew-time term of the master-down interval is honored. |
| `tracked-uplink-hands-the-vip-to-keepalived` | Ze at priority 200 tracks a veth worth a decrement of 150. The veth goes down, Ze advertises 50, and keepalived at 100 takes the VIP; the veth returns and Ze preempts. keepalived's own notify script is the assertion, so ANOTHER implementation acts on the decremented priority. |
| `v2-owner-keeps-the-address` | Ze is the VRRPv2 address owner of `192.0.2.254` (VRID 20). keepalived claims the same VRID and address at priority 255 and advertises by unicast from `192.0.2.252`, above Ze's source `192.0.2.251`, so only the owner discard of RFC 3768 Section 7.1 keeps Ze Master. Ze must keep the `/32` on its virtual-MAC interface for 8 seconds, log no state change, and count at least 3 `owner` discards in `show vrrp statistics`, read through Ze's own CLI. With the discard removed, Ze loses the address. |

The first three names predate the rule that an interop scenario is NAMED rather
than numbered (`ai/rules/interop-and-goal-validation.md`). Renaming them is not
the tracking scenario's work, and the later scenarios do not copy the pattern.

The owner scenario is the QEMU path of the Docker scenario
`vrrp-v2-owner-keepalived`, with the same address ordering: the owned address
sits above keepalived's, so a tie-break that compared the sender with the first
virtual address instead of the address Ze sends from would also hide the missing
discard. It adds an SSH server on `127.0.0.1` and a fixture account to Ze's
config, because the discard count is only visible through the CLI.

<!-- source: internal/le/test/qemu/guestlabs.go -- vrrpScenarioNames -->
<!-- source: internal/le/test/qemu/vrrp_keepalived_linux.go -- runTrackedUplink -->
<!-- source: internal/le/test/qemu/vrrp_owner_linux.go -- runOwnerKeepsTheAddress -->

## Reference Implementations

| What | File |
|------|------|
| Network namespace helper | `internal/component/iface/integration_helpers_linux_test.go` |
| Netlink integration test | `internal/plugins/traffic/netlink/integration_linux_test.go` |
| nftables integration test | `internal/plugins/firewall/nft/integration_linux_test.go` |
| Kernel test in a namespace of its own (`linux` tag, `userns.Enter`) | `internal/core/routewatch/integration_linux_test.go` |
| PTY/termios integration | `internal/component/config/system/console_integration_linux_test.go` |
| QEMU runner | `internal/le/test/qemu/run.go` |

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| "Needs real hardware, skipping test" | Use the virtual substitute in the table above |
| `integration && linux` on a kernel-touching test a user namespace serves | Use `//go:build linux` and call `userns.Enter` first, including in RFC-tagged tests. Keep `integration` only for a need from the second Build Tags row |
| A new Linux package absent from `integrationPackages` | The test compiles and never runs. Add it to `internal/le/test/qemu/alltests.go` |
| `t.Skip` for a refused privilege inside `userns.Enter` | Fail the test with the operation and the errno: the namespace grants the privilege, so a refusal is a defect a skip would hide. `t.Skip` stays for an absent kernel feature or module |
| Hardcoding `/dev/ttyS0` | Use `pty.Open()` for a real PTY pair |
| Reading a QEMU timeout as "TCG is slow" | On Linux, check `kvm-access` first with `./le setup check`. A user outside the `kvm` group makes QEMU refuse to start, which surfaces as a timeout |
| Selecting the accelerator on the existence of `/dev/kvm` | Existence is not access. Probe for read and write, and take an explicit `hvf` branch on darwin |

## Troubleshooting

**VM fails to boot:** Check that `qemu-system-x86_64` (or `qemu-system-aarch64`
on ARM Macs) is installed. On macOS: `brew install qemu`.

**Tests time out:** The default timeout is 120 seconds. Change the timeout in
the owning action under `internal/le/test/qemu`.

**Package not found in Alpine:** Check the Alpine package name at
`https://pkgs.alpinelinux.org/`. Alpine package names sometimes differ from
Debian/Ubuntu (e.g., `iproute2` not `iproute`).

**Go module download fails:** The VM needs internet access. QEMU's user-mode
networking provides NAT. Check that the host has connectivity.

## Existing Integration Test Packages

The population is `integrationPackages` in `internal/le/test/qemu/alltests.go`, a
closed list. `TestEveryIntegrationPackageIsNamed` derives every package holding
an `integration`-tagged test file from the tree and fails when one is absent
from that list; `TestEveryNamedIntegrationPackageExists` fails on a named
package that is not in the tree. Read the Go list rather than a copy of it.

<!-- source: internal/le/test/qemu/alltests.go -- integrationPackages -->
<!-- source: internal/le/test/qemu/integration_coverage_test.go -- TestEveryIntegrationPackageIsNamed, TestEveryNamedIntegrationPackageExists -->

A package holding `integration` tests that cannot run in the guest is named in
`excludedIntegrationPackages` instead, against a reason a reader can check.
`internal/le/interoplab` is one: `TestDockerKernelCheckOnThisHost` judges the
kernel of the host's own Docker daemon, which the guest cannot stand in for.

### Non-root fixture process-control proof

`TestSysribExecutablePrivilegedCancellationJoinsRootChild` needs the non-root
fixture role with effective `CAP_KILL` and the native guest's passwordless sudo
account. The ordinary root package pass skips that role. Its dedicated carrier
uses the existing `./le test qemu docker-lab kernel <vmlinuz> root command`
action: compile `fixture.test` with CGO disabled on the host, copy it onto an
executable guest tmpfs, grant only `cap_kill+ep`, then run it as the cidata user
`ze`. The test requires an actual stopped root child, cancellation of the real
inspection runner and that child's exit after the join. A named PASS with no
skips is required; a successful boot or a root-role skip proves nothing.
<!-- source: internal/test/fixture/sysrib_process_executable_cancellation_integration_linux_test.go -- TestSysribExecutablePrivilegedCancellationJoinsRootChild -->
<!-- source: internal/le/test/qemu/alltests.go -- excludedIntegrationPackages -->

From the checkout on the native host, build the guest test binary using the
feature manifest. Pass `GOOS=linux` only to the admitted child build, so the
host le stays native. The guest artifact's architecture stays the host's,
matching the runtime kernel this action accepts.

```bash
. ./feature-tags
rp5_tags="ze_core,$(feature_tags feature-gates.txt ,),integration"
rp5_scratch="$(./le --name session session scratch ensure)"
rp5_fixture="$rp5_scratch/fixture.test"
CGO_ENABLED=0 ./le --name session job run label fixture-control-build quiet command \
  env GOOS=linux go test -c -race=false -p 1 -tags "$rp5_tags" -o "$rp5_fixture" ./internal/test/fixture
```

Set `rp5_kernel` to the absolute matching runtime `vmlinuz` path from the image
build. The command branch shares the checkout and scratch at their absolute
host paths; `root` does not create a `/workspace` binding. Quote the compiled
artifact path as shell data when constructing the guest command:

```bash
rp5_quote() { printf "'%s'" "${1//\'/\'\\\'\'}"; }
rp5_fixture_argument="$(rp5_quote "$rp5_fixture")"
rp5_guest_command="set -e
mkdir -p /tmp/rp5-control-exec
mount -t tmpfs -o mode=0755,nodev tmpfs /tmp/rp5-control-exec
cp $rp5_fixture_argument /tmp/rp5-control-exec/fixture.test
chmod 0755 /tmp/rp5-control-exec/fixture.test
setcap cap_kill+ep /tmp/rp5-control-exec/fixture.test
getcap /tmp/rp5-control-exec/fixture.test
cd /tmp/rp5-control-exec
sudo -n -u ze env HOME=/tmp TMPDIR=/tmp /tmp/rp5-control-exec/fixture.test -test.v -test.count=1 -test.timeout=1m -test.run '^TestSysribExecutablePrivilegedCancellationJoinsRootChild$'"
CGO_ENABLED=0 ./le --name session job run label fixture-control-proof quiet command \
  ./le --name session test qemu docker-lab kernel "$rp5_kernel" root timeout 45m command "$rp5_guest_command"
```

The private tmpfs omits `nosuid`, so the kernel honors its file capability.
Its mode0755 also permits the cidata user to traverse it when that user's UID
differs from the namespace role's UID1000. The cidata user `ze` has the host
owner's UID in command mode and the existing passwordless sudo policy. This
test needs only the observed `cap_kill+ep` grant on its compiled test binary;
the namespace fixture's broader grant is a separate role. The native action
stops the VM and reclaims the private mount after the selected command ends.

## The docker population

The guest has no Docker, so a package test that starts a real server container
through `labtest.Start` carries the build tag `docker` and never `integration`.
`./le test integration docker-peers` runs that population on the host: it
derives every package under `internal/` and `cmd/` holding a test file that needs
the `docker` tag from the tree, so a package added later runs with no edit. It
compiles them with the checkout's feature tags plus `docker` (`live` for
live-rpki), the set `./le test unit` gives the same packages
(`Toolchain.TestTags`): such a package also holds untagged tests, and under
`docker` alone the gated schemas never register, so a test parsing product
config fails on a block it cannot name. It sets
`ZE_REQUIRE_DOCKER=1`, under which an absent `docker` binary fails the test
instead of skipping it. A file needs a tag when its `//go:build` constraint
holds with the tag set and cannot hold without it, whatever the other tags:
`docker && linux` needs `docker`, while `!docker`, `integration && !docker` and
`docker || integration` do not. The integration population is derived the same
way. `TestDockerTagNeverJoinsIntegration` refuses any test file whose
constraint names both `docker` and `integration`, whatever the operators. A
file needing both runs nowhere. `docker || integration` and
`(docker && linux) || integration` need neither tag, so they join neither
population: docker-peers never lists them, and nothing runs them with Docker.
A negation still counts: `integration && !docker` ties the file to both
runners, so it is refused too. Split such a file into a `docker` file and an
`integration` file. `./le test integration live-rpki` runs the `live`-tagged
RPKI tests through the same switch. Both run their `go test` child under the
verb's signal root: a SIGINT or SIGTERM kills the child's whole process group,
so `le` answers non-zero at once rather than waiting out the child's own
900 s bound. Before launch, the runner passes its Docker client's launch id
through `Docker.ChildEnvironment`. That kill skips every `labtest` `t.Cleanup`,
so a signaled run calls `Docker.CleanupOwnedOrphans` on that same client.
This non-dry sweep uses a context the signal did not cancel, bounded at 60 s,
and prints its summary. Only resources with that launch id can be removed,
after the ordinary owner checks prove their creating process gone. Resources
with another launch id or no launch label stay. Reaping `go test` does not
reap the test binaries it started, which are members of its process group and
may still be exiting, and a binary still exiting reads as a live owner. So the
sweep first waits, for at most 10 s of its 60, until `kill(-group, 0)` answers
`ESRCH`; a group still holding members then is swept anyway, and the summary
names the pids left and the cleanup to run once they exit. The group is asked
again after `ps` lists its members, so a last member that exits during the
listing counts as an empty group and draws no warning. A group still present
whose members `ps` cannot list, or does not list (one that joined after the
listing, or one hidden from this user), is named as such instead of by pid. A child that ends on its own
ran its cleanups and gets no sweep. Both run on the hosted runner's own Docker
in `evidence-nightly.yml`, because no container in either runs Ze, so the Docker
host kernel check does not apply.
<!-- source: internal/le/test/integration/dockerpeers.go -- dockerPeersArgv, runRequiringDocker, sweepAfterSignal, requireDockerEnvironment, liveRPKIArgv -->
<!-- source: internal/le/interoplab/run_identity.go -- Docker.ChildEnvironment -->
<!-- source: internal/le/interoplab/orphans.go -- Docker.CleanupOwnedOrphans, Docker.sweepOne -->
<!-- source: internal/le/gaterun/gaterun.go -- StreamContext, RunContext -->
<!-- source: internal/le/gaterun/group.go -- ProcessGroup.AwaitExit -->
<!-- test: internal/le/test/integration/dockerpeers_signal_test.go TestSignalSweepWaitsForTheGroup, TestSignalSweepNamesMembersLeft -->
<!-- source: internal/le/test/buildtag/buildtag.go -- TestFile.Needs, TestFile.Names, Packages -->
<!-- source: internal/le/test/qemu/integration_coverage_test.go -- TestDockerTagNeverJoinsIntegration, dockerIntegrationOverlap -->
<!-- source: internal/le/test/integration/gates.go -- Table, the docker-peers and live-rpki rows -->
