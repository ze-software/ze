---
kind: directive
level: MUST
stage:
---
**Every functional QEMU proof MUST boot Ze's runtime kernel, never the stock Alpine kernel.**

`./le qemu run kernel <vmlinuz> command "<command>"` owns the Alpine image, QEMU process, bounded waits, SSH execution, and cleanup. When `kernel` is present, `Run.assertRuntimeKernel` (`internal/le/qemu/run_exec.go`) reads `internal/appliance/kernel.version` and refuses the result unless the guest reports that release. A failure to load the supplied kernel can leave the ISO kernel running, so checking the staged file on the host is insufficient.

The caller MUST supply the runtime kernel path. "The stock kernel has the needed feature" is not an exception: the verdict would describe Alpine's kernel while reading as a verdict about Ze. A focused proof passes its command through `./le qemu run`; the full in-guest test population is `./le qemu all-tests`.

`internal/le/qemu/run.go` owns the boot plan, and `internal/le/qemu/alltests.go` owns the functional-suite and integration-package populations. Update those Go producers together when the VM contract changes. `TestTheAreaPublishesTheNativeActions` and the all-tests coverage checks in `internal/le/qemu` refuse a command or suite that leaves the native inventory.
