---
kind: directive
level:
stage:
---
- `make ze-qemu-needs-linux-test` -- the tight loop. Sets `ZE_QEMU_LINUX_ONLY=1`, which flips the runner to skip every test that is NOT `needs-linux`, so the VM spends its whole boot only on the Linux-only surface. Use this while iterating on a Linux-only feature.
- `make ze-qemu-all-test` -- the full pass. Runs every functional suite in the VM, so `needs-linux` tests are exercised alongside everything else.
