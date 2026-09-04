# QEMU Integration Testing

Ze runs on Linux (gokrazy appliances, Debian/Ubuntu servers). Code that uses
Linux-only syscalls (termios, netlink, nftables, sysctl) cannot be tested on
macOS or in Docker Desktop. Ze uses a QEMU Alpine Linux VM to run these tests
with full kernel capabilities.

## Quick Start

```bash
# Stage the kernel every QEMU target boots. A hit costs a copy.
./le build-artifacts host

# all-tests runs INSIDE the guest. le qemu run boots the guest and carries it in.
./le qemu run kernel tmp/kernel/build/vmlinuz packages "coreutils iproute2" \
  command "./le qemu all-tests"
```

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
| `./le qemu netns-test suites <comma-separated-suites>` | The selected kernel-dependent functional suites. A tight iteration loop |
| `./le qemu run ... command "./le qemu all-tests"` | Every functional suite, the Linux unit pass, the installer phase, and every registered integration package |
| `./le qemu run ... command "./le qemu all-tests only needs-linux"` | The same suites, each narrowed to the `.ci` tests marked `option=needs-linux`. The unit, installer and integration phases stay whole, and the report names the population it covered |

Neither entry point needs per-test wiring. The suites are the same ones the
native runner discovers, so the QEMU pass finds a new `needs-linux` test with no
registration.

<!-- source: internal/le/qemu/alltests.go -- AllTestsRun.Run, the phase population -->
<!-- source: internal/le/qemu/guestlabs.go -- the netns-test suite selection -->

### `all-tests` is a GUEST action, and four things must be true before it runs

`le qemu all-tests` refuses to start outside the VM: `internal/le/qemu/actions.go`
registers it with "This action runs inside the VM. Its caller is the command
value passed to the host qemu run action." Typed on the host it answers
`qemu: the repository is not mounted: /workspace` and nothing runs. The host
driver is always `./le qemu run ... command "<the guest command>"`.

Four preconditions, each of which fails with a message that does NOT name the
precondition. Measured 2026-09-04, six guest boots to establish:

| Precondition | What its absence looks like |
|--------------|-----------------------------|
| `packages coreutils` | `timeout: unrecognized option: kill-after=15s` and BusyBox usage, once per suite. EVERY functional suite reports failed to launch and no `.ci` test runs. `suiteCommand` (`internal/le/qemu/alltests.go`) passes GNU `timeout --kill-after=15s`, and Alpine ships BusyBox `timeout` |
| `packages iproute2` | `ZE-OBSERVER-FAIL: ... ip: invalid argument 'replace' to 'ip'`. BusyBox `ip` has no `neigh replace`, so an observer that programs a neighbour dies in test setup, before any assertion |
| The three binaries, under canonical names | `qemu: bin/ze-stripped is missing or not executable -- cross-compile it on the host first`. `le qemu run` shares the checkout, where the cross-built artifacts carry `-linux-arm64` suffixes, so name them with `ZE_BIN`, `ZE_STRIPPED_BIN` and `ZE_TEST_BIN`. `shim()` symlinks them to `ze`, `ze-stripped` and `ze-test` because the tools dispatch on basename |
| The `bgp` verb before the suite | The `ze plugin` help text, and exit 1. `ze-test plugin <name>` is read as the `ze plugin` command; the suite form is `ze-test bgp <suite> <name>`, which is what `vmSuites` passes (`alltests.go`) |

`le qemu run` installs only `git curl musl-dev` beyond the base image
(`internal/le/qemu/run.go`), so anything else a test shells out to has to be
named in `packages`.

### Running ONE `.ci` test in a throwaway guest

The tight loop for a single Linux-only test, about 30 seconds of test after the
boot. It does the binary shim by hand because that is `all-tests`'s job and this
path skips `all-tests`:

```bash
./le qemu run kernel tmp/kernel/build/vmlinuz packages "coreutils iproute2" \
  command "mkdir -p /tmp/zb \
    && ln -sf /workspace/bin/ze-linux-arm64 /tmp/zb/ze \
    && ln -sf /workspace/bin/ze-test-linux-arm64 /tmp/zb/ze-test \
    && ln -sf /workspace/bin/ze-stripped-linux-arm64 /tmp/zb/ze-stripped \
    && cd /workspace && PATH=/tmp/zb:\$PATH ze-test bgp plugin <test-name>"
```

Wrap it in `./le job run label <name> quiet command ...` so it takes its turn
with the other sessions on the machine.

<!-- source: internal/le/qemu/actions.go -- all-tests is registered as a guest action -->
<!-- source: internal/le/qemu/alltests.go -- suiteCommand, shim, the ZE_*_BIN knobs -->
<!-- source: internal/le/qemu/run.go -- runBootstrapCommand and the package list -->

### How `option=needs-linux` behaves on each host

| Host | Behavior |
|------|----------|
| `GOOS != linux` | The runner sets `SkipReason` and the test reports SKIP, never FAIL. `./le verify worktree` and `./le functional gating` stay green on darwin without running the test |
| `GOOS == linux`, inside the VM | The option is inert, so the same `.ci` test runs for real against the Linux kernel |

<!-- source: internal/test/runner/record_parse.go -- the needs-linux option -->

## Every QEMU target boots the kernel ze ships

The VM runs Alpine userland on **ze's own runtime kernel**, never the kernel on
the Alpine ISO. The host action `./le qemu run` owns the Alpine cache, the QEMU
lifecycle, both 9p shares, bounded SSH waits, package installation, and cleanup.
Its `kernel <path>` parameter supplies the staged runtime kernel, and the guest
release check refuses a boot whose `uname -r` disagrees with
`internal/appliance/kernel.version`.

The native host action cross-compiles a Linux `cmd/ze` personality with the
`ze_le` tag before boot. That guest binary runs the selected action. The full
Linux suite is:

```text
./le qemu run kernel <vmlinuz> packages "<packages>" timeout 3600s \
  command '<guest-le-binary> le qemu all-tests'
```

The extra `le` after the guest binary is intentional. The cross-compiled file
has an architecture-qualified basename, so `cmd/ze` treats it as a Ze
personality; the `ze_le` crossing selects the same `qemu all-tests` action the
standalone root launcher exposes. Guest-side VRRP, PPPoE, network-namespace, and
full-suite work is Go in `internal/le/qemu`, not an interpreted guest driver.

A shared checkout hands the guest a symlink as a symlink. When `tmp/` points
outside the checkout, `Run.scratchShare` adds a second 9p share for that target.
<!-- source: internal/le/qemu/run.go -- Run.scratchShare -->

```text
host                                      QEMU Alpine VM
────                                      ──────────────
./le qemu all-tests
  ├─ cross-compile cmd/ze and the native guest runner
  └─ boot the staged Ze kernel and execute the registered QEMU actions
       ├─ boot staged Ze kernel             → verify uname -r
       ├─ mount checkout and tmp target      → /workspace
       ├─ install declared packages
       └─ SSH native guest command           → le qemu all-tests
                                                ├─ functional suites
                                                ├─ Linux unit pass
                                                ├─ installer initrd tests
                                                └─ integration-tagged tests
```

Staging remains cache-backed. `ze-kernel-vmlinuz-stage` materializes from
`~/.cache/ze` on a hit and builds only when the kernel key changes.

<!-- source: internal/le/qemu/run.go -- Run, Plan -->
<!-- source: internal/le/qemu/actions.go -- Answer -->
<!-- source: internal/le/qemu/alltests.go -- AllTestsRun.Run -->

The installer phase runs `go test -tags 'ze_core ze_installer'` over
`./internal/install/...`. No other phase compiles those files. The tag is a
personality, not a feature the manifest declares. The unit pass therefore
excludes every file behind it. On a host that is not Linux,
`./le test-unit installer` can only type-check them, so this virtual machine
is where they run.

## Writing Integration Tests

### Which test each Linux-only change needs

| You wrote | You need |
|-----------|----------|
| `//go:build linux` source file | A matching `*_integration_linux_test.go` |
| termios / serial port code | A PTY-pair test (`creack/pty`, vendored) |
| netlink / interface code | A network namespace plus veth or dummy test |
| nftables / firewall code | A network namespace plus nft test |
| sysctl / kernel tuning | A procfs read test (a write may need `t.Skip`) |
| Any new Linux-only package | An entry in `integrationPackages`, `internal/le/qemu/alltests.go` |
| A Docker interop lab needing host-kernel features | A native `./le qemu <feature>` action beside the Docker action |

### Build Tags

Two patterns, choose based on what the test needs:

| Build tag | Use when | Example |
|-----------|----------|---------|
| `//go:build linux` | Test imports linux-only types but needs no kernel capabilities | `host/cpu_linux_test.go` |
| `//go:build integration && linux` | Test needs root, devices, namespaces, ioctls | `iface/config_integration_linux_test.go` |

Tests tagged `integration && linux` run through the applicable `./le qemu`
action, which supplies `-tags integration`. Tests tagged only `linux` also run
in native unit groups on a Linux host.

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
`util-linux`, passes it after the `packages` keyword to `./le qemu run`.

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
`internal/le/qemu/alltests.go`. If it needs an Alpine package, add that package
to the owning native action. Do not add a guest script or a second QEMU
lifecycle.

For a distinct guest proof, add one action to `internal/le/qemu/actions.go` and
keep the action callable from Go. The host recipe invokes it through:

```text
./le qemu run kernel <vmlinuz> packages "<packages>" \
  command '<guest-le-binary> le qemu <action>'
```

The host prepares the binary and kernel; the guest action owns only the proof.

## VM Environment

The guest is an Alpine live system with no systemd. It provides:

| Feature | Available | Notes |
|---------|-----------|-------|
| Root access | Yes | All capabilities |
| PTY pairs | Yes | `/dev/ptmx` |
| Network namespaces | Yes | `ip netns` |
| nftables | Yes | Installed through the `packages` keyword of `./le qemu run` |
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
`=y` for this reason.

`Run.setupCommand` still issues `modprobe` for ppp, l2tp and netfilter modules,
each with `|| true`. Those lines are best-effort and load nothing on a
`kernel`-supplied boot. They are not the reason those features work.

<!-- source: internal/le/qemu/run.go -- Run.setupCommand, the best-effort modprobe list -->
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
| L2TP (Ze LNS against xl2tpd) | `./le deployment docker-l2tp-ppp-test` | `./le deployment gokrazy-l2tp-ppp-test` | `internal/le/deployment` |
| PPPoE (Ze client against accel-ppp) | `./le deployment docker-pppoe-accel-test` | `./le qemu pppoe-accel-test` | `internal/le/qemu/pppoe_accel_linux.go` |
| VRRP (Ze against keepalived) | `./le integration interop`, scenario `vrrp-mastership-keepalived` | `./le qemu vrrp-keepalived-test` | `internal/le/qemu/vrrp_keepalived_linux.go` |

<!-- source: internal/le/deployment/actions.go -- gokrazy-l2tp-ppp-test, docker-l2tp-ppp-test, docker-pppoe-accel-test -->
<!-- source: internal/le/qemu/actions.go -- pppoe-accel-test, vrrp-keepalived-test -->

## Reference Implementations

| What | File |
|------|------|
| Network namespace helper | `internal/component/iface/integration_helpers_linux_test.go` |
| Netlink integration test | `internal/plugins/traffic/netlink/integration_linux_test.go` |
| nftables integration test | `internal/plugins/firewall/nft/integration_linux_test.go` |
| Route watch integration | `internal/core/routewatch/integration_linux_test.go` |
| PTY/termios integration | `internal/component/config/system/console_integration_linux_test.go` |
| QEMU runner | `internal/le/qemu/run.go` |

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| "Needs real hardware, skipping test" | Use the virtual substitute in the table above |
| `//go:build linux` on a test that needs root | Use `//go:build integration && linux` |
| A new Linux package absent from `integrationPackages` | The test compiles and never runs. Add it to `internal/le/qemu/alltests.go` |
| `t.Fatal` for a missing capability | Use `t.Skip`, so the file stays portable |
| Hardcoding `/dev/ttyS0` | Use `pty.Open()` for a real PTY pair |
| Reading a QEMU timeout as "TCG is slow" | On Linux, check `kvm-access` first with `./le setup check`. A user outside the `kvm` group makes QEMU refuse to start, which surfaces as a timeout |
| Selecting the accelerator on the existence of `/dev/kvm` | Existence is not access. Probe for read and write, and take an explicit `hvf` branch on darwin |

## Troubleshooting

**VM fails to boot:** Check that `qemu-system-x86_64` (or `qemu-system-aarch64`
on ARM Macs) is installed. On macOS: `brew install qemu`.

**Tests time out:** The default timeout is 120 seconds. Change the timeout in
the owning action under `internal/le/qemu`.

**Package not found in Alpine:** Check the Alpine package name at
`https://pkgs.alpinelinux.org/`. Alpine package names sometimes differ from
Debian/Ubuntu (e.g., `iproute2` not `iproute`).

**Go module download fails:** The VM needs internet access. QEMU's user-mode
networking provides NAT. Check that the host has connectivity.

## Existing Integration Test Packages

The population is `integrationPackages` in `internal/le/qemu/alltests.go`, a
closed list. `TestEveryIntegrationPackageIsNamed` derives every package holding
an `integration`-tagged test file from the tree and fails when one is absent
from that list; `TestEveryNamedIntegrationPackageExists` fails on a named
package that is not in the tree. Read the Go list rather than a copy of it.

<!-- source: internal/le/qemu/alltests.go -- integrationPackages -->
<!-- source: internal/le/qemu/integration_coverage_test.go -- TestEveryIntegrationPackageIsNamed, TestEveryNamedIntegrationPackageExists -->
