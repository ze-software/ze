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

# all-tests runs INSIDE the guest. le test qemu run boots the guest and carries it in.
./le --name qemu-current test qemu run kernel "<kernel-directory>/vmlinuz" packages "iproute2 libcap" \
  command "./le test qemu all-tests"
```

Replace `<kernel-directory>` with the directory the kernel builder prints.
This example targets an amd64 guest. Use the matching architecture for other
guests. `build-artifacts host` builds only `ze-host`; it does not stage a kernel.

The Go release the guest unpacks is DERIVED from the `go` directive of `go.mod`
(`goversion.DeclaredRelease`), so the guest and the host compile with one
toolchain. It was a constant in `internal/le/test/qemu/run.go` until 2026-09-05, went
a minor behind, and every guest `go` command then downloaded a second toolchain
until the run died on `error: command ssh exceeded its deadline`.

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

First run takes ~1 min to download Alpine ISO and Go toolchain. Both are
cached in `tmp/qemu/` and reused on subsequent runs. A typical run boots
the VM in ~15s and runs tests in ~30-60s.

### The two entry points

Both entry points run one VM for the whole population, never one VM per test.

| Command | Population |
|---------|------------|
| `./le test qemu netns-test suites <comma-separated-suites>` | The explicit kernel-dependent subset for each selected suite. `plugin` selects only the seven ASPA cases |
| `./le test qemu run ... command "./le test qemu all-tests"` | Every functional suite, the Linux unit pass, the installer phase, and every registered integration package. Four of the suites run in a per-test network namespace, which needs `packages "iproute2 libcap"` |
| `./le test qemu run ... command "./le test qemu all-tests only needs-linux"` | The same suites, each narrowed to the `.ci` tests marked `option=needs-linux`. The unit, installer and integration phases stay whole, and the report names the population it covered |

`all-tests` discovers the suites' tests through the native runner, so a new
`needs-linux` test needs no registration there. `netns-test` uses the explicit
named populations in `netnsSelections`; selecting a suite does not run every
test in its directory.

<!-- source: internal/le/test/qemu/alltests.go -- AllTestsRun.Run, the phase population -->
<!-- source: internal/le/test/qemu/guestlabs.go -- the netns-test suite selection -->

### `all-tests` is a GUEST action, and four things must be true before it runs

`le test qemu all-tests` refuses to start outside the VM: `internal/le/test/qemu/actions.go`
registers it with "This action runs inside the VM. Its caller is the command
value passed to the host qemu run action." Typed on the host it answers
`qemu: the repository is not mounted: /workspace` and nothing runs. The host
driver is always `./le test qemu run ... command "<the guest command>"`.

Four preconditions, each of which fails with a message that does NOT name the
precondition. Measured 2026-09-04 and 2026-09-05, seven guest boots to establish:

| Precondition | What its absence looks like |
|--------------|-----------------------------|
| `packages iproute2` | `ZE-OBSERVER-FAIL: ... ip: invalid argument 'replace' to 'ip'`. BusyBox `ip` has no `neigh replace`, so an observer that programs a neighbour dies in test setup, before any assertion |
| The three binaries, under canonical names | `qemu: bin/ze-stripped is missing or not executable -- cross-compile it on the host first`. `le test qemu run` shares the checkout, where the cross-built artifacts carry `-linux-arm64` suffixes, so name them with `ZE_BIN`, `ZE_STRIPPED_BIN` and `LE_TEST_BIN`. `shim()` symlinks them to `ze`, `ze-stripped` and `le-test` because the tools dispatch on basename |
| The three binaries, STATICALLY linked | `qemu: bin/ze: is dynamically linked against /lib64/ld-linux-x86-64.so.2, which the musl guest does not have`, and nothing runs. `runnableInGuest` (`alltests.go`) reads each binary's PT_INTERP header before the first child. Until 2026-09-05 `verify()` only stat'd the file, and the same cause surfaced as 326 identical per-test failures (`start ze: fork/exec ... : no such file or directory`), because the kernel answers ENOENT for the LOADER while naming the BINARY. Build with `CGO_ENABLED=0`, which is what the native toolchain sets (`internal/le/gotoolchain`) |
| `packages libcap` | `qemu: setcap is not on the guest PATH, so the per-test network namespace suites cannot hold their capabilities`. The four routed suites run ze as an ordinary user inside a fresh namespace, and `setcap` is what gives that user CAP_NET_ADMIN |
| The `bgp` verb before the suite | The `ze plugin` help text, and exit 1. `le-test plugin <name>` is read as the `ze plugin` command; the suite form is `le-test bgp <suite> <name>`, which is what `vmSuites` passes (`alltests.go`) |

`le test qemu run` installs only `git curl musl-dev` beyond the base image
(`internal/le/test/qemu/run.go`), so anything else a test shells out to has to be
named in `packages`. `iproute2` and `libcap` are both preconditions of a whole
run, and the refusal for each one names it.

`coreutils` was a fifth precondition until 2026-09-05. The suite wrapper passed
GNU `timeout --kill-after=15s`, which the guest's BusyBox `timeout` answers with
`unrecognized option` and exit 1, so every suite printed the usage text under its
own header and ran no test. The wrapper now passes `-k 15`, which BusyBox and GNU
coreutils both accept (`killAfterFlag`, `internal/le/test/qemu/alltests.go`). A run
with `packages "iproute2"` alone reached ten suites and several thousand `.ci`
tests on 2026-09-05, with no BusyBox usage text anywhere in its log.

### Running ONE `.ci` test in a throwaway guest

The tight loop for a single Linux-only test, about 30 seconds of test after the
boot. It does the binary shim by hand because that is `all-tests`'s job and this
path skips `all-tests`:

```bash
./le test qemu run kernel tmp/kernel/build/vmlinuz packages "iproute2" \
  command "mkdir -p /tmp/zb \
    && ln -sf /workspace/bin/ze-linux-arm64 /tmp/zb/ze \
    && ln -sf /workspace/bin/le-test-linux-arm64 /tmp/zb/le-test \
    && ln -sf /workspace/bin/ze-stripped-linux-arm64 /tmp/zb/ze-stripped \
    && cd /workspace && PATH=/tmp/zb:\$PATH le-test bgp plugin <test-name>"
```

Wrap it in `./le job run label <name> quiet command ...` so it takes its turn
with the other sessions on the machine.

<!-- source: internal/le/test/qemu/actions.go -- all-tests is registered as a guest action -->
<!-- source: internal/le/test/qemu/alltests.go -- suiteCommand, shim, the ZE_*_BIN knobs -->
<!-- source: internal/le/test/qemu/run.go -- runBootstrapCommand and the package list -->

### Four suites do not run in the guest root namespace

`all-tests` runs inside an SSH session, and that session's transport lives in
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
with a non-root `ZE_TEST_UID`. `all-tests` sets both for a routed suite, copies
`ze` and `ze-stripped` to the guest's own tmpfs (a 9p mount carries no extended
attribute, so a `setcap` on `/workspace` would buy nothing), and gives the
dropped user a state directory to write in.

Routing a suite also un-skips its `option=netns-link` tests. Outside this mode
the runner SKIPS them (`applyNetnsLinkGate`, `internal/test/runner/caps.go`),
so eight `test/ospf` and three `test/ospfv3` tests ran in no VM phase at all.
The `plugin` suite stays in the guest root namespace under `all-tests`; its
`netns-link` cases are therefore skipped there. The separate
`netns-test suites plugin` selection runs the seven ASPA validation and policy
cases with their declared `eth1` address, `10.0.0.254/24`. Their received
NEXT_HOP, `10.0.0.1`, is then nonlocal, on-link and syntactically valid.
The selector does not include other plugin namespace cases or move the whole
plugin suite into namespaces.

`./le test qemu netns-test suites <names>` is the same launcher over a named subset,
and it also asserts the guest root nft ruleset is unchanged by the run. It is
the tight loop; `all-tests` retains the whole-suite namespace table above.

The seven-case ASPA subset is scheduled in `qemu-nightly.yml`'s
`runtime-kernel-labs` job, alongside the PPPoE proof. Run the same action inside
the runtime-kernel guest:

```bash
./le test qemu run kernel tmp/kernel/build/vmlinuz packages "nftables iproute2 libcap kmod" \
  timeout 1200s command 'bin/ze-le-linux-amd64 le test qemu netns-test suites plugin'
```

This uses the architecture-qualified guest binaries selected by the existing
`ZE_QEMU_BIN`, `ZE_QEMU_STRIPPED_BIN` and `LE_QEMU_TEST_BIN` overrides or their
defaults. The harness default is `bin/le-test-linux-<guest arch>`, derived from
`QEMU_GOARCH` or the host architecture by `qemuTestBin`, its only reader. The
retired spelling `ZE_QEMU_TEST_BIN` is still read, with a deprecation line. A native prerequisite skip is not evidence for this subset.

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
disagrees with `internal/appliance/kernel.version`.

The native host action cross-compiles a Linux `cmd/ze` personality with the
`ze_le` tag before boot. That guest binary runs the selected action. The full
Linux suite is:

```text
./le test qemu run kernel <vmlinuz> packages "<packages>" timeout 3600s \
  command '<guest-le-binary> le test qemu all-tests'
```

The extra `le` after the guest binary is intentional. The cross-compiled file
has an architecture-qualified basename, so `cmd/ze` treats it as a Ze
personality; the `ze_le` crossing selects the same `qemu all-tests` action the
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
<!-- source: internal/core/tmplink/tmplink.go -- Shares -->
<!-- source: internal/le/test/qemu/run.go -- Run.scratchShares -->
<!-- source: internal/appliance/kernelbuilder/qemu.go -- guestSetup -->

```text
host                                      QEMU Alpine VM
────                                      ──────────────
./le test qemu run kernel <vmlinuz> ...
  ├─ cross-compile cmd/ze and the native guest runner
  └─ boot the named kernel and run ONE command over SSH
       ├─ boot the named kernel             → verify uname -r
       ├─ mount checkout and tmp target      → /workspace
       ├─ install declared packages
       └─ SSH native guest command           → le test qemu all-tests   (in the guest)
                                                ├─ per-test namespace preparation
                                                ├─ functional suites
                                                ├─ Linux unit pass
                                                ├─ installer initrd tests
                                                └─ integration-tagged tests
```

`all-tests` is on the GUEST side of that diagram. It is an action of the same
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

## Appliance first-boot storage import

The host-side `install-test` action boots a fresh appliance through the HTTP
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
./le --name storage-proof test qemu install-test
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
| `//go:build integration && linux` | Test needs root, devices, namespaces, ioctls | `iface/config_integration_linux_test.go` |

Tests tagged `integration && linux` run through the applicable `./le test qemu`
action, which supplies `-tags integration`. Tests tagged only `linux` also run
in native unit groups on a Linux host.

One case takes the first tag for a test that does need a capability, and it is
the only one: a unit carrying an `RFC requirement:` tag. `./le rfc
discriminate-record` runs the tagged unit with an empty build-tag set, so a
unit behind `integration` matches nothing, `go test` exits 0, and no break can
ever be observed to redden it
(`plan/journal/gate-excludes-part-of-its-population.md`). Such a unit carries
bare `linux`, and it calls `t.Skip` when the capability is absent, so it stays
silent in the merge gate and runs for real in a privileged guest or container.
`internal/core/network/ttl_gtsm_linux_test.go` and
`internal/component/gtsm/gtsm_rfc5082_linux_test.go` are the two files that do
this, and both say so in their headers.

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

`le-test fail-syscall` supplies one. It installs a classic seccomp filter that
answers a chosen errno for a chosen syscall, then `execve`s the command, so no
tracer is in the path and the refused call is charged to the daemon's own CPU
time.

```
cmd=background:seq=1:exec=le-test fail-syscall syscall recvfrom errno ENETDOWN length 1500 -- ze -:stdin=config
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
  `le-test fail-syscall ... -- ze -` and gives it a stable per-daemon config
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
the Alpine QEMU VM has no Docker. Each such lab therefore ships two native
actions.

| Lab | Docker action | QEMU action | Native producer |
|-----|---------------|-------------|-----------------|
| L2TP (Ze LNS against xl2tpd) | `./le test deployment docker-l2tp-ppp-test` | `./le test deployment gokrazy-l2tp-ppp-test` | `internal/le/test/deployment` |
| PPPoE (Ze client against accel-ppp) | `./le test deployment docker-pppoe-accel-test` | `./le test qemu pppoe-accel-test` | `internal/le/test/qemu/pppoe_accel_linux.go` |
| VRRP (Ze against keepalived) | `./le test integration interop`, scenario `vrrp-mastership-keepalived` | `./le test qemu vrrp-keepalived-test` | `internal/le/test/qemu/vrrp_keepalived_linux.go` |
| MOBIKE (Ze initiator and responder against strongSwan) | `./le test integration interop-ipsec`, scenarios `mobike-initiator` and `mobike-responder` | `./le test qemu ipsec-mobike-test kernel <vmlinuz>` | `internal/le/interoplab/ipsec/mobike_netns_linux.go` |

<!-- source: internal/le/test/deployment/actions.go -- gokrazy-l2tp-ppp-test, docker-l2tp-ppp-test, docker-pppoe-accel-test -->
<!-- source: internal/le/test/qemu/actions.go -- pppoe-accel-test, vrrp-keepalived-test -->

The MOBIKE action is a manual runtime-kernel proof, not a merge gate or a
scheduled job. It runs both existing movement scenarios, without a selector:

```text
./le --name mobike-proof test qemu ipsec-mobike-test kernel <runtime-vmlinuz> timeout 1200s
```

The host builds a static Linux daemon and the native guest runner through build
admission, with tags derived from `feature-gates.txt`. Both artifacts live under
the invoking session's scratch directory until the VM run ends; host tools are
not cross-compiled. The existing QEMU harness boots the supplied runtime kernel,
checks its release, installs Alpine's `strongswan`, `iproute2`, `iputils` and
`util-linux` packages, and invokes
`le test deployment ipsec-mobike-test daemon <guest-daemon-path>` in the guest.
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

`./le test qemu vrrp-keepalived-test` runs four scenarios, selectable with
`scenarios=<csv>`.

| Scenario | What it proves |
|----------|----------------|
| `QS-1` | Ze wins the election against keepalived and the advertisement fields match the RFC. |
| `QS-2` | Ze dies, keepalived promotes within the master-down band and sends a gratuitous ARP; Ze returns and preempts. |
| `QS-3` | The skew-time term of the master-down interval is honored. |
| `tracked-uplink-hands-the-vip-to-keepalived` | Ze at priority 200 tracks a veth worth a decrement of 150. The veth goes down, Ze advertises 50, and keepalived at 100 takes the VIP; the veth returns and Ze preempts. keepalived's own notify script is the assertion, so ANOTHER implementation acts on the decremented priority. |

The first three names predate the rule that an interop scenario is NAMED rather
than numbered (`ai/rules/interop-and-goal-validation.md`). Renaming them is not
the tracking scenario's work, and the fourth does not copy the pattern.

<!-- source: internal/le/test/qemu/guestlabs.go -- vrrpScenarioNames -->
<!-- source: internal/le/test/qemu/vrrp_keepalived_linux.go -- runTrackedUplink -->

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
| `//go:build linux` on a test that needs root | Use `//go:build integration && linux`, unless the unit carries an `RFC requirement:` tag: see Build Tags |
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
