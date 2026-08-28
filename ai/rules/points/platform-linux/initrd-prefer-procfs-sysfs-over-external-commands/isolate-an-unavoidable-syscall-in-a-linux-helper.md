---
kind: directive
level: MUST
stage:
---
**Where a syscall is unavoidable, you MUST isolate it in a named `_linux.go` helper so the platform dependency is visible and testable behind a fake.** `internal/install/disk` and `cmd/ze-installer` MUST contain no `exec.Command` of an external binary; `./le qemu install-test` proves that the initrd boots and installs cleanly.
