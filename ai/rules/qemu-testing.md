# QEMU Integration Testing

**BLOCKING:** Linux-only code (`//go:build linux`) MUST ship with integration
tests that run in the QEMU Alpine VM. "Needs real hardware" is never a valid
reason to skip tests. Virtual substitutes exist for every kernel feature ze uses.

## When QEMU Tests Are Required

| You wrote | You need |
|-----------|----------|
| `//go:build linux` source file | Corresponding `*_integration_linux_test.go` |
| termios / serial port code | PTY-pair test (`creack/pty`, already vendored) |
| netlink / interface code | Network namespace + veth/dummy test |
| nftables / firewall code | Network namespace + nft test |
| sysctl / kernel tuning | procfs read test (may need `t.Skip` for write) |
| Any new linux-only package | Package added to `ze-qemu-integration-test` Makefile target |

## How to Write a QEMU Integration Test

### 1. File naming and build tags

```go
//go:build integration && linux

package mypkg  // internal package, not _test, to access unexported functions
```

File name: `<feature>_integration_linux_test.go`

Two build tag patterns exist in the codebase:

| Tag | When to use | Runs during |
|-----|------------|-------------|
| `//go:build linux` | Test only needs Linux types (imports linux-only packages) but no kernel capabilities | `go test` on any Linux host, including QEMU |
| `//go:build integration && linux` | Test needs kernel capabilities (root, /dev, netns, ioctl) | `make ze-qemu-integration-test` only (passes `-tags integration`) |

Use `integration && linux` for anything that touches the kernel. Use bare
`linux` only when the test imports linux-only types but makes no syscalls.

### 2. Virtual substitutes for hardware

Never require physical hardware. The QEMU VM provides kernel features; use
virtual devices.

| Hardware | Virtual substitute | Example |
|----------|--------------------|---------|
| Serial port (`/dev/ttyS*`) | PTY pair via `creack/pty` | `master, slave, _ := pty.Open()` then `applyTermios(slave.Name(), 9600)` |
| Network interface | `veth` pair or `dummy` in a netns | `ip link add ze0 type dummy` |
| Firewall table | `nftables` in a netns | `nft add table ip ze_test` |
| Kernel route | Netlink in a netns | `route.Add(...)` |
| Block device | Loop device on tmpfs file | `losetup` |

### 3. Graceful skip when capabilities are missing

The same test file may run in environments with different capabilities. Use
`t.Skip`, not `t.Fatal`, when a prerequisite is absent:

```go
master, slave, err := pty.Open()
if err != nil {
    t.Skipf("cannot open pty: %v", err)
}
```

```go
newNS, err := netns.NewNamed(name)
if err != nil {
    t.Skipf("requires CAP_NET_ADMIN: %v", err)
}
```

### 4. Register the package in the Makefile

Add your package to the `--run` argument of `ze-qemu-integration-test`:

```makefile
ze-qemu-integration-test:
    python3 scripts/evidence/qemu-run.py \
        --packages "nftables iproute2 iputils-ping kmod iptables" \
        --run 'go test -tags integration -count=1 -timeout 120s \
            ./internal/component/iface/... \
            ./internal/component/config/system/... \   # <-- add here
            ...'
```

If your tests need extra Alpine packages (e.g., `strace`, `util-linux`), add
them to `--packages`.

## What the QEMU VM Provides

Alpine Linux live system (no systemd) with:

- Root access (all capabilities)
- `/dev/ptmx` for PTY pairs
- Network namespaces (`ip netns`)
- Kernel modules: nftables, l2tp, ppp (loaded at boot)
- Go toolchain (downloaded and cached in `tmp/qemu/`)
- Repo mounted read-write via virtio-9p at `/workspace`
- No systemd, no getty, no desktop (Alpine minimal)

## What the QEMU VM Does NOT Provide

- systemd (use Alpine's OpenRC or skip systemd-specific tests)
- Physical serial ports (use PTY pairs)
- Multiple physical NICs (use veth pairs)
- GPU or display (tests are headless)
- Persistent state between runs (boots fresh from ISO each time)

## Running QEMU Tests

```bash
make ze-qemu-integration-test          # All integration packages
```

First run downloads Alpine ISO + Go toolchain (~1 min). Subsequent runs
reuse the cache in `tmp/qemu/` (~30s boot + test time).

On macOS: requires `qemu` (`brew install qemu`). Uses HVF acceleration
when available, falls back to TCG (software emulation).

## Reference Implementations

| What | File |
|------|------|
| Network namespace helper | `internal/component/iface/integration_helpers_linux_test.go` |
| Netlink integration test | `internal/plugins/traffic/netlink/integration_linux_test.go` |
| nftables integration test | `internal/plugins/firewall/nft/integration_linux_test.go` |
| Route watch integration | `internal/core/routewatch/integration_linux_test.go` |
| PTY/termios integration | `internal/component/config/system/console_integration_linux_test.go` |
| QEMU runner script | `scripts/evidence/qemu-run.py` |

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| "Needs real hardware, skipping test" | Use a virtual substitute (see table above) |
| `//go:build linux` on a test that needs root | Use `//go:build integration && linux` |
| Forgetting to add package to Makefile | Test compiles but never runs in CI |
| Using `t.Fatal` for missing capabilities | Use `t.Skip` so the test is portable |
| Hardcoding `/dev/ttyS0` in a test | Use `pty.Open()` for a real PTY pair |
