---
kind: directive
level: MUST
stage:
---
**Where a syscall is unavoidable, you MUST isolate it in a named `_linux.go` helper so the platform dependency is visible and testable behind a fake.** `internal/install/disk` and `cmd/ze-installer` MUST contain no `exec.Command` of an external binary; a QEMU install (`make ze-qemu-install-test`) proves it boots and installs cleanly.
