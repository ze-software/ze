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
| Any new Linux-only package | Add it to `integrationPackages` in `internal/le/qemu/alltests.go` |
| Docker interop lab needing host-kernel features | Add a native `./le qemu <feature>` action beside the Docker action |

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

- On a non-Linux host (`GOOS != linux`): the runner MUST set `SkipReason` and the test MUST report **SKIP**, never FAIL. Native `./le verify worktree` / `./le functional` on darwin stays green without running the unsupported test.
- Inside the QEMU Alpine VM (`GOOS == linux`): the directive is inert, so the same `.ci` test runs for real against the Linux kernel.

Two QEMU entry points run these tests, both **one VM for all of them** (never
one VM per test):

- `./le qemu netns-test suites <comma-separated-suites>` runs the selected kernel-dependent functional suites for a tight iteration.
- `./le qemu all-tests` runs every functional suite, the unit pass, and every registered integration package inside the prepared VM.

No per-test wiring is needed for either: the suites are the same as the native
runner, so the QEMU pass discovers `needs-linux` tests automatically.

**Every functional QEMU proof MUST boot Ze's runtime kernel, never the stock Alpine kernel.**

`./le qemu run kernel <vmlinuz> command "<command>"` owns the Alpine image, QEMU process, bounded waits, SSH execution, and cleanup. When `kernel` is present, `Run.assertRuntimeKernel` (`internal/le/qemu/run_exec.go`) reads `internal/appliance/kernel.version` and refuses the result unless the guest reports that release. A failure to load the supplied kernel can leave the ISO kernel running, so checking the staged file on the host is insufficient.

The caller MUST supply the runtime kernel path. "The stock kernel has the needed feature" is not an exception: the verdict would describe Alpine's kernel while reading as a verdict about Ze. A focused proof passes its command through `./le qemu run`; the full in-guest test population is `./le qemu all-tests`.

`internal/le/qemu/run.go` owns the boot plan, and `internal/le/qemu/alltests.go` owns the functional-suite and integration-package populations. Update those Go producers together when the VM contract changes. `TestTheAreaPublishesTheNativeActions` and the all-tests coverage checks in `internal/le/qemu` refuse a command or suite that leaves the native inventory.

Decision rule:

| The `.ci` test ... | Use |
|--------------------|-----|
| Only validates config (`ze config validate -`), parses, or runs offline `ze show`/`ze env` | nothing (runs natively on every OS) |
| Boots a daemon that **applies** Linux-only config (interface/VLAN, firewall, L2TP kernel) | `option=needs-linux` |
| Same, AND needs privileged network configuration (creates interfaces, brings links up, netlink) | `option=needs-linux:caps=net-admin` |
| Same, AND opens a raw/packet socket (`resolve ping`, traceroute: `net.ListenPacket("ip4:icmp", ...)`) | `option=needs-linux:caps=net-raw` |
| Same, AND loads eBPF | `option=needs-linux:caps=bpf` |
| Needs to skip only on a specific non-Linux OS for an unrelated reason | `option=skip-os:value=darwin` |
| Needs an optional heavyweight artifact the checkout does not carry (the appliance module cache produced by `go mod download all`) | `option=needs-path:value=<repo-rel>:hint=<cmd>` |

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
DAEMON exits 1, so you MUST NOT read this as the daemon hanging. The TEST hangs
because its check peer waits for a BGP session the exited daemon will never
open. The gate reads `CapEff` from
`/proc/self/status` (`internal/test/runner/caps_linux.go`), not uid 0: a setcap'd
binary holds the capability without being root, and a restricted container can be
root without it.

A `caps=` typo is a parse error on every host, so it cannot silently disable the
gate.

**Know what you are trading.** A `caps=net-admin` test does NOT run in the merge
gate: `./le verify worktree` runs unprivileged, so the marker turns an opaque hang into
an honest skip there. Its home is `.github/workflows/qemu-nightly.yml`, which
runs `./le qemu all-tests` on a schedule, so the marker RELOCATES the
coverage rather than deleting it. `TestCapabilityGatedTestsHaveAQemuHome`
(`internal/le/workflowcheck/workflowcheck_test.go`) fails if that link is ever broken:
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
| `//go:build integration && linux` | Test needs kernel capabilities (root, /dev, netns, ioctl) | `./le qemu all-tests` (passes `-tags integration`) |

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

### 4. Dataplane counters need a real remote peer

**A probe that asserts on a counter sitting behind state written for a remote
peer MUST send its traffic over an egress that really carries it, and MUST carry
a positive control.** An `ip xfrm` byte counter is the case in hand. Two network
namespaces, two VMs or two containers satisfy the first requirement. A host
addressing itself does not.

**The reason is the SELECTOR, not the interface.** An `ip xfrm` byte counter
belongs to a security association whose policy names a remote peer. A packet a
host sends to its own address matches no such policy, so no SA encrypts it and
the counter stays at zero. The counter then reads zero for a working dataplane
and zero for a broken one. A counter that names no peer is outside this
directive: a plain nftables rule counter in an input or output chain does advance
for a self-addressed packet, so a probe MAY read it from one host.

**Without the positive control the zero is unreadable.** A run known to move the
counter is what separates "the mechanism is broken" from "this setup never moves
this counter". It is the absence-assertion trap
`ai/rules/interop-and-goal-validation.md` names: ask what would still be absent
if the mechanism were deleted.

**Prefer the PEER's counter to your own.** A local outbound counter advances on
any key, including a wrong one, because sending is not proof of acceptance. The
receiver's inbound counter advances only after it has accepted what arrived, so
it is the one that answers the question the probe is really asking.

### 5. Register the package in the native QEMU inventory

Add your package to `integrationPackages` in `internal/le/qemu/alltests.go`. `./le qemu all-tests` runs that closed list with the `integration` tag and refuses a path that does not exist.

```go
var integrationPackages = []string{
    "./internal/component/iface/...",
    "./internal/component/config/system/...", // add the package here
}
```

If a focused VM run needs extra Alpine packages such as `strace` or `util-linux`, pass them after the `packages` keyword to `./le qemu run`.

## Interop Labs and Docker-Based Tests Need a QEMU Runner Too

A Linux-only interop lab that runs as **Docker containers and depends on
host-kernel features** (L2TP, PPPoE, netfilter, ...) does NOT run on macOS or in
CI by itself: Docker Desktop's VM lacks the kernel modules, and the Alpine QEMU
VM has no Docker. Shipping only the Docker lab leaves the test unrunnable on the
dev machine. Every such lab MUST also ship a QEMU-runnable path; treat "it's
Linux-only / needs the host kernel" as the trigger to build the QEMU runner, not
as an excuse to skip it.

The pattern (do all four in the same change):

1. **Native netns evidence:** implement the lab under `internal/le` and register a named `./le deployment <verb>` or `./le qemu <verb>` action. Run Ze and the peer daemon in separate network namespaces joined by a veth, without Docker.
2. **Peer from Alpine packages:** install the peer daemon through the `packages` parameter of `./le qemu run`, or declare it in the dedicated native QEMU action. Use the same packaged peer in the Docker and QEMU proofs where Alpine supplies it.
3. **Runtime kernel, always:** pass Ze's staged runtime kernel through `./le qemu run kernel <vmlinuz>`. `Run.assertRuntimeKernel` refuses a guest whose `uname -r` does not match `internal/appliance/kernel.version`. Add every required `CONFIG_*` symbol to `gokrazy/kernel/runtime.config` and `gokrazy/kernel/runtime.require`.
4. **Registered action:** add the feature action to the owning Go action table and expose it through `./le qemu` or `./le deployment`. The bare area command is the inventory and must list the new action.

| Lab | Docker action | QEMU action | Native producer |
|-----|---------------|-------------|-----------------|
| L2TP (Ze LNS vs xl2tpd) | `./le deployment docker-l2tp-ppp-test` | `./le deployment gokrazy-l2tp-ppp-test` | `internal/le/deployment` |
| PPPoE (Ze client vs accel-ppp) | `./le deployment docker-pppoe-accel-test` | `./le qemu pppoe-accel-test` | `internal/le/qemu/pppoe_accel_linux.go` |
| VRRP (Ze vs keepalived) | `./le integration interop` | `./le qemu vrrp-keepalived-test` | `internal/le/qemu/vrrp_keepalived_linux.go` |

**When you add a new interop lab, you MUST add its row here and ship both native actions together.**

**Probing the stock Alpine kernel proves nothing about a lab, and its result MUST NOT be recorded as a reason to skip step 3's `--kernel`.** A probe answers a question about Alpine, while the lab's verdict is about the kernel ze ships, so a green probe and a green lab on stock together establish only that Alpine works. A capability the probe found MUST be declared in `gokrazy/kernel/runtime.config` with its symbol in `gokrazy/kernel/runtime.require`, so the lab gets it from the kernel under test and a silent demotion to `=m` fails the build instead of the lab.

## What the QEMU VM Provides

Alpine Linux live system (no systemd) with:

- Root access (all capabilities)
- `/dev/ptmx` for PTY pairs
- Network namespaces (`ip netns`)
- Kernel modules: NONE. Every QEMU target boots ze's runtime kernel, and a `--kernel` run pairs it with Alpine's initramfs and Alpine's `/lib/modules`, which are built for 6.12.13-0-virt, so no module loads at all: not Alpine's, and not ze's own. Every symbol any QEMU run needs MUST therefore be `=y` in `gokrazy/kernel/*.config`, and the matching `gokrazy/kernel/*.require` manifest is what makes a silent demotion to `=m` fail the build instead of the test. The Alpine modules a stock boot used to supply (nftables, l2tp, ppp) are now config symbols like every other
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
./le qemu all-tests
```

First run downloads Alpine ISO + Go toolchain (~1 min). Subsequent runs
reuse the cache in `tmp/qemu/` (~30s boot + test time).

On macOS: requires `qemu` (`brew install qemu`). Uses HVF acceleration
when available, falls back to TCG (software emulation).

On Linux: the invoking user MUST be in the `kvm` group. `/dev/kvm` is
`root:kvm` 0660, so a user outside it does not get a slow run, it gets no run:
qemu exits with `Could not access KVM kernel module: Permission denied` and the
calling evidence script reports the generic "did not reach SSH within the
timeout", which reads as flakiness. `./le setup` checks this as `kvm-access`
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
| QEMU runner script | `internal/le/qemu/run.go` |

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
| `./le verify worktree` (unit + functional + static gates) | `.github/workflows/verify.yml`, push + pull_request | yes |
| `./le fuzz` | `.github/workflows/evidence-nightly.yml`, scheduled | advisory |
| `./le integration` actions | `.github/workflows/evidence-nightly.yml`, scheduled; root where required | advisory |
| `./le qemu run ... command '<native action>'` for the Linux-only functional surface | `.github/workflows/qemu-nightly.yml`, job `needs-linux`, scheduled | advisory |
| `./le qemu` routing-protocol actions inside `./le qemu run` | `.github/workflows/qemu-nightly.yml`, job `protocol-labs`, scheduled | advisory |
| `./le qemu` access-protocol actions inside `./le qemu run` | `.github/workflows/qemu-nightly.yml`, job `runtime-kernel-labs`, scheduled | advisory |
| `./le integration interop`, `./le integration interop-ipsec` | `.github/workflows/evidence-nightly.yml`, scheduled | advisory |
| `./le qemu all-tests` inside the runtime-kernel guest | `.github/workflows/qemu-nightly.yml`, job `needs-linux`, scheduled | advisory |

Two notes on the nightly row:

The cron lives IN `evidence-nightly.yml` (`on: schedule: - cron:`), so merging it
to the default branch CREATES the schedule, unlike Woodpecker, whose cron was a
separate repo setting nothing in the repo recorded. The one caveat: GitHub
disables scheduled workflows after 60 days with NO repository activity, so a long
quiet period silently stops the nightly; a `workflow_dispatch` (manual) trigger
is provided as the re-arm.

Native integration actions run on GitHub because their suites need
`CAP_NET_ADMIN` or `CAP_NET_BIND_SERVICE`. GitHub jobs can run the action with
the required privileges; the shared Codeberg runner cannot grant them.
It is advisory-first (`continue-on-error: true`): a red suite reports without
marking the run failed, until a green baseline lets it flip to blocking.

**QEMU evidence is scheduled and advisory.** `.github/workflows/qemu-nightly.yml` drives `./le qemu run` with Ze's runtime kernel and invokes `./le qemu all-tests` inside the guest. You MUST NOT treat it as a blocking push gate or skip the focused QEMU proof for your change.

**Every registered `./le qemu` and `./le integration` action MUST have a real caller**: a workflow job, another native action, or an explicit manual classification. `TestQemuAndInteropTargetsHaveACaller` in `internal/le/workflowcheck/workflowcheck_test.go` derives actions and callers from the Go registries and workflows.

`internal/le/workflowcheck/workflowcheck_test.go` pins the workflow set: that the nightly is
scheduled-only, runs fuzz and integration by native action name, is advisory,
does not smuggle in the QEMU action, that `verify.yml` stays a fast
push/pull_request gate, that every `./le` action named by a workflow is
registered, and that no `.woodpecker` pipeline remains.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| "Needs real hardware, skipping test" | Use a virtual substitute (see table above) |
| `//go:build linux` on a test that needs root | Use `//go:build integration && linux` |
| Forgetting to add a Linux package to `integrationPackages` in `internal/le/qemu/alltests.go` | Test compiles but never runs in the QEMU inventory |
| Using `t.Fatal` for missing capabilities | Use `t.Skip` so the test is portable |
| Hardcoding `/dev/ttyS0` in a test | Use `pty.Open()` for a real PTY pair |
| Reading a QEMU evidence timeout as "tcg is slow" | On Linux, check `kvm-access` first (`./le setup check`). A user outside the `kvm` group makes qemu refuse to start, which surfaces as a timeout |
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

These init operations run in-process in Go:

- **Bring a link up / apply address + route**: you MUST use `netlink`, not `ip`.
- **DHCP lease**: you MUST use in-process `nclient4` (`internal/install/disk/dhcp_linux.go`), not `udhcpc` plus a lease script.
- **HTTP image/database download**: you MUST use `net/http` (`internal/install/disk/download.go`), not `wget`.
- **mount / umount / loop / mknod / reboot / poweroff**: you MUST use `golang.org/x/sys/unix` syscalls and ioctls isolated in named `_linux.go` helpers (`mount_linux.go`, `loop_linux.go`, `blockdev_linux.go`), not `mount`/`losetup`/`reboot`.

**Where a syscall is unavoidable, you MUST isolate it in a named `_linux.go` helper so the platform dependency is visible and testable behind a fake.** `internal/install/disk` and `cmd/ze-installer` MUST contain no `exec.Command` of an external binary; `./le qemu install-test` proves that the initrd boots and installs cleanly.

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
4. **You MUST regenerate the go.sums cleanly.** Delete the affected builddir `go.sum` files (filesystem removal, never `git rm`), then run `go mod download all` in each affected builddir. The sums regenerate from the new build list and prune the old version string. You MUST NOT hand-edit hashes.
5. **Re-vendor and prune.** The module download extracts the new version under `gokrazy/modcache/github.com/gokrazy/gokrazy@<new>/`. Remove the old `@<old>` directory. Confirm the working tree holds only the expected old-file deletions and new source.
6. **Refresh coupling.** Search for the old version string and update every document or spec that names the old module-cache path.
7. **Verify (BLOCKING).** Confirm the old version string is absent, the new committed `go.mod` names the fixed dependency, `ze appliance build` succeeds, and `./le deployment gokrazy-l2tp-ppp-test` boots the appliance. An image build alone is insufficient.

On step 4, one of the eight go.sum files is **untracked**:
`gokrazy/ze/builddir/github.com/ze-software/ze/go.sum` is gitignored (see
`.gitignore`), because that module is only `replace ze => <repo root>` and every
line of its sum is already in the root `go.sum`. Regenerate it like the rest;
expect no diff. The other seven are tracked locks and DO show a diff.

On step 7, use a proof that actually boots an image, and check what it asserts
before citing it:

| Proof | What it does | Use it for |
|-------|--------------|------------|
| `./le qemu vpp-hugepages-test` | builds a real image via `ze appliance build`, boots it in QEMU, asserts the kernel cmdline and the reserved hugepage count | the default boot proof |
| `./le deployment gokrazy-l2tp-ppp-test` | builds the appliance and boots it against a real LAC | the L2TP path |
| ~~`test/appliance/serial-login.ci`~~ | **boots nothing.** Its header says the QEMU plan applies "when appliance serial test infrastructure is ready"; it asserts the argv[0] shell-invocation gate offline | never cite it as a boot proof |

**A `SKIP` MUST NOT be treated as evidence.** Under a hardware accelerator the hugepage proof treats a no-answer as a FAIL; if it skips for want of KVM access, you MUST fix that (on Linux, group membership: `./le setup check`) and rerun.

### Git safety

**The re-vendor deletes ~60 tracked files and adds ~60 new ones. You MUST NOT use bare `git rm`/`git add`: you MUST stage the whole change through the commit-helper script at closure so the deletion and addition land in one commit.**

### Cache permissions

**Anything that downloads into `gokrazy/modcache/` MUST carry `-modcacherw` (`GOFLAGS=-modcacherw`):** go's default read-only cache permissions (dirs `r-x`) make git unable to delete or overwrite modcache files on later checkouts and rebases (a `git pull --rebase` across the 2026-07 init bump wedged exactly this way).

`ze appliance build` (`ensureModcacheRW`, `internal/appliance/cmd_build.go`) and
`ze-gok` (`cmd/ze-gok/main.go`) set it. Keep the flag when running
`go mod download` directly. A cache written before the flag existed needs a
one-time `chmod -R u+w gokrazy/modcache`.

### Module cache hygiene: what may accumulate, and what must never

`gokrazy/modcache/` is a real Go module cache and Go never garbage-collects it.
Two kinds of growth are expected, one is a defect.

**Expected.** Superseded versions after a pin bump (runbook step 5 tells you to
`rm -rf` the old dir; you MUST do it, or every bump leaves 15-50 MB behind), and the breadth
of `go mod download all`, which is the whole module graph
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

A derived parent needs to pass its `builddir` to `gok`; an instance with no
`builddir` discards every pin
(`vendor/github.com/gokrazy/tools/packer/gotool.go`, `getPkg`/`getIncomplete`).

That route is closed. **Every** image build now runs from a prepared copy of the
instance under the project `tmp/`, carrying the full `builddir` with its
filesystem-path replaces rewritten to absolute paths
(`internal/appliance/instance`). Both entry points go through it: `ze appliance
build` via `resolveBuildParentDir`, and `ze-gok` via `cmd/ze-gok`, which rewrites
`--parent_dir` before gok sees it. Preparation fails closed when the builddir is
missing or empty, rather than letting gok synthesize modules.

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

Some root `go.mod` direct dependencies are pinned to pseudo-versions
(`v0.0.0-<date>-<hash>`) because their upstreams publish no semver tag. Confirm
with `go list -m -versions`, `proxy.golang.org/<mod>/@v/list`, and `@latest`
before you classify a pseudo-version pin as a defect.

A module that disappears from this table has either been tagged or moved. Find
out which before you re-add a row.

| Root dep (root `go.mod`) | Pin form | Upstream semver tag? |
|--------------------------|----------|----------------------|
| `github.com/insomniacslk/dhcp` | pseudo-version | none published |
| `github.com/packetcap/go-pcap` | pseudo-version | none published |
| `golang.zx2c4.com/wireguard/wgctrl` | pseudo-version | none published |
| `github.com/gokrazy/tools` | pseudo-version | none published |
| `github.com/gokrazy/updater` | pseudo-version | none published |

**You MUST keep the pseudo-versions. You MUST re-check for a first tag when bumping any of these, and move the pin to a tag the day upstream cuts one.** Until then a pseudo-version is the only available form and is legal. The note exists so a future reviewer does not "fix" a non-problem or assume the pins were never examined.
