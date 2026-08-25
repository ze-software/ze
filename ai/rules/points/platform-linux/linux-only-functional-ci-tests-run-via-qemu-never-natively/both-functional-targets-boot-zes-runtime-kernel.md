---
kind: directive
level: MUST
stage:
---
**EVERY QEMU target boots ze's own runtime kernel, never the stock Alpine one.**
All thirteen `qemu-run.py` invocations in `mk/test-integration.mk` pass
`--kernel $(ZE_QEMU_KERNEL)` and refuse to start without it, so
`make ze-kernel-vmlinuz-stage KERNEL_ARCH=<amd64|arm64>` is a precondition of
each. That command costs a copy on a cache hit and only builds on a miss. `make
ze-kernel-build` also satisfies it and additionally assembles the gokrazy kernel
package, which needs the module cache `make ze-gokrazy-deps-download` downloads
and a VM boot never reads.

**A QEMU target MUST NOT be written to boot stock.** "It only needs features
stock has" MUST NOT be offered as a reason. Seven targets booted stock until
2026-08-24. Six stated no reason at all. The seventh cited two of the other six,
which is a circle rather than a reason. A target's verdict is about the kernel
it ran on. A target on stock therefore reports about Alpine, and reads as a
report about ze. The cost that appeared to justify it does not exist: the ~30
minutes is a COLD-CACHE cost, paid once per kernel config change and shared by
every target. An edit under `tools/kernel-builder/` also invalidates that cache
key, because the builder decides what the kernel is.

**`qemu-run.py` reads `uname -r` in the booted guest and MUST refuse to continue
unless it matches `internal/appliance/kernel.version`** (`assert_runtime_kernel_booted`).
The guard below runs on the HOST before the VM exists, so it proves the staged
FILE and cannot see what QEMU booted. A `-kernel` QEMU fails to load leaves the
ISO's own kernel running, and only the guest check sees that.

**A test that drives a real `make ze-kernel-build` or `ze-kernel-vmlinuz-stage`
MUST set `XDG_CACHE_HOME` to its own work directory.** The stage's cache-MISS
branch POPULATES the durable cache that `ze-qemu-kernel-guard` reads. Unset, a
fixture's fake kernel lands in the developer's real
`~/.cache/ze/runtime-kernel/<key>`. The guard then compares a real staged kernel
against that fake and refuses every QEMU target. The next stage is worse: it
materializes the fake and reports a match over a file QEMU cannot boot. Measured
2026-08-24: `test/install/ze-kernel-overlay.ci` left a 518-byte vmlinuz there.
Saving and restoring `tmp/kernel/` does not reach it.

**A caller that runs one of these targets MUST supply the kernel itself.** The
guard denies before the VM, so a caller that cannot stage a kernel runs no test.
`TestQemuKernelPreconditionIsMetInTheSameJob`
(`scripts/dev/github_workflows_test.go`) derives both sides from the make
fragments. It fails when a workflow JOB runs a guarded target and no target in
that same job stages a kernel.

**The guard compares, it does not merely check existence.** `GOKRAZY_ARCH`
defaults to `amd64` on every host while `QEMU_GOARCH` follows `uname`, so a bare
`make ze-kernel-build` on an Apple Silicon machine stages an amd64 vmlinuz that a
`test -f` accepts and QEMU then fails to boot, with no line naming the
architecture. `ze-qemu-kernel-guard` (`mk/test-integration.mk`) compares the
staged kernel against the architecture-keyed durable cache entry instead, which
also catches a kernel staged before a config fragment changed. A missing or
mismatched kernel is an error exit, never a silent fall back to stock.

**All thirteen kernel-consuming targets use that one guard.** **A target that
uses it MUST declare `: ze-host-build`.** The guard's first command execs that
binary. Without the prerequisite the guard still denies, but it names the wrong
cause. The three properties travel together, and each has its own check in
`scripts/evidence/qemu_kernel_wiring_test.go`:

| Property | Check |
|----------|-------|
| the `--kernel` flag | `TestQemuFunctionalTargetsBootTheRuntimeKernel` |
| the guard | `TestQemuTargetsGuardTheStagedKernel` |
| the `ze-host-build` prerequisite | `TestQemuTargetsDependOnHostBuild` |

All three derive the target list from the makefile rather than from a written
list. A fourteenth target is therefore checked the day it is written. Each names
which of the three went missing, instead of one compound red.

**Two files carry the default kernel behavior and they MUST move together:**
`mk/test-integration.mk` and `scripts/evidence/qemu-all-tests.sh`. The script
default wins when the script is invoked directly.
