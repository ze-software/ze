---
kind: directive
level: MUST
stage:
---
**Every functional QEMU proof MUST boot Ze's runtime kernel, never the stock Alpine kernel, and the caller MUST supply that kernel path.** `./le qemu run kernel <vmlinuz> command "<command>"` owns the Alpine image, the QEMU process, the bounded waits, SSH execution and cleanup; `Run.assertRuntimeKernel` (`internal/le/qemu/run_exec.go`) then refuses the result unless the guest reports the release in `internal/appliance/kernel.version`.

"The stock kernel has the needed feature" is not an exception. A failure to load the supplied kernel can leave the ISO kernel running, so checking the staged file on the host proves nothing, and the verdict would describe Alpine's kernel while reading as a verdict about Ze.

`internal/le/qemu/run.go` owns the boot plan and `internal/le/qemu/alltests.go` owns the functional-suite and integration-package populations. You MUST update those Go producers together when the VM contract changes. The VM's own contract is `docs/architecture/testing/qemu-integration.md`.
