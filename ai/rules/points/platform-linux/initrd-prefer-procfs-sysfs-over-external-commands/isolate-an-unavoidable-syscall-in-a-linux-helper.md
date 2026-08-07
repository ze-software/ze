---
kind: directive
level:
stage:
---
**Where a syscall is unavoidable, isolate it in a named `_linux.go` helper so the platform dependency is visible and testable behind a fake.** `internal/install/disk` and `cmd/ze-installer` must contain no `exec.Command` of an external binary; a QEMU install (`make ze-install-qemu-test`) proves it boots and installs cleanly.
