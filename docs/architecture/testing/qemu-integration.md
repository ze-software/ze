# QEMU Integration Testing

Ze runs on Linux (gokrazy appliances, Debian/Ubuntu servers). Code that uses
Linux-only syscalls (termios, netlink, nftables, sysctl) cannot be tested on
macOS or in Docker Desktop. Ze uses a QEMU Alpine Linux VM to run these tests
with full kernel capabilities.

## Quick Start

```bash
# Build the host driver, then the amd64 runtime kernel with Docker.
CGO_ENABLED=0 ./le --name qemu-setup build-artifacts host
./ze-host appliance kernel --target runtime --arch amd64 --builder docker

# test qemu all runs INSIDE the guest. le test qemu run boots the guest and carries it in.
./le --name qemu-current test qemu run kernel "<kernel-directory>/vmlinuz" packages "iproute2 libcap" \
  command "./le test qemu all"
```

Replace `<kernel-directory>` with the directory the kernel builder prints.
This example targets an amd64 guest. Use the matching architecture for other
guests. `build-artifacts host` builds only `ze-host`; it does not stage a kernel.

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

The first run fetches the Alpine ISO and Go toolchain. Alpine is cached under
`${XDG_CACHE_HOME:-$HOME/.cache}/ze/alpine-iso/`; Go archives remain under
`tmp/qemu/go-dl/`. Later runs reuse those downloads.

### The two entry points

Both entry points run one VM for the whole population, never one VM per test.

| Command | Population |
|---------|------------|
| `./le test qemu netns suites <comma-separated-suites>` | The explicit kernel-dependent subset for each selected suite. `plugin` selects the seven ASPA cases and the RFC 2545 next-hop replay case |
| `./le test qemu run ... command "./le test qemu all"` | Every functional suite, the Linux unit pass, the installer phase, and every registered integration package. Four of the suites run in a per-test network namespace, which needs `packages "iproute2 libcap"` |
| `./le test qemu run ... command "./le test qemu all only needs-linux"` | The same suites, each narrowed to the `.ci` tests marked `option=needs-linux`. The unit, installer and integration phases stay whole, and the report names the population it covered |
| `./le test qemu run ... command "./le test qemu all test test/<dir>/<name>.ci"` | That one `.ci`, through the suite that walks its directory (`vmSuiteFor`), with the same shim, environment and network-namespace preparation. No other suite or phase runs. `./le feature run` and `./le rfc record` reach a capability-gated test this way, through `testqemu.OneCICommand` and `testqemu.OneCIPackages` |

`test qemu all` discovers the suites' tests through the native runner, so a new
`needs-linux` test needs no registration there. `netns` uses the explicit
named populations in `netnsSelections`; selecting a suite does not run every
test in its directory.

<!-- source: internal/le/test/qemu/alltests.go -- AllTestsRun.Run, the phase population -->
<!-- source: internal/le/test/qemu/guestlabs.go -- the netns suite selection -->

### `test qemu all` is a GUEST action, and four things must be true before it runs

`le test qemu all` refuses to start outside the VM: `internal/le/test/qemu/actions.go`
registers it with "This action runs inside the VM. Its caller is the command
value passed to the host qemu run action." Typed on the host it answers
`qemu: the repository is not mounted: /workspace` and nothing runs. The host
driver is always `./le test qemu run ... command "<the guest command>"`.

Four preconditions, each of which fails with a message that does NOT name the
precondition. Measured 2026-09-04 and 2026-09-05, seven guest boots to establish:

| Precondition | What its absence looks like |
|--------------|-----------------------------|
| `packages iproute2` | `ZE-OBSERVER-FAIL: ... ip: invalid argument 'replace' to 'ip'`. BusyBox `ip` has no `neigh replace`, so an observer that programs a neighbour dies in test setup, before any assertion |
| The two daemon binaries, under canonical names | `qemu: bin/ze-stripped is missing or not executable -- cross-compile it on the host first`. `le test qemu run` shares the checkout, where the cross-built artifacts carry `-linux-arm64` suffixes, so name them with `ZE_BIN` and `ZE_STRIPPED_BIN`. `shim()` symlinks them to `ze` and `ze-stripped`, and links `le` to the running guest le (`guestLeLink`), because the tools dispatch on basename |
| The three binaries, STATICALLY linked | `qemu: bin/ze: is dynamically linked against /lib64/ld-linux-x86-64.so.2, which the musl guest does not have`, and nothing runs. `runnableInGuest` (`alltests.go`) reads each binary's PT_INTERP header before the first child. Until 2026-09-05 `verify()` only stat'd the file, and the same cause surfaced as 326 identical per-test failures (`start ze: fork/exec ... : no such file or directory`), because the kernel answers ENOENT for the LOADER while naming the BINARY. Build with `CGO_ENABLED=0`, which is what the native toolchain sets (`internal/le/go/toolchain`) |
| `packages libcap` | `qemu: setcap is not on the guest PATH, so the per-test network namespace suites cannot hold their capabilities`. The four routed suites run ze as an ordinary user inside a fresh namespace, and `setcap` is what gives that user CAP_NET_ADMIN |
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

`coreutils` was a fifth precondition until 2026-09-05. The suite wrapper passed
GNU `timeout --kill-after=15s`, which the guest's BusyBox `timeout` answers with
`unrecognized option` and exit 1, so every suite printed the usage text under its
own header and ran no test. The wrapper now passes `-k 15`, which BusyBox and GNU
coreutils both accept (`killAfterFlag`, `internal/le/test/qemu/alltests.go`). A run
with `packages "iproute2"` alone reached ten suites and several thousand `.ci`
tests on 2026-09-05, with no BusyBox usage text anywhere in its log.

### Running ONE `.ci` test in a throwaway guest

For a single Linux-only test, reproduce the shim that `test qemu all` normally
creates. Build the target binaries first; the QEMU action builds the guest `le`
itself:

```bash
./le test qemu run \
  kernel tmp/kernel/build/vmlinuz packages "iproute2" command '
    set -e
    shim=$(mktemp -d /tmp/ze-bgp.XXXXXX)
    ln -s /workspace/bin/ze-linux-arm64 "$shim/ze"
    ln -s /workspace/tmp/qemu/linux-arm64/le "$shim/le"
    ln -s /workspace/bin/ze-stripped-linux-arm64 "$shim/ze-stripped"
    cd /workspace
    PATH="$shim:$PATH" LE_TEST_NO_BUILD=1 \
      ZE_BIN=/workspace/bin/ze-linux-arm64 "$shim/le" test bgp plugin <test-name>
  '
```

`LE_TEST_NO_BUILD=1` and `ZE_BIN` are not optional. Without them the runner
builds its own ze from the source tree inside the guest and never looks at the
`ze` on `PATH` (`NewRunner` and `setupBinShims`, `internal/test/runner/runner.go`).
The test then passes or fails on a binary you did not build. A red you forced by
cross-building a broken ze, or by an overlay, never reaches the test, and the
green it reports proves nothing. `test qemu all` sets both for every suite
(`environment`, `internal/le/test/qemu/alltests.go`).

`./le test qemu run` holds the host's admission slot itself, under the label
`qemu-run`, and its guest setup writes a stand-in entry in the guest's own
`/tmp` and exports it as `ZE_RUN_JOB`, so the guest runner runs inside that slot
instead of touching the host's PID registry: neither machine can judge whether
a process on the other is alive. A guest le that admitted itself would also
fingerprint the whole checkout over 9p, where `git diff HEAD` reads every
tracked file because the index's stat data matches none of them; on 2026-10-27
that ran past the 60-second git limit and killed `le test firewall` before its
first test. The run never attaches to another run's verdict: its kernel image
and cross-built binaries are untracked, so equal words over an equal tree can
still judge different code.

<!-- source: internal/le/test/qemu/run_admit.go -- admitQEMURun, runArgv -->
<!-- source: internal/le/test/qemu/guestle.go -- guestRunParentSetup, guestRunJobParent -->

<!-- source: internal/le/test/qemu/actions.go -- test qemu all is registered as a guest action -->
<!-- source: internal/le/test/qemu/alltests.go -- suiteCommand, shim, the ZE_*_BIN knobs -->
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

### Four suites do not run in the guest root namespace

`test qemu all` runs inside an SSH session, and that session's transport lives in
the guest ROOT network namespace. A suite that programs the firewall, the
routing policy or an interface THERE reaches its own transport.

It did. On 2026-09-05 `test/firewall/firewall-nat-exclude.ci` installed a nat
prerouting chain in the guest root namespace, the run died inside
`functional/firewall` with ssh's own exit 255, and the twelve suites, the unit
pass, the installer phase and the integration phase still to come never ran
(`plan/journal/gate-excludes-part-of-its-population.md`).

Every row of `vmSuites` therefore states its namespace, and a row that states
none is refused before the run starts.

| Namespace | Suites | What the child gets |
|-----------|--------|---------------------|
| `guest-root` | every other suite | the guest's own namespace, and ze as root |
| `per-test` | `firewall`, `policy`, `ospf`, `ospfv3` | a fresh namespace for each test, entered by the `.ci` runner before it spawns anything, and ze as uid 1000 holding file capabilities |

The runner enters the namespace itself, once per test (`enterTestNetns`,
`internal/test/runner/netns_linux.go`), and only when `ZE_TEST_NETNS` is set
with a non-root `ZE_TEST_UID`. `test qemu all` sets both for a routed suite, copies
`ze` and `ze-stripped` to the guest's own tmpfs (a 9p mount carries no extended
attribute, so a `setcap` on `/workspace` would buy nothing), and gives the
dropped user a state directory to write in.

Routing a suite also un-skips its `option=netns-link` tests. Outside this mode
the runner SKIPS them (`applyNetnsLinkGate`, `internal/test/runner/caps.go`),
so eight `test/ospf` and three `test/ospfv3` tests ran in no VM phase at all.
The `plugin` suite stays in the guest root namespace under `test qemu all`; its
ASPA `netns-link` cases are therefore skipped there. The separate
`netns suites plugin` selection runs the seven ASPA validation and policy
cases with their declared `eth1` address, `10.0.0.254/24`. Their received
NEXT_HOP, `10.0.0.1`, is then nonlocal, on-link and syntactically valid.
The selection also includes `adj-rib-in-replay-rfc2545-next-hop`. That carrier
owns three distinct namespaces for the speaker, recipient and next-hop entity,
connected by real veth/bridge links; it no longer requests a dummy `netns-link`.
The fixture checks address ownership and attachment before traffic. Both live
forwarding and replay must retain the complete global/link-local pair on that
common subnet. The fixture owns shutdown after both recipient assertions.
The selector does not move the whole plugin suite into namespaces.

`./le test qemu netns suites <names>` is the same launcher over a named subset,
and it also asserts the guest root nft ruleset is unchanged by the run. It is
the tight loop; `test qemu all` retains the whole-suite namespace table above.

The plugin subset is scheduled in `qemu-nightly.yml`'s
`runtime-kernel-labs` job, alongside the PPPoE proof. Run the same action inside
the runtime-kernel guest:

```bash
./le test qemu run kernel tmp/kernel/build/vmlinuz packages "nftables iproute2 libcap kmod" \
  timeout 1200s command 'tmp/qemu/linux-amd64/le test qemu netns suites plugin'
```

This uses the architecture-qualified guest binaries selected by the existing
`ZE_QEMU_BIN` and `ZE_QEMU_STRIPPED_BIN` overrides or their defaults. The
harness is the guest le that runs the action: `./le test qemu run` builds it
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
| `GOOS == linux`, inside the VM | The option is inert, so the same `.ci` test runs for real against the Linux kernel |

<!-- source: internal/test/runner/record_parse.go -- the needs-linux option -->

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
./le test qemu run kernel <vmlinuz> packages "<packages>" timeout 3600s \
  command '<guest-le-binary> le test qemu all'
```

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
<!-- source: internal/le/test/qemu/run.go -- setupCommand, ensureScratch -->
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
installed, and nothing of le's runs in the guest. The action demands HVF or KVM,
because a lab under TCG measures the emulator. It then:

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
   path) resolves in the guest (spec R-7). The image boots a read-only root,
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
   `tmp/qemu/docker-lab-console-<port>.log`; its first program line is
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
<!-- source: internal/le/test/qemu/dockerlab.go -- runDockerLabHere, runDockerLab, Run.dockerLab, Run.dockerLabPrepare, dockerLabVM.argv, dockerLabGuest.cidataFiles, Run.runHostLe, dockerLabSession.forward, dockerLabLeftoverListing, BuildLinuxZe, refuseUnmountableShare -->
<!-- source: internal/le/test/qemu/cidata.go -- cidataISO -->
<!-- source: internal/le/setup/docker/host/actions.go -- EnsureImage -->
<!-- source: internal/le/job/job.go -- Admission.RunContext -->
<!-- test: internal/le/job/runcontext_test.go TestRunContextEndsTheWaitForASlot, TestRunContextStartsNoChildUnderADoneRoot, TestRunContextStopsARunningChildWithSIGTERM, TestRunContextEndsTheFollowOfASharedRun, TestRunContextNotesAStopRatherThanItsError -->
<!-- test: internal/le/test/qemu/dockerlab_root_test.go TestDockerLabBuildsUnderTheVerbRoot, TestDockerLabBuildsSayTheyWereStopped -->
<!-- test: internal/le/test/qemu/dockerlab_leftover_test.go TestDockerLabReportsWhatTheGuestStillHolds -->
<!-- test: internal/le/test/qemu/dockerlab_test.go TestDockerLabBootsTheDockerHostImage, TestDockerLabCidataPassesTheGuestParser, TestDockerLabHostLeTargetsTheForwardedSocket, TestDockerLabForwardsOneInterruptThenStopsTheVM -->
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

### VPN facilities without module loading

`./le test qemu vpn-kernel-test timeout 30m` uses the native-architecture
docker-host image and the shared hardware-accelerated guest runner. It accepts
no image, kernel or architecture override. Before boot, it checks the emitted
config from the image's runtime-kernel cache resolver against both requirement
manifests.

Guest preparation disables module loading, refuses an already-loaded module,
and removes `/sbin/modprobe` in the disposable snapshot before Docker starts.
It does not modify the cached image or the parent host. The inner
`test deployment vpn-kernel-test` runs the five ESP combinations against
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
`run`, `docker-lab`, `installer`, hugepages, MPLS boot and crash capture,
the `installer` web forward, and the crash-capture HTTP forward and monitor.
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
`installer`, hugepages, crash-capture and MPLS boot VMs carry neither
mechanism, so a le killed outright leaves them running on their port with no
lock. The second check covers them: it binds the port on `127.0.0.1` and on the
wildcard, IPv4, with `SO_REUSEADDR` cleared, and so finds a port another
program holds on any address, including a VM whose le died and took its lock
with it. With the option set, as Go sets it by default, macOS lets the probe
bind beside a listening holder.

When every port in the band fails a check, the error names the fixed-port
setting of each action that reaches it (`ze.qemu.ssh.port` for `run`,
`docker-lab` and MPLS boot, `ze.install.ssh.port` for `installer`,
`ze.vpp.hp.ssh.port` for hugepages; crash capture has none), and `lsof +D
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
crash capture and the MPLS boot start outside `Run` carry neither mechanism.
<!-- source: internal/le/test/qemu/vmrecord.go -- recordVM, reapOrphanVMs, reapOne, reapUnreadable, removeStaleStaged -->
<!-- source: internal/le/test/qemu/run.go -- terminalWriter, runHostCommand -->
<!-- source: internal/le/test/qemu/run_procattr_linux.go -- childProcAttr -->
<!-- test: internal/le/test/qemu/vmrecord_test.go TestReapStopsOnlyAQEMUWhoseLeIsGone, TestReapKeepsARecordItCannotJudge, TestReapKeepsAYoungUnreadableRecord, TestReapRemovesAnOldStagedRecord, TestRecordVMLeavesOnlyTheRecord, TestExecutePlanRecordsTheVMWhileItRuns -->
<!-- source: internal/le/test/qemu/run_guestcommand.go -- guestCommandBody, guestInterruptRemote, runGuestCommand, stopGuestCommand, interruptGuest -->
<!-- source: internal/le/test/qemu/run.go -- commandSpec.OwnGroup, notifyInterrupts, runHostCommand -->
<!-- source: internal/le/test/qemu/run_exec.go -- executePlan, sshSession -->
<!-- test: internal/le/test/qemu/run_guestcommand_test.go TestGuestCommandInterruptReachesTheGuestBeforeTheVMStops, TestGuestCommandTeardownPastTheGraceStopsTheVM, TestGuestCommandSecondInterruptStopsTheVM, TestGuestCommandUndeliveredInterruptSaysSo, TestGuestCommandLineKeepsTheCommandsStatus, TestGuestCommandTimeoutForwardsTheInterrupt, TestGuestCommandSSHFailureAfterTheForwardSaysSSH, TestHostCommandOutlivesADeadTerminal, TestHostCommandOwnGroupLeadsItsOwnProcessGroup -->
<!-- test: internal/le/test/qemu/run_guestcommand_linux_test.go TestGuestInterruptLineSignalsTheRecordedGroup, TestChildProcAttrDiesWithLe -->

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
| `//go:build linux` | Test imports linux-only types but needs no kernel capabilities | `host/cpu_linux_test.go` |
| `//go:build integration && linux` | Test needs root, devices, namespaces, ioctls | `l2tp/bridge_integration_linux_test.go` |
| `//go:build docker` | Test starts a real server container through `labtest.Start`, on the host's Docker; never together with `integration`, because the guest has no Docker ("The docker population") | `interoplab/labtest/labtest_docker_test.go` |

Tests tagged `integration && linux` run through the applicable `./le test qemu`
action, which supplies `-tags integration`. Tests tagged only `linux` also run
in native unit groups on a Linux host.

Kernel-touching tests use `integration && linux`, including tests carrying
an `RFC requirement:` tag. The native discrimination recorder tries the
host's actual build tags first, then a Linux guest with `integration`.
A guest observation requires Ze's runtime kernel through the `kernel`
keyword; a host observation rejects that keyword. See
[RFC discrimination placement](../../contributing/rfc-conformance-gates.md).

For tests needing only their own network, a fixed port or an address on `lo`,
`userns.Enter` (`internal/test/userns`) re-executes the test in a new user,
network and mount namespace, where the test is root over namespaces of its
own, and skips only when the kernel refuses them; the skip names
`./le setup install` and its `userns-unrestricted` check. `Enter` is the first
statement of a top-level test, and such a test never calls `t.Parallel`,
because the child re-runs that one named test in a fresh process. A well-known
port bound there never collides with a ze running on the host or with a
parallel run of the same package.

The child runs with `-test.run '^<Name>$' -test.v -test.count=1`, and the
parent reads the child's own top-level result line, `--- PASS: <Name> (` or
`--- SKIP: <Name> (` at the start of a line. A child that exits 0 with neither
line ran no test (a name the binary does not hold selects nothing and still
exits 0), and the parent then fails rather than pass on nothing. A subtest's
result line is indented and does not count: a skipped subtest neither skips nor
passes the test it belongs to.

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

The
BFD wire tests (`internal/component/bfd/rfc5881_wire_port_linux_test.go`,
`internal/component/bfd/engine/rfc5881_ttl_wire_linux_test.go`) use it.
`Enter` brings up the literal kernel-created `lo` inside that new namespace.
This is fixture topology, not an operator-selected logical interface, so the
direct lookup in `loopbackUp` is an explicit interface-resolution gate exemption.
<!-- source: internal/test/userns/userns_linux.go -- Enter, refusal, loopbackUp -->

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
| Route watch integration | `internal/core/routewatch/integration_linux_test.go` |
| PTY/termios integration | `internal/component/config/system/console_integration_linux_test.go` |
| QEMU runner | `internal/le/test/qemu/run.go` |

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| "Needs real hardware, skipping test" | Use the virtual substitute in the table above |
| `//go:build linux` on a kernel-touching test | Use `//go:build integration && linux`, including RFC-tagged tests |
| A new Linux package absent from `integrationPackages` | The test compiles and never runs. Add it to `internal/le/test/qemu/alltests.go` |
| `t.Fatal` for a missing capability | Use `t.Skip`, so the file stays portable |
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

## The docker population

The guest has no Docker, so a package test that starts a real server container
through `labtest.Start` carries the build tag `docker` and never `integration`.
`./le test integration docker-peers` runs that population on the host: it
derives every package under `internal/` and `cmd/` holding a test file that needs
the `docker` tag from the tree, so a package added later runs with no edit, and it sets
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
900 s bound. That kill skips every `labtest` `t.Cleanup`, so a signalled run
then makes the call `./le test integration clean` makes, one non-dry orphan
sweep on a context the signal did not cancel, bounded at 60 s, and prints its
summary: the killed test binaries owned the containers, so the sweep proves
them gone and removes them in the same invocation. Reaping `go test` does not
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
<!-- source: internal/le/gaterun/gaterun.go -- StreamContext, RunContext -->
<!-- source: internal/le/gaterun/group.go -- ProcessGroup.AwaitExit -->
<!-- test: internal/le/test/integration/dockerpeers_signal_test.go TestSignalSweepWaitsForTheGroup, TestSignalSweepNamesMembersLeft -->
<!-- source: internal/le/test/buildtag/buildtag.go -- TestFile.Needs, TestFile.Names, Packages -->
<!-- source: internal/le/test/qemu/integration_coverage_test.go -- TestDockerTagNeverJoinsIntegration, dockerIntegrationOverlap -->
<!-- source: internal/le/test/integration/gates.go -- Table, the docker-peers and live-rpki rows -->
