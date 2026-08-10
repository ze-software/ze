# Linux, QEMU and the Appliance

**When:** writing Linux-only code, changing the installer initrd, or bumping and booting an appliance dependency
**Severity:** blocking
**Related:** completion, testing, git-safety

## Directives

- Linux-only code (`//go:build linux`) MUST ship with integration tests that run in the QEMU Alpine VM. "Needs real hardware" is never a valid reason to skip tests. Virtual substitutes exist for every kernel feature ze uses.
- A functional `.ci` test that boots a daemon which exercises a real Linux kernel feature MUST be marked `option=needs-linux`, and MUST be validated inside the QEMU Alpine VM, never natively on darwin.
- Every Linux-only interop lab that runs as Docker containers and depends on host-kernel features MUST also ship a QEMU-runnable path. Treat "it is Linux-only / needs the host kernel" as the trigger to build the QEMU runner, not as an excuse to skip it.
- The installer initrd is a single statically-linked Go binary (`cmd/ze-installer`) running as PID 1 with zero external binaries (busybox removed). Detect system state through `/proc` and `/sys` reads, not external commands, and never reintroduce `exec.Command` of an external tool.
- A Dependabot alert on a `go.mod` under `gokrazy/modcache/` is almost always a stale vendored upstream manifest, not your real dependency graph. Follow the runbook under "Appliance Dependency Bumps".
- Never cross-compile a host binary. A target-arch `ze-host` cannot exec on the build host ("exec format error"). Apply `GOARCH=<target>` only to the build of a target binary, or to the `ze appliance initrd` invocation that cross-compiles one internally, never to the build of the host tool that runs it.

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

- On a non-Linux host (`GOOS != linux`): the runner MUST set `SkipReason` and the test MUST report **SKIP**, never FAIL. Native `make ze-verify` / `make ze-functional-test` on darwin stays green without running the unsupported test.
- Inside the QEMU Alpine VM (`GOOS == linux`): the directive is inert, so the same `.ci` test runs for real against the Linux kernel.

Two QEMU entry points run these tests, both **one VM for all of them** (never
one VM per test):

- `make ze-qemu-needs-linux-test` -- the tight loop. Sets `ZE_QEMU_LINUX_ONLY=1`, which flips the runner to skip every test that is NOT `needs-linux`, so the VM spends its whole boot only on the Linux-only surface. You MAY use this while iterating on a Linux-only feature.
- `make ze-qemu-all-test` -- the full pass. Runs every functional suite in the VM, so `needs-linux` tests are exercised alongside everything else.

No per-test wiring is needed for either: the suites are the same as the native
runner, so the QEMU pass discovers `needs-linux` tests automatically.

**Both targets boot ze's own runtime kernel, never the stock Alpine one
(2026-08-07).** Each passes `--kernel tmp/kernel/vmlinuz` and refuses to start
without it, so `make ze-kernel KERNEL_ARCH=<amd64|arm64>` is a precondition of
each. That command costs a copy on a cache hit and only builds on a miss.

**The guard compares, it does not merely check existence.** `GOKRAZY_ARCH`
defaults to `amd64` on every host while `QEMU_GOARCH` follows `uname`, so a bare
`make ze-kernel` on an Apple Silicon machine stages an amd64 vmlinuz that a
`test -f` accepts and QEMU then fails to boot, with no line naming the
architecture. `ze-qemu-kernel-guard` (`mk/test-integration.mk`) compares the
staged kernel against the architecture-keyed durable cache entry instead, which
also catches a kernel staged before a config fragment changed. A missing or
mismatched kernel is an error exit, never a silent fall back to stock.

**All six kernel-consuming targets use that one guard**, the two above plus
`ze-qemu-pppoe-test`, `ze-qemu-l2tp-ppp-test`, `ze-qemu-pppoe-accel-test` and
`ze-qemu-traffic-usage-test`. **A target that uses it MUST declare `: ze-host`**,
because the guard's first command execs that binary; without the prerequisite it
still denies, but it names the wrong cause. `TestQemuTargetsGuardTheStagedKernel`
(`scripts/evidence/qemu_kernel_wiring_test.go`) reads the guard's users out of the
makefile rather than from a list, so a seventh target is checked the day it is
written.

**Why they moved.** The stock Alpine 6.12.13-0-virt kernel crashes on the nft
set-element-timeout operations the firewall suite performs, so `firewall` sat in
the default skip list and the suite proved nothing. ze also declares that kernel
unsupported: `tools/kernel-builder/build.py` refuses anything below 7.0. On
7.1.4 the same operations succeed and the VM survives them, so `firewall` left
that list. **Two files carry the default and they MUST move together**:
`mk/test-integration.mk` and `scripts/evidence/qemu-all-tests.sh`. The script
default wins whenever the script is invoked directly, so changing only the
makefile leaves the old behavior in force.

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

**`skip-os:value=darwin` MUST NOT be used as a substitute for `caps=`.** It hides a test from
macOS and therefore RUNS it, unprivileged, on the Linux CI runner, which is
exactly where it cannot pass. `test/plugin/resolve-ping.ci` carried a bare
`skip-os` and failed every CI run with `resolve ping status=error`, because
`doPingCtx` (`internal/component/ping/cmd/ping.go`) needs CAP_NET_RAW. If the
reason a test cannot run on macOS is a capability, you MUST declare the capability.

**`caps=net-admin` exists because Linux alone is not the requirement.** On an
unprivileged Linux host (CI runners, most dev boxes, any rootless container) a
test that applies interface config does NOT fail cleanly. The interface plugin
fails its stage-2 (configure) handshake with `operation not permitted` and the
DAEMON does exit 1 (verified 2026-07-25 in QEMU as an unprivileged user), so you
MUST NOT read this as the daemon hanging. The TEST hangs, to the suite timeout,
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
runs `ze-qemu-needs-linux-test` on a schedule, so the marker RELOCATES the
coverage rather than deleting it. `TestCapabilityGatedTestsHaveAQemuHome`
(`scripts/dev/github_workflows_test.go`) fails if that link is ever broken:
marking tests with a capability nobody's CI has would be a coverage deletion
wearing a skip's clothing (`ai/rules/completion.md`). The nightly is advisory and
MAY run under TCG emulation, so it is slower than a merge gate and reports rather
than blocks; you MUST run the QEMU target locally when you add a test, and say so.

**You MUST NOT use `skip-os:value=darwin` as a substitute for `needs-linux`:** `skip-os` says "do not run here", whereas `needs-linux` documents intent ("this is a Linux-only test, validated in QEMU") and keeps the test in the QEMU suite.

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

1. **Netns evidence script** `scripts/evidence/effective-<feature>.py`: you MUST run Ze and the peer daemon in two network namespaces joined by a veth, no Docker. You MUST mirror `effective-l2tp-ppp.py` (LineCollector, marker waits, kernel probe, cleanup).
2. **Peer from Alpine packages**: you MUST install the peer daemon with `apk` via `--packages`, e.g. `xl2tpd` (L2TP) or `accel-ppp` (PPPoE). If the lab's Docker image built the peer from source, you MUST switch it to the Alpine package so both paths use the same build: `accel-ppp`, `frr`, and `xl2tpd` are all in Alpine community.
3. **Runtime kernel for kernel modules**: if the feature needs a module absent from the stock Alpine VM kernel, you MUST run with `--kernel tmp/kernel/vmlinuz`, add the `CONFIG_*` to `gokrazy/kernel/runtime.config`, and add the symbol to `gokrazy/kernel/runtime.require`. PPPoE added `CONFIG_PPPOE` there exactly as L2TP added `CONFIG_PPPOL2TP`. `make ze-kernel` stages the kernel to `tmp/kernel/vmlinuz` (gitignored scratch) but routes through a DURABLE cache first: it asks `ze-host` for the arch+config-keyed dir under `~/.cache/ze/runtime-kernel` and materializes from it in seconds on a hit (no ~30-min rebuild), building + populating only on a miss (or a `runtime.config` change). So `rm -rf tmp` costs a copy, not a rebuild, and a fresh worktree reuses the compiled kernel. The Alpine ISO is likewise cached and `.sha256`-verified under `~/.cache/ze/alpine-iso`. `scripts/dev/ensure-links.py` maintains the repo `cache` symlink (and, after the opt-in `make ze-migrate-scratch`, the `tmp` symlink) so the expensive artifacts live outside the disposable scratch tree.
4. **You MUST add a `ze-qemu-<feature>-test` target** in `mk/test-integration.mk` calling `qemu-run.py --kernel ... --packages ... --run 'python3 scripts/evidence/effective-<feature>.py'`, and add it to `.PHONY` and the `Makefile` help block.

| Lab | Docker target | QEMU target | Netns script |
|-----|---------------|-------------|--------------|
| L2TP (Ze LNS vs xl2tpd) | `ze-deployment-l2tp-ppp-docker-test` | `ze-qemu-l2tp-ppp-test` | `effective-l2tp-ppp.py` |
| PPPoE (Ze client vs accel-ppp) | `ze-deployment-pppoe-accel-docker-test` | `ze-qemu-pppoe-accel-test` | `effective-pppoe-accel.py` |
| VRRP (Ze vs keepalived) | `ze-interop-test INTEROP_SCENARIO=vrrp-*-keepalived` | `ze-qemu-vrrp-keepalived-test` | `effective-vrrp-keepalived.py` |

**When you add a new interop lab, you MUST add its row here and ship both targets together.**

Step 3's custom kernel is conditional for a LAB, not automatic: use `--kernel`
only when a `CONFIG_*` the lab needs is absent from the stock Alpine kernel.
L2TP and PPPoE need it (`CONFIG_PPPOL2TP`, `CONFIG_PPPOE`); VRRP does not,
because the stock Alpine 6.12.13-0-virt kernel already creates macvlan (bridge
mode), bridge, veth and netns (probed 2026-07-15). Probe the stock kernel before
reaching for it, so a lab that gains nothing does not gain a precondition.

**The cost that used to decide this is gone (2026-08-07).** `make ze-kernel`
routes through the durable architecture- and config-keyed cache under
`~/.cache/ze`, so it materializes in seconds on a hit and builds only on a miss
or after a config fragment changes. The older advice, that `--kernel` "forces a
~30-minute build on everyone who runs the lab", described a checkout where the
kernel lived in `tmp/`. It now costs a copy. The two functional targets
(`ze-qemu-all-test`, `ze-qemu-needs-linux-test`) use `--kernel` unconditionally
for that reason.

## What the QEMU VM Provides

Alpine Linux live system (no systemd) with:

- Root access (all capabilities)
- `/dev/ptmx` for PTY pairs
- Network namespaces (`ip netns`)
- Kernel modules: nftables, l2tp, ppp (loaded at boot) -- **only under the stock Alpine kernel.** A `--kernel` run pairs ze's kernel with Alpine's initramfs and Alpine's `/lib/modules`, which are built for 6.12.13-0-virt, so NO module of the ze kernel can load. Every symbol such a run needs MUST be `=y` in `gokrazy/kernel/*.config`, and `gokrazy/kernel/kernel.require` is what makes a silent demotion to `=m` fail the build instead of the test
- Go toolchain (downloaded and cached in `tmp/qemu/`)
- Repo mounted read-write via virtio-9p at `/workspace`
- No systemd, no getty, no desktop (Alpine minimal)

## What the QEMU VM Does NOT Provide

- systemd (you MUST use Alpine's OpenRC or skip systemd-specific tests)
- Physical serial ports (you MUST use PTY pairs)
- Multiple physical NICs (you MUST use veth pairs)
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

This rule is blocking, so it is worth being precise about which gate enforces
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
to the default branch CREATES the schedule, unlike Woodpecker, whose cron was a
separate repo setting nothing in the repo recorded. The one caveat: GitHub
disables scheduled workflows after 60 days with NO repository activity, so a long
quiet period silently stops the nightly; a `workflow_dispatch` (manual) trigger
is provided as the re-arm.

`ze-integration-test` runs here now, which it could not on Codeberg: its six
suites need `CAP_NET_ADMIN` / `CAP_NET_BIND_SERVICE` (`mk/test-integration.mk`),
and Woodpecker's only lever for that (`privileged: true`) is a BLOCKING lint
error on an untrusted shared instance that aborts the whole pipeline. On GitHub
the job simply runs under `sudo` as root, which has those capabilities natively.
It is advisory-first (`continue-on-error: true`): a red suite reports without
marking the run failed, until a green baseline lets it flip to blocking.

**`ze-qemu-integration-test` is still NOT automated:** it additionally needs nested virt / KVM, which GitHub-hosted runners do not reliably provide. It remains enforced by review and by this rule ALONE. You MUST NOT assume CI catches a broken QEMU test for you; wiring it up needs a self-hosted or KVM-capable runner.

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

## Initrd: Prefer Procfs/Sysfs Over External Commands

Read before modifying the installer initrd (`cmd/ze-installer`,
`internal/install/disk/*_linux.go`).

The installer initrd is a single statically-linked Go binary (`cmd/ze-installer`)
running as PID 1 with **zero external binaries** (busybox removed). Detect system
state through `/proc` and `/sys` reads, not external commands, and never
reintroduce `exec.Command` of an external tool.

### Decision table

| Need | Use | Do NOT use |
|------|-----|------------|
| Check for IPv4 connectivity | `/proc/net/route` (default route = `00000000`) | `ip addr`, `ifconfig` |
| Check NIC carrier/link | `/sys/class/net/*/carrier` | `ip link show`, `ethtool` |
| Check interface flags | `/sys/class/net/*/flags` | `ip link show` |
| List interfaces | `/sys/class/net/` glob | `ip link`, `ls /sys/class/net` |
| Read NIC operstate | `/sys/class/net/*/operstate` | `ip link show` |
| Bring interface up, set address/route | `netlink` (`internal/plugins/iface/netlink`) | `ip link set`, `ip addr`, `ip route` |

### In-process replacements (no external client)

The operations the old shell init shelled out to are now in-process Go:

- **Bring a link up / apply address + route**: you MUST use `netlink`, not `ip`.
- **DHCP lease**: you MUST use in-process `nclient4` (`internal/install/disk/dhcp_linux.go`), not `udhcpc` plus a lease script.
- **HTTP image/database download**: you MUST use `net/http` (`internal/install/disk/download.go`), not `wget`.
- **mount / umount / loop / mknod / reboot / poweroff**: you MUST use `golang.org/x/sys/unix` syscalls and ioctls isolated in named `_linux.go` helpers (`mount_linux.go`, `loop_linux.go`, `blockdev_linux.go`), not `mount`/`losetup`/`reboot`.

**Where a syscall is unavoidable, you MUST isolate it in a named `_linux.go` helper so the platform dependency is visible and testable behind a fake.** `internal/install/disk` and `cmd/ze-installer` MUST contain no `exec.Command` of an external binary; a QEMU install (`make ze-install-qemu-test`) proves it boots and installs cleanly.

## Appliance Dependency Bumps

A Dependabot alert on a `go.mod` under `gokrazy/modcache/` is almost always a
stale *vendored upstream manifest*, not your real dependency graph. Follow this
runbook.

### Why this happens

The appliance is built by `gok` (`cmd/ze-gok`, wrapping `github.com/gokrazy/tools`).
`gok` compiles every appliance package with `go build -mod=mod` and fetches with
`go get` (`vendor/github.com/gokrazy/tools/packer/gotool.go`). It has **zero vendor
support**: a `vendor/` tree in a builddir is ignored. So the build resolves through a
**checked-in module cache** (`gokrazy/modcache/`, `GOMODCACHE` set by `cmd/ze-gok/main.go`).

`gokrazy/modcache/.gitignore` ignores everything except the gokrazy init source
(`github.com/gokrazy/gokrazy@*/**`). That committed source includes upstream's own
`go.mod`, and GitHub's dependency graph scans **every** `go.mod` in the repo as a
manifest. When upstream's `go.mod` names a version with a later advisory, the alert
fires on that file even though the image never builds the vulnerable version (the
builddir modules pin the fix and MVS takes the max).

**You MUST NOT try to convert to `go mod vendor`: `gok` cannot consume it. You MUST NOT hand-edit modcache go.sum hashes.**

### The fix: bump the vendored init to an upstream commit that carries the fix

1. **Find a fixed upstream version.** You MUST fetch the candidate `.mod` from the proxy (`https://proxy.golang.org/github.com/gokrazy/gokrazy/@v/<version>.mod` or `@latest`) and confirm it `require`s the fixed dependency version. Only then bump.
2. **You MUST bump the version string in the 7 builddir modules** under `gokrazy/ze/builddir/`: the `require` in `gokrazy` + `cmd/{dhcp,ntp,heartbeat,randomd}`, and the `replace` RHS in `serial-busybox` + `rtr7/kernel`. <!-- doc-links: ignore (cmd/{dhcp,ntp,heartbeat,randomd} are gokrazy submodules under gokrazy/ze/builddir/github.com/gokrazy/gokrazy/, not top-level cmd/) -->
3. **You MUST remove any now-false workaround pin/comment** (e.g. an explicit `x/net` pin added because "upstream pins the old version"). Verify it is safe: `go list -m <dep>` in each builddir MUST still resolve `>=` the fixed version via the new upstream `require`.
4. **You MUST regenerate the go.sums cleanly.** You MUST delete the affected builddir `go.sum` files (filesystem `rm`, never `git rm`), then run `make ze-gokrazy-deps` (runs `go mod download all` per builddir; the deleted sums regenerate from the new build list, pruning the old version string). You MUST NOT hand-edit hashes.
5. **Re-vendor + prune.** `ze-gokrazy-deps` extracts the new version's source under `gokrazy/modcache/github.com/gokrazy/gokrazy@<new>/` (auto-whitelisted by the `@*` glob). You MUST `rm -rf` the old `@<old>` directory. You MUST confirm the working tree: old tracked files deleted, new source untracked, nothing unexpected (no docs/website, no binaries).
6. **Refresh coupling.** You MUST run `git grep <old-version-string>`, and update any doc/spec that referenced the old modcache path (e.g. `plan/spec-kernel-lockdown-hardening.md`).
7. **Verify (BLOCKING).** You MUST confirm `grep -r <old-version>` is empty; the new committed `go.mod` names the fixed dependency version; `make ze-gokrazy` builds; and the appliance **boots in QEMU**. The image *build* alone is not sufficient: an init bump can regress boot.

On step 4, one of the eight go.sum files is **untracked**:
`gokrazy/ze/builddir/github.com/ze-software/ze/go.sum` is gitignored (see
`.gitignore`), because that module is only `replace ze => <repo root>` and every
line of its sum is already in the root `go.sum`. Regenerate it like the rest;
expect no diff. The other seven are tracked locks and DO show a diff.

On step 7, use a proof that actually boots an image, and check what it asserts
before citing it:

| Proof | What it does | Use it for |
|-------|--------------|------------|
| `make ze-vpp-hugepages-qemu-test` | builds a real image via `ze appliance build`, boots it in QEMU, asserts the kernel cmdline and the reserved hugepage count | the default boot proof |
| `ze-deployment-gokrazy-l2tp-ppp-test` | builds the appliance and boots it against a real LAC | the L2TP path |
| ~~`test/appliance/serial-login.ci`~~ | **boots nothing.** Its header says the QEMU plan applies "when appliance serial test infrastructure is ready"; it asserts the argv[0] shell-invocation gate offline | never cite it as a boot proof |

**A `SKIP` MUST NOT be treated as evidence.** Under a hardware accelerator the hugepage proof treats a no-answer as a FAIL; if it skips for want of KVM access, you MUST fix that (on Linux, group membership: `make ze-setup CHECK=1`) and rerun.

### Git safety

**The re-vendor deletes ~60 tracked files and adds ~60 new ones. You MUST NOT use bare `git rm`/`git add`: you MUST stage the whole change through the commit-helper script at closure so the deletion and addition land in one commit.**

### Cache permissions

**Anything that downloads into `gokrazy/modcache/` MUST carry `-modcacherw` (`GOFLAGS=-modcacherw`):** go's default read-only cache permissions (dirs `r-x`) make git unable to delete or overwrite modcache files on later checkouts and rebases (a `git pull --rebase` across the 2026-07 init bump wedged exactly this way).

`make ze-gokrazy-deps` (`mk/gokrazy.mk`), `ze appliance build`
(`ensureModcacheRW`, `internal/appliance/cmd_build.go`), and `ze-gok`
(`cmd/ze-gok/main.go`) all set it; keep the flag when running `go mod download` by
hand. A cache written before the flag existed needs a one-time
`chmod -R u+w gokrazy/modcache`.

### Module cache hygiene: what may accumulate, and what must never

`gokrazy/modcache/` is a real Go module cache and Go never garbage-collects it.
Two kinds of growth are expected, one is a defect.

**Expected.** Superseded versions after a pin bump (runbook step 5 tells you to
`rm -rf` the old dir; you MUST do it, or every bump leaves 15-50 MB behind), and the breadth
of `go mod download all` (`mk/gokrazy.mk`), which is the whole module graph
including test-only deps and their fixtures: `pierrec/lz4` is 75 MB of `testdata/`,
`klauspost/compress` 46 MB. A second Go toolchain also lands here
(`golang.org/toolchain@...`, ~310 MB with its zip) whenever a builddir `go`
directive is newer than the host toolchain and `GOTOOLCHAIN=auto`.

**A defect: this MUST NOT happen.** Either of these means a build resolved over the network instead of
through the pins, and the version it built is not the version this repo chose:

| What you find | What it means |
|---------------|---------------|
| `github.com/ze-software/ze@v0.0.0-<date>-<hash>` | ze was fetched from the proxy. The builddir replaces ze with the working tree, so a build that reaches the proxy for ze did not read the builddir, and it compiled a *pushed commit* rather than your tree |
| A version of a builddir-pinned module that is not the pinned one | `gok` fell back to `go get` and took whatever upstream had. For `github.com/rtr7/kernel` that is the appliance's **kernel** |

Both were live between 2026-07-18 and 2026-07-22: the derived (hugepage) parent
handed `gok` an instance with no `builddir`, so every pin was discarded
(against `vendor/github.com/gokrazy/tools/packer/gotool.go` `getPkg`/`getIncomplete`).

That route is closed. **Every** image build now runs from a prepared copy of the
instance under the project `tmp/`, carrying the full `builddir` with its
filesystem-path replaces rewritten to absolute paths
(`internal/appliance/instance`). Both entry points go through it: `ze appliance
build` via `resolveBuildParentDir`, and `make ze-gokrazy` via `cmd/ze-gok`, which
rewrites `--parent_dir` before gok sees it. Preparation fails closed when the
builddir is missing or empty, rather than letting gok synthesize modules.

`TestPrepareRealInstanceCarriesEveryModule` and
`TestPreparedModulesResolveIdenticallyToTracked` gate it against the real
eight-module instance, the latter by comparing `go list -m all` before and after
preparation. **A reappearance is a regression in whatever new path prepares an instance: find that path, do not just delete the directory.**

**You MUST NOT `rm -rf gokrazy/modcache`.** 60 tracked files live inside it (the gokrazy init source, whitelisted by `gokrazy/modcache/.gitignore`). You MUST delete named `@version` directories plus their `cache/download/<module>/@v/<version>.*` files, and confirm with `git status --porcelain gokrazy/` that nothing tracked moved.

### Do not just dismiss

**Dismissing the alert leaves the stale manifest; a future advisory below the pin will re-fire on the same file. You MUST bump the pin instead of dismissing the alert: bumping removes the manifest at the source.**

### Proactive review cadence (builddir pins)

The appliance builddir modules (`gokrazy/ze/builddir/`) and the checked-in module
cache (`gokrazy/modcache/`) are **excluded from Dependabot** (`.github/dependabot.yml`)
on purpose: an automated PR would fight the hand-pin (the MVS `max` is chosen
deliberately, and a bot bump reopens the stale-manifest churn described above).
Dependabot stays off; a **proactive review** replaces it: *review*, never an
automated bump.

**Cadence:** you MUST review the builddir pins **once per release cycle, and at minimum quarterly**, whichever comes first. Each review:

1. For the vendored gokrazy init and `rtr7/kernel`, you MUST fetch the latest upstream `.mod` from the proxy (as in "The fix" above) and note whether a newer commit carries security-relevant fixes.
2. If a fix applies, you MUST run the bump runbook above. If not, you MUST record the review date so the next reviewer knows the pins were checked, not forgotten.
3. You MUST re-confirm the GPLv2 source-offer sign-off below is still current.

**The pins MUST move only through the runbook; they MUST NOT move through a bot PR.**

### GPLv2 source-offer sign-off (rtr7/kernel): UNRESOLVED, flag only

The appliance image ships a GPLv2 Linux kernel: `github.com/rtr7/kernel`
(`gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod`, pinned as an indirect
pseudo-version). Distributing a GPLv2 binary obliges the distributor to make the
**corresponding source** available (typically a written offer accompanying the image).

**Status: UNRESOLVED.** No source-offer compliance sign-off is recorded. This note
**flags** the obligation; it does not adjudicate it. That is a licensing/legal call,
out of scope here. **Before the image is distributed to third parties, a source-offer sign-off MUST be produced and recorded.** Re-confirm each review cycle above.

### Root-module pseudo-version pins (no upstream tags)

Separate from the builddir concern: five **root** `go.mod` direct dependencies are
pinned to pseudo-versions (`v0.0.0-<date>-<hash>`) rather than semver tags. This is
**not a defect**. It was verified (2026-07-21, `spec-fixit-supply-chain-hardening`
AC-4) that **none of these upstreams publish any semver tag**: `go list -m -versions`
and `proxy.golang.org/<mod>/@v/list` return an empty version list for every one, and
`@latest` resolves to a pseudo-version. There is nothing to move the pin to.

The list was six until 2026-08-07. `github.com/charmbracelet/ssh` left it because
upstream MOVED the module rather than tagging it: the same code now publishes as
`charm.land/ssh`, which carries semver, and the root pin is `charm.land/ssh v0.4.2`.
A module that disappears from this table has either been tagged or been moved. Find
out which before you re-add a row.

| Root dep (root `go.mod`) | Pin form | Upstream semver tag? |
|--------------------------|----------|----------------------|
| `github.com/insomniacslk/dhcp` | pseudo-version | none published |
| `github.com/packetcap/go-pcap` | pseudo-version | none published |
| `golang.zx2c4.com/wireguard/wgctrl` | pseudo-version | none published |
| `github.com/gokrazy/tools` | pseudo-version | none published |
| `github.com/gokrazy/updater` | pseudo-version | none published |

**You MUST keep the pseudo-versions. You MUST re-check for a first tag when bumping any of these, and move the pin to a tag the day upstream cuts one.** Until then a pseudo-version is the only available form and is legal. The note exists so a future reviewer does not "fix" a non-problem or assume the pins were never examined.
