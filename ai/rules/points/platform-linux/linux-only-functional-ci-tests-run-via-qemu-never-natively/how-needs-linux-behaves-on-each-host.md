---
kind: directive
level:
stage:
---
- On a non-Linux host (`GOOS != linux`): the runner sets `SkipReason` and the test reports **SKIP**, never FAIL. Native `make ze-verify` / `make ze-functional-test` on darwin stays green without running the unsupported test.
- Inside the QEMU Alpine VM (`GOOS == linux`): the directive is inert, so the same `.ci` test runs for real against the Linux kernel.
