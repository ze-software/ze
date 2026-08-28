# QEMU Integration Testing

Ze runs on Linux (gokrazy appliances, Debian/Ubuntu servers). Code that uses
Linux-only syscalls (termios, netlink, nftables, sysctl) cannot be tested on
macOS or in Docker Desktop. Ze uses a QEMU Alpine Linux VM to run these tests
with full kernel capabilities.

## Quick Start

```bash
# Stage the kernel every QEMU target boots. A hit costs a copy.
./le build-artifacts host
./le qemu all-tests
```

Prerequisites: `qemu` (`brew install qemu` on macOS).

First run takes ~1 min to download Alpine ISO and Go toolchain. Both are
cached in `tmp/qemu/` and reused on subsequent runs. A typical run boots
the VM in ~15s and runs tests in ~30-60s.

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

The Alpine VM provides:

| Feature | Available | Notes |
|---------|-----------|-------|
| Root access | Yes | All capabilities |
| PTY pairs | Yes | `/dev/ptmx` |
| Network namespaces | Yes | `ip netns` |
| nftables | Yes | Installed via `--packages` |
| Kernel modules | Yes | ppp, l2tp, nft loaded at boot |
| Go toolchain | Yes | Downloaded and cached |
| systemd | **No** | Alpine uses OpenRC |
| Physical NICs | **No** | Use veth pairs |
| Persistent state | **No** | Boots fresh from ISO each run |

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

| Package | What it tests |
|---------|---------------|
| `internal/component/iface/` | Interface create/delete, config apply, migration, mirroring, monitoring |
| `internal/component/config/system/` | Serial console termios configuration via PTY |
| `internal/core/routewatch/` | Kernel route change notifications |
| `internal/plugins/fib/kernel/` | FIB route installation via netlink |
| `internal/plugins/firewall/nft/` | nftables rule management |
| `internal/plugins/firewall/vpp/` | VPP firewall backend (fakeOps) |
| `internal/plugins/traffic/netlink/` | Traffic shaping via netlink |
