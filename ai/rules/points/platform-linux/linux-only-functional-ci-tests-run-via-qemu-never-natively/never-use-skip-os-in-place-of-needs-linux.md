---
kind: directive
level:
stage:
---
**Do NOT use `skip-os:value=darwin` as a substitute for `needs-linux`:** `skip-os` says "do not run here", whereas `needs-linux` documents intent ("this is a Linux-only test, validated in QEMU") and keeps the test in the QEMU suite.
