---
kind: directive
level: MUST
stage:
---
**Both targets boot ze's own runtime kernel, never the stock Alpine one
(2026-08-07).** Each passes `--kernel tmp/kernel/vmlinuz` and refuses to start
without it, so `make ze-kernel-vmlinuz KERNEL_ARCH=<amd64|arm64>` is a
precondition of each. That command costs a copy on a cache hit and only builds
on a miss. `make ze-kernel` also satisfies it and additionally assembles the
gokrazy kernel package, which needs the module cache `make ze-gokrazy-deps`
downloads and a VM boot never reads.

**A caller that runs one of these targets MUST supply the kernel itself
(2026-08-11).** The guard denies before the VM, so a caller that cannot stage a
kernel runs no test at all. `.github/workflows/qemu-nightly.yml` was that caller
for four nights: it never staged one, and its job-level `continue-on-error`
reported every failed run as `success`.
`TestQemuKernelPreconditionIsMetInTheSameJob`
(`scripts/dev/github_workflows_test.go`) now derives both sides from the make
fragments. It fails when a workflow JOB runs a guarded target and no target in
that same job stages a kernel.

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
