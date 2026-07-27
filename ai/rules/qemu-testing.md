# QEMU Integration Testing

**When:** writing Linux-only code (`//go:build linux`) that must ship with QEMU integration tests
**Severity:** blocking

## Directives

Linux-only code (`//go:build linux`) MUST ship with integration
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
| Docker interop lab needing host-kernel features (l2tp, pppoe, ...) | A netns `effective-<feature>.py` + `ze-qemu-<feature>-test` target -- the Docker lab cannot run in the Alpine VM (see "Interop Labs" below) |

## Linux-only functional (`.ci`) tests run via QEMU, never natively

A functional `.ci` test that boots a daemon (or runs `ze`) which
exercises a real Linux kernel feature -- netlink interface/VLAN/veth creation,
nftables, kernel sockets, L2TP/PPPoE kernel modules -- MUST be marked
`option=needs-linux`. Such a test cannot pass natively on darwin and must be
validated automatically inside the QEMU Alpine VM instead.

```
option=needs-linux
```

How it behaves (`internal/test/runner/record_parse.go`, the `needs-linux`
option):

- On a non-Linux host (`GOOS != linux`): the runner sets `SkipReason` and the
  test reports **SKIP**, never FAIL. Native `make ze-verify` / `make
  ze-functional-test` on darwin stays green without running the unsupported
  test.
- Inside the QEMU Alpine VM (`GOOS == linux`): the directive is inert, so the
  same `.ci` test runs for real against the Linux kernel.

Two QEMU entry points run these tests, both **one VM for all of them** (never
one VM per test):

- `make ze-qemu-needs-linux-test` -- the tight loop. Sets `ZE_QEMU_LINUX_ONLY=1`,
  which flips the runner to skip every test that is NOT `needs-linux`, so the VM
  spends its whole boot only on the Linux-only surface. Use this while iterating
  on a Linux-only feature.
- `make ze-qemu-all-test` -- the full pass. Runs every functional suite in the
  VM, so `needs-linux` tests are exercised alongside everything else.

No per-test wiring is needed for either -- the suites are the same as the native
runner, so the QEMU pass discovers `needs-linux` tests automatically.

Decision rule:

| The `.ci` test ... | Use |
|--------------------|-----|
| Only validates config (`ze config validate -`), parses, or runs offline `ze show`/`ze env` | nothing (runs natively on every OS) |
| Boots a daemon that **applies** Linux-only config (interface/VLAN, firewall, L2TP kernel) | `option=needs-linux` |
| Same, AND needs privileged network configuration (creates interfaces, brings links up, netlink) | `option=needs-linux:caps=net-admin` |
| Same, AND opens a raw/packet socket (`resolve ping`, traceroute: `net.ListenPacket("ip4:icmp", ...)`) | `option=needs-linux:caps=net-raw` |
| Same, AND loads eBPF | `option=needs-linux:caps=bpf` |
| Needs to skip only on a specific non-Linux OS for an unrelated reason | `option=skip-os:value=darwin` |
| Needs an OPTIONAL heavyweight artifact a checkout does not carry (the appliance module cache: `make ze-gokrazy-deps`) | `option=needs-path:value=<repo-rel>:hint=<cmd>` |

**`skip-os:value=darwin` is NOT a substitute for `caps=`.** It hides a test from
macOS and therefore RUNS it, unprivileged, on the Linux CI runner -- which is
exactly where it cannot pass. `test/plugin/resolve-ping.ci` carried a bare
`skip-os` and failed every CI run with `resolve ping status=error`, because
`doPingCtx` (`internal/component/ping/cmd/ping.go`) needs CAP_NET_RAW. If the
reason a test cannot run on macOS is a capability, declare the capability.

**`caps=net-admin` exists because Linux alone is not the requirement.** On an
unprivileged Linux host (CI runners, most dev boxes, any rootless container) a
test that applies interface config does NOT fail cleanly. The interface plugin
fails its stage-2 (configure) handshake with `operation not permitted` and the
DAEMON does exit 1 -- verified 2026-07-25 in QEMU as an unprivileged user, so do
not read this as the daemon hanging. The TEST hangs, to the suite timeout,
because its check peer goes on waiting for a BGP session the exited daemon will
never open. Seven `test/reload/` tests spent their life in exactly that state,
mis-recorded as "load-sensitive". The gate reads `CapEff` from
`/proc/self/status` (`internal/test/runner/caps_linux.go`), not uid 0: a setcap'd
binary holds the capability without being root, and a restricted container can be
root without it.

A `caps=` typo is a parse error on every host, so it cannot silently disable the
gate.

**Know what you are trading.** A `caps=net-admin` test does NOT run in the merge
gate: `make ze-verify` runs unprivileged, so the marker turns an opaque hang into
an honest skip there. Its home is `.github/workflows/qemu-nightly.yml`, which
runs `ze-qemu-needs-linux-test` on a schedule -- so the marker RELOCATES the
coverage rather than deleting it. `TestCapabilityGatedTestsHaveAQemuHome`
(`scripts/dev/github_workflows_test.go`) fails if that link is ever broken:
marking tests with a capability nobody's CI has would be a coverage deletion
wearing a skip's clothing (`ai/rules/no-parking.md`). The nightly is advisory and
may run under TCG emulation, so it is slower than a merge gate and reports rather
than blocks; run the QEMU target locally when you add a test, and say so.

Do NOT use `skip-os:value=darwin` as a substitute for `needs-linux`: `skip-os`
says "do not run here", whereas `needs-linux` documents intent ("this is a
Linux-only test, validated in QEMU") and keeps the test in the QEMU suite.

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

## Interop Labs and Docker-Based Tests Need a QEMU Runner Too

A Linux-only interop lab that runs as **Docker containers and depends on
host-kernel features** (L2TP, PPPoE, netfilter, ...) does NOT run on macOS or in
CI by itself: Docker Desktop's VM lacks the kernel modules, and the Alpine QEMU
VM has no Docker. Shipping only the Docker lab leaves the test unrunnable on the
dev machine. Every such lab MUST also ship a QEMU-runnable path; treat "it's
Linux-only / needs the host kernel" as the trigger to build the QEMU runner, not
as an excuse to skip it.

The pattern (do all four in the same change):

1. **Netns evidence script** `scripts/evidence/effective-<feature>.py`: run Ze and
   the peer daemon in two network namespaces joined by a veth, no Docker. Mirror
   `effective-l2tp-ppp.py` (LineCollector, marker waits, kernel probe, cleanup).
2. **Peer from Alpine packages**: install the peer daemon with `apk` via
   `--packages`, e.g. `xl2tpd` (L2TP) or `accel-ppp` (PPPoE). If the lab's Docker
   image built the peer from source, switch it to the Alpine package so both paths
   use the same build -- `accel-ppp`, `frr`, and `xl2tpd` are all in Alpine community.
3. **Runtime kernel for kernel modules**: if the feature needs a module absent from
   the stock Alpine VM kernel, run with `--kernel tmp/kernel/vmlinuz`, add the
   `CONFIG_*` to `gokrazy/kernel/runtime.config`, and add the symbol to
   `gokrazy/kernel/runtime.require`. PPPoE added `CONFIG_PPPOE` there exactly as
   L2TP added `CONFIG_PPPOL2TP`. `make ze-kernel` stages the kernel to
   `tmp/kernel/vmlinuz` (gitignored scratch) but routes through a DURABLE cache first: it
   asks `ze-host` for the arch+config-keyed dir under `~/.cache/ze/runtime-kernel` and
   materializes from it in seconds on a hit (no ~30-min rebuild), building + populating only
   on a miss (or a `runtime.config` change). So `rm -rf tmp` costs a copy, not a rebuild, and
   a fresh worktree reuses the compiled kernel. The Alpine ISO is likewise cached and
   `.sha256`-verified under `~/.cache/ze/alpine-iso`. `scripts/dev/ensure-links.py` maintains
   the repo `cache` symlink (and, after the opt-in `make ze-migrate-scratch`, the `tmp`
   symlink) so the expensive artifacts live outside the disposable scratch tree. See
   `plan/spec-relocate-scratch-and-cache.md`.
4. **`ze-qemu-<feature>-test` target** in `mk/test-integration.mk` calling
   `qemu-run.py --kernel ... --packages ... --run 'python3 scripts/evidence/effective-<feature>.py'`,
   added to `.PHONY` and the `Makefile` help block.

| Lab | Docker target | QEMU target | Netns script |
|-----|---------------|-------------|--------------|
| L2TP (Ze LNS vs xl2tpd) | `ze-deployment-l2tp-ppp-docker-test` | `ze-qemu-l2tp-ppp-test` | `effective-l2tp-ppp.py` |
| PPPoE (Ze client vs accel-ppp) | `ze-deployment-pppoe-accel-docker-test` | `ze-qemu-pppoe-accel-test` | `effective-pppoe-accel.py` |
| VRRP (Ze vs keepalived) | `ze-interop-test INTEROP_SCENARIO=vrrp-*-keepalived` | `ze-qemu-vrrp-keepalived-test` | `effective-vrrp-keepalived.py` |

When you add a new interop lab, add its row here and ship both targets together.

Step 3's custom kernel is conditional, not automatic: use `--kernel` only when a
`CONFIG_*` the lab needs is absent from the stock Alpine kernel. L2TP and PPPoE
need it (`CONFIG_PPPOL2TP`, `CONFIG_PPPOE`); VRRP does not, because the stock
Alpine 6.12.13-0-virt kernel already creates macvlan (bridge mode), bridge, veth
and netns (probed 2026-07-15). Adding `--kernel` when it is not needed forces a
~30-minute `make ze-kernel` build on everyone who runs the lab, so probe the
stock kernel before reaching for it.

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

On Linux: the invoking user MUST be in the `kvm` group. `/dev/kvm` is
`root:kvm` 0660, so a user outside it does not get a slow run, it gets no run:
qemu exits with `Could not access KVM kernel module: Permission denied` and the
calling evidence script reports the generic "did not reach SSH within the
timeout", which reads as flakiness. `make ze-setup` checks this as `kvm-access`
and applies `sudo usermod -aG kvm $USER`; the new group only reaches a new
login, so use `sg kvm -c '<command>'` in an existing shell. A host with no
`/dev/kvm` reports `n/a` and legitimately runs under TCG.

## Reference Implementations

| What | File |
|------|------|
| Network namespace helper | `internal/component/iface/integration_helpers_linux_test.go` |
| Netlink integration test | `internal/plugins/traffic/netlink/integration_linux_test.go` |
| nftables integration test | `internal/plugins/firewall/nft/integration_linux_test.go` |
| Route watch integration | `internal/core/routewatch/integration_linux_test.go` |
| PTY/termios integration | `internal/component/config/system/console_integration_linux_test.go` |
| QEMU runner script | `scripts/evidence/qemu-run.py` |

## What actually RUNS these suites

This rule says "BLOCKING", so it is worth being precise about which gate enforces
it, because for a long time none did.

Validation runs on **GitHub Actions** (`.github/workflows/`), not Codeberg. The
repo is pushed to both codeberg.org and github.com/ze-software/ze; CI moved to
GitHub because running heavy nightly sweeps on Codeberg's donated shared runners
is inconsiderate of a free service, and because GitHub's `ubuntu-latest` grants
the root / `CAP_NET_ADMIN` the integration suite needs, which the shared
Woodpecker instance could not.

| Suite | Where it runs | Blocking? |
|-------|---------------|-----------|
| `make ze-verify` (unit + functional + static gates) | `.github/workflows/verify.yml`, push + pull_request | yes |
| `ze-fuzz-test` | `.github/workflows/evidence-nightly.yml`, scheduled | advisory |
| `ze-integration-test` (non-QEMU kernel suites) | `.github/workflows/evidence-nightly.yml`, scheduled, `sudo` (root) | advisory |
| `ze-qemu-needs-linux-test` (Linux-only `.ci` functional surface) | `.github/workflows/qemu-nightly.yml`, scheduled | advisory |
| `ze-qemu-integration-test` (Go `integration && linux` packages) | NOTHING automated | -- |

Two notes on the nightly row:

The cron lives IN `evidence-nightly.yml` (`on: schedule: - cron:`), so merging it
to the default branch CREATES the schedule -- unlike Woodpecker, whose cron was a
separate repo setting nothing in the repo recorded. The one caveat: GitHub
disables scheduled workflows after 60 days with NO repository activity, so a long
quiet period silently stops the nightly; a `workflow_dispatch` (manual) trigger
is provided as the re-arm.

`ze-integration-test` runs here now, which it could not on Codeberg: its six
suites need `CAP_NET_ADMIN` / `CAP_NET_BIND_SERVICE` (`mk/test-integration.mk`),
and Woodpecker's only lever for that -- `privileged: true` -- is a BLOCKING lint
error on an untrusted shared instance that aborts the whole pipeline. On GitHub
the job simply runs under `sudo` as root, which has those capabilities natively.
It is advisory-first (`continue-on-error: true`): a red suite reports without
marking the run failed, until a green baseline lets it flip to blocking.

`ze-qemu-integration-test` is still NOT automated: it additionally needs nested
virt / KVM, which GitHub-hosted runners do not reliably provide. It remains
enforced by review and by this rule ALONE -- do not assume CI catches a broken
QEMU test for you; wiring it up needs a self-hosted or KVM-capable runner.

`scripts/dev/github_workflows_test.go` pins the workflow set: that the nightly is
scheduled-only, runs fuzz AND integration by make-target name, is advisory, does
not smuggle in the QEMU target, that `verify.yml` stays a fast push/pull_request
gate, that every `make <target>` any workflow names exists, and that no
`.woodpecker` pipeline remains.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| "Needs real hardware, skipping test" | Use a virtual substitute (see table above) |
| `//go:build linux` on a test that needs root | Use `//go:build integration && linux` |
| Forgetting to add package to Makefile | Test compiles but never runs in CI |
| Using `t.Fatal` for missing capabilities | Use `t.Skip` so the test is portable |
| Hardcoding `/dev/ttyS0` in a test | Use `pty.Open()` for a real PTY pair |
| Reading a QEMU evidence timeout as "tcg is slow" | On Linux, check `kvm-access` first (`make ze-setup CHECK=1`). A user outside the `kvm` group makes qemu refuse to start, which surfaces as a timeout |
| Selecting the accelerator on `Path("/dev/kvm").exists()` | Existence is not access. Probe `os.access(..., R_OK\|W_OK)`, and branch on `sys.platform == "darwin"` for `hvf` |
