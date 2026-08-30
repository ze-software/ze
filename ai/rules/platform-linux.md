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

## Linux-only functional (`.ci`) tests run via QEMU, never natively

**A functional `.ci` test that boots a daemon, or runs `ze`, against a real Linux kernel feature MUST carry `option=needs-linux`.** Netlink interface, VLAN and veth creation, nftables, kernel sockets and the L2TP or PPPoE kernel paths are all such features. The test cannot pass natively on darwin, and the marker is what routes it to the QEMU Alpine VM instead. Which marker each test needs, and how `caps=` narrows it, is `docs/architecture/testing/ci-format.md`.

**`option=skip-os:value=darwin` MUST NOT stand in for `option=needs-linux`, and it MUST NOT stand in for `caps=`.** `skip-os` says "do not run here", so it hides the test from macOS and therefore RUNS it, unprivileged, on the Linux CI runner, which is exactly where it cannot pass. `needs-linux` states the intent, keeps the test in the QEMU suite, and `caps=` declares the capability the host has to hold. When the reason a test cannot run on macOS is a capability, you MUST declare that capability.

**A `caps=` marker RELOCATES coverage; it MUST NOT delete it.** `./le verify worktree` runs unprivileged, so a `caps=net-admin` test does not run in the merge gate. Its home is the scheduled QEMU nightly, and `TestCapabilityGatedTestsHaveANativeVMHome` (`internal/le/workflowcheck/workflowcheck_test.go`) fails when that link is broken: a capability nobody's CI has would be a coverage deletion wearing a skip's clothing (`ai/rules/completion.md`). The nightly reports rather than blocks, so you MUST run the QEMU target locally when you add such a test, and MUST say so. The workflow map is `docs/architecture/testing/ci-workflows.md`.

**A tight loop MAY be used while iterating, and the full pass MUST be the one that reports the result**, because it is the only form that covers the whole population. The entry points and the population each one covers are `docs/architecture/testing/qemu-integration.md`.

**Every functional QEMU proof MUST boot Ze's runtime kernel, never the stock Alpine kernel, and the caller MUST supply that kernel path.** `./le qemu run kernel <vmlinuz> command "<command>"` owns the Alpine image, the QEMU process, the bounded waits, SSH execution and cleanup; `Run.assertRuntimeKernel` (`internal/le/qemu/run_exec.go`) then refuses the result unless the guest reports the release in `internal/appliance/kernel.version`.

"The stock kernel has the needed feature" is not an exception. A failure to load the supplied kernel can leave the ISO kernel running, so checking the staged file on the host proves nothing, and the verdict would describe Alpine's kernel while reading as a verdict about Ze.

`internal/le/qemu/run.go` owns the boot plan and `internal/le/qemu/alltests.go` owns the functional-suite and integration-package populations. You MUST update those Go producers together when the VM contract changes. The VM's own contract is `docs/architecture/testing/qemu-integration.md`.

## How to Write a QEMU Integration Test

**A test that touches the kernel MUST carry `integration && linux`, and bare `linux` MUST be used only when the test imports linux-only types and makes no syscall.** Which tag runs where, and the file-name pattern each one takes, is `docs/architecture/testing/qemu-integration.md`.

**An integration test MUST NOT require physical hardware.** The QEMU VM provides the kernel features, so a virtual device stands in for the hardware. The substitute for each device is `docs/architecture/testing/qemu-integration.md`.

**A test whose prerequisite is absent MUST call `t.Skip`, never `t.Fatal`.** One test file runs in environments with different capabilities, and a fatal there reports a broken product for a missing capability. The worked example is `docs/architecture/testing/qemu-integration.md`.

**A new integration package MUST be added to `integrationPackages` in `internal/le/qemu/alltests.go`.** `./le qemu all-tests` runs that closed list, so a package absent from it never runs and nothing goes red.

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

## Interop Labs and Docker-Based Tests Need a QEMU Runner Too

**Every Linux-only interop lab that runs as Docker containers and depends on host-kernel features MUST also ship a QEMU-runnable path.** Treat "it is Linux-only, it needs the host kernel" as the trigger to build the QEMU runner, never as a reason to skip it: Docker Desktop's VM lacks the kernel modules and the Alpine QEMU VM has no Docker, so a Docker-only lab is unrunnable on the dev machine. The paired actions each lab ships are `docs/architecture/testing/qemu-integration.md`.

A lab that needs a QEMU runner MUST be built with all four steps below. Three
of them fail closed on their own; the fourth, registration, does not, so a lab
that skips it is invisible rather than red.

1. **Native netns evidence:** implement the lab under `internal/le` and register a named `./le deployment <verb>` or `./le qemu <verb>` action. Run Ze and the peer daemon in separate network namespaces joined by a veth, without Docker.
2. **Peer from Alpine packages:** install the peer daemon through the `packages` parameter of `./le qemu run`, or declare it in the dedicated native QEMU action. Use the same packaged peer in the Docker and QEMU proofs where Alpine supplies it.
3. **Runtime kernel, always:** pass Ze's staged runtime kernel through `./le qemu run kernel <vmlinuz>`. `Run.assertRuntimeKernel` refuses a guest whose `uname -r` does not match `internal/appliance/kernel.version`. Add every required `CONFIG_*` symbol to `gokrazy/kernel/runtime.config` and `gokrazy/kernel/runtime.require`.
4. **Registered action:** add the feature action to the owning Go action table and expose it through `./le qemu` or `./le deployment`. The bare area command is the inventory, and it MUST list the new action.

**A new interop lab MUST ship both native actions in the same change, and MUST add its row to the lab table in `docs/architecture/testing/qemu-integration.md`.**

**Probing the stock Alpine kernel proves nothing about a lab, and its result MUST NOT be recorded as a reason to skip step 3's `--kernel`.** A probe answers a question about Alpine, while the lab's verdict is about the kernel ze ships, so a green probe and a green lab on stock together establish only that Alpine works. A capability the probe found MUST be declared in `gokrazy/kernel/runtime.config` with its symbol in `gokrazy/kernel/runtime.require`, so the lab gets it from the kernel under test and a silent demotion to `=m` fails the build instead of the lab.

## What Actually Runs These Suites

**QEMU evidence is scheduled and advisory. You MUST NOT treat it as a blocking push gate, and you MUST NOT skip the focused QEMU proof for your change.** Which workflow runs which suite, and which of them block, is `docs/architecture/testing/ci-workflows.md`.

**Every registered `./le qemu` and `./le integration` action MUST be given a real caller in the same change**: a workflow job, another native action, or an explicit manual classification. No gate checks this direction today. `TestEveryWorkflowNativeActionExists` (`internal/le/workflowcheck/workflowcheck_test.go`) checks only the other one, that every action a workflow names is registered, so an action nobody calls stays green and runs nowhere. Which workflow job runs each action is `docs/architecture/testing/ci-workflows.md`.

## Initrd: Prefer Procfs/Sysfs Over External Commands

**An initrd operation MUST use its in-process Go replacement; it MUST NOT shell out to an external tool.** That covers bringing a link up, applying an address and a route, the DHCP lease, the image download, and mount, umount, loop, block-device ioctls, reboot and poweroff. The replacement for each operation, and the procfs or sysfs source for each state read, is `docs/architecture/appliance/installer-initrd.md`.

**Where a syscall is unavoidable, you MUST isolate it in a named `_linux.go` helper so the platform dependency is visible and testable behind a fake.** `internal/install/disk` and `cmd/ze-installer` MUST contain no `exec.Command` of an external binary; `./le qemu install-test` proves that the initrd boots and installs cleanly.

## Appliance Dependency Bumps

**Dismissing the alert leaves the stale manifest; a future advisory below the pin will re-fire on the same file. You MUST bump the pin instead of dismissing the alert: bumping removes the manifest at the source.**

**You MUST NOT try to convert to `go mod vendor`: `gok` cannot consume it. You MUST NOT hand-edit modcache go.sum hashes.**

1. **Find a fixed upstream version.** You MUST fetch the candidate `.mod` from the proxy (`https://proxy.golang.org/github.com/gokrazy/gokrazy/@v/<version>.mod` or `@latest`) and confirm it `require`s the fixed dependency version. Only then bump.
2. **You MUST bump the version string in the 7 builddir modules** under `gokrazy/ze/builddir/`: the `require` in `gokrazy` + `cmd/{dhcp,ntp,heartbeat,randomd}`, and the `replace` RHS in `serial-busybox` + `rtr7/kernel`. <!-- doc-links: ignore (cmd/{dhcp,ntp,heartbeat,randomd} are gokrazy submodules under gokrazy/ze/builddir/github.com/gokrazy/gokrazy/, not top-level cmd/) -->
3. **You MUST remove any now-false workaround pin/comment** (e.g. an explicit `x/net` pin added because "upstream pins the old version"). Verify it is safe: `go list -m <dep>` in each builddir MUST still resolve `>=` the fixed version via the new upstream `require`.
4. **You MUST regenerate the go.sums cleanly.** Delete the affected builddir `go.sum` files (filesystem removal, never `git rm`), then run `go mod download all` in each affected builddir. The sums regenerate from the new build list and prune the old version string. You MUST NOT hand-edit hashes.
5. **Re-vendor and prune.** The module download extracts the new version under `gokrazy/modcache/github.com/gokrazy/gokrazy@<new>/`. Remove the old `@<old>` directory. Confirm the working tree holds only the expected old-file deletions and new source.
6. **Refresh coupling.** Search for the old version string and update every document or spec that names the old module-cache path.
7. **Verify (BLOCKING).** Confirm the old version string is absent, the new committed `go.mod` names the fixed dependency, `ze appliance build` succeeds, and `./le deployment gokrazy-l2tp-ppp-test` boots the appliance. An image build alone is insufficient.

**A `SKIP` MUST NOT be treated as evidence.** Under a hardware accelerator the hugepage proof treats a no-answer as a FAIL; if it skips for want of KVM access, you MUST fix that (on Linux, group membership: `./le setup check`) and rerun.

**The re-vendor deletes ~60 tracked files and adds ~60 new ones. You MUST NOT use bare `git rm`/`git add`: you MUST stage the whole change through the commit-helper script at closure so the deletion and addition land in one commit.**

**Anything that downloads into `gokrazy/modcache/` MUST carry `-modcacherw` (`GOFLAGS=-modcacherw`).** Go's default read-only cache permissions leave directories `r-x`, which makes git unable to delete or overwrite modcache files on a later checkout or rebase. Which tools already set the flag, and how to repair a cache written without it, is `docs/architecture/appliance/gokrazy-build-pins.md`.

**An unexpected module version in `gokrazy/modcache/` MUST NOT be dismissed as cache noise.** A `github.com/ze-software/ze@v0.0.0-...` directory, or an off-pin copy of a builddir-pinned module, means a build resolved over the network instead of through the pins, so the version it built is not the version this repository chose. You MUST find the path that prepared that instance without the builddir and fix it, rather than deleting the directory. What each finding means, and what growth is expected instead, is `docs/architecture/appliance/gokrazy-build-pins.md`.

**You MUST NOT `rm -rf gokrazy/modcache`.** 60 tracked files live inside it (the gokrazy init source, whitelisted by `gokrazy/modcache/.gitignore`). You MUST delete named `@version` directories plus their `cache/download/<module>/@v/<version>.*` files, and confirm with `git status --porcelain gokrazy/` that nothing tracked moved.

**Cadence:** you MUST review the builddir pins **once per release cycle, and at minimum quarterly**, whichever comes first. Each review:

1. For the vendored gokrazy init and `rtr7/kernel`, you MUST fetch the latest upstream `.mod` from the proxy, as in step 1 of the bump runbook, and note whether a newer commit carries security-relevant fixes.
2. If a fix applies, you MUST run the bump runbook. If not, you MUST record the review date so the next reviewer knows the pins were checked, not forgotten.
3. You MUST re-confirm that the GPLv2 source-offer sign-off is still current.

**The pins MUST move only through the runbook; they MUST NOT move through a bot PR.**

**Before the appliance image is distributed to third parties, a GPLv2 source-offer sign-off MUST be produced and recorded, and it MUST be re-confirmed at each pin review.** None is recorded today. That is a licensing call, not an engineering one, so this point flags the obligation and does not adjudicate it. What the image ships and why the offer is owed is `docs/architecture/appliance/gokrazy-build-pins.md`.

**You MUST keep the pseudo-versions. You MUST re-check for a first tag when bumping any of these, and move the pin to a tag the day upstream cuts one.** Until then a pseudo-version is the only available form and is legal. The note exists so a future reviewer does not "fix" a non-problem or assume the pins were never examined.
