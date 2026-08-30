---
kind: directive
level: MUST NOT
stage:
---
**`option=skip-os:value=darwin` MUST NOT stand in for `option=needs-linux`, and it MUST NOT stand in for `caps=`.** `skip-os` says "do not run here", so it hides the test from macOS and therefore RUNS it, unprivileged, on the Linux CI runner, which is exactly where it cannot pass. `needs-linux` states the intent, keeps the test in the QEMU suite, and `caps=` declares the capability the host has to hold. When the reason a test cannot run on macOS is a capability, you MUST declare that capability.
