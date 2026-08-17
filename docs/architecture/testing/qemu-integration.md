# QEMU Integration Testing

Ze runs on Linux (gokrazy appliances, Debian/Ubuntu servers). Code that uses
Linux-only syscalls (termios, netlink, nftables, sysctl) cannot be tested on
macOS or in Docker Desktop. Ze uses a QEMU Alpine Linux VM to run these tests
with full kernel capabilities.

## Quick Start

```bash
# Run all QEMU integration tests (first run downloads Alpine ISO + Go)
make ze-qemu-integration-test

# Run L2TP PPP tests (requires gokrazy kernel)
make ze-qemu-l2tp-ppp-test
```

Prerequisites: `qemu` (`brew install qemu` on macOS).

First run takes ~1 min to download Alpine ISO and Go toolchain. Both are
cached in `tmp/qemu/` and reused on subsequent runs. A typical run boots
the VM in ~15s and runs tests in ~30-60s.

## How It Works

`scripts/evidence/qemu-run.py` boots an Alpine Linux live ISO in QEMU,
mounts the repository via virtio-9p, installs Go, and runs `go test` with
`-tags integration` inside the VM via SSH.

A share of the repository hands the guest a symlink as a symlink. So when
`tmp/` is a symlink to an out-of-tree scratch directory
(`scripts/dev/ensure-links.py`), `qemu-run.py` adds a second 9p share of the
link's target and mounts it at the path the link names. Without it
`/workspace/tmp` dangles in the guest, and every path below it fails to
resolve: the session's own binaries (`tmp/session/<YYYY-MM-DD>-<id>/bin/ze`,
`mk/session.mk`) most of all. `scratch_share` decides this and
`qemu-run.py --selftest` covers both layouts.

```
macOS                          QEMU Alpine VM
─────                          ──────────────
make ze-qemu-integration-test
  └─ qemu-run.py
       ├─ boots Alpine ISO      → login as root
       ├─ virtio-9p mount       → /workspace (repo)
       ├─ virtio-9p mount       → the tmp/ target, when tmp/ is a symlink
       ├─ SSH tunnel (port 2222)
       ├─ installs Go + packages
       └─ ssh: CGO_ENABLED=0 go test -tags integration ...
                                   ├─ full root
                                   ├─ /dev/ptmx (PTY)
                                   ├─ CAP_NET_ADMIN
                                   ├─ nftables, netns
                                   └─ kernel modules
```

## Writing Integration Tests

### Build Tags

Two patterns, choose based on what the test needs:

| Build tag | Use when | Example |
|-----------|----------|---------|
| `//go:build linux` | Test imports linux-only types but needs no kernel capabilities | `host/cpu_linux_test.go` |
| `//go:build integration && linux` | Test needs root, devices, namespaces, ioctls | `iface/config_integration_linux_test.go` |

Tests tagged `integration && linux` only run via `make ze-qemu-integration-test`
(which passes `-tags integration`). Tests tagged just `linux` also run during
normal `go test` on any Linux host.

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

Add your package to the `--run` argument in the Makefile:

```makefile
ze-qemu-integration-test:
    python3 scripts/evidence/qemu-run.py \
        --packages "nftables iproute2 iputils-ping kmod iptables" \
        --run 'CGO_ENABLED=0 go test -tags integration -count=1 -timeout 120s \
            ./internal/component/iface/... \
            ./your/new/package/... \           # add here
            ...'
```

If your tests need Alpine packages not already listed, add them to `--packages`.

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

**Tests time out:** Default timeout is 120s. For long-running tests, increase
the `-timeout` flag in the Makefile `--run` argument.

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
