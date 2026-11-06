---
kind: directive
level: MUST
stage:
---
**Every functional QEMU proof MUST boot Ze's runtime kernel, never the stock Alpine kernel, and the caller MUST supply that kernel path.** `./le test qemu docker-lab kernel <vmlinuz> harness command "<command>"` is the proof route: it boots Ze's image, owns the QEMU process, the bounded waits, SSH execution and cleanup, and refuses a kernel whose bytes differ from the image's runtime kernel, then a guest whose release, PID 1 or root filesystem is not Ze's (`dockerLabCommandKernel` and `Run.dockerLabCommand`, `internal/le/test/qemu/dockerlab_command.go`). A guest not yet moved off Alpine boots through `./le test qemu run kernel <vmlinuz> command "<command>"`, where `Run.assertRuntimeKernel` (`internal/le/test/qemu/run_exec.go`) refuses the result unless the guest reports the release in `internal/appliance/kernel.version`.

"The stock kernel has the needed feature" is not an exception. A failure to load the supplied kernel can leave the ISO kernel running, so checking the staged file on the host proves nothing, and the verdict would describe Alpine's kernel while reading as a verdict about Ze.

`internal/le/test/qemu/run.go` owns the boot plan and `internal/le/test/qemu/alltests.go` owns the functional-suite and integration-package populations. You MUST update those Go producers together when the VM contract changes. The VM's own contract is `docs/architecture/testing/qemu-integration.md`.
