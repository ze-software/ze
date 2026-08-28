---
kind: directive
level: MUST NOT
stage:
---
**QEMU evidence is scheduled and advisory.** `.github/workflows/qemu-nightly.yml` drives `./le qemu run` with Ze's runtime kernel and invokes `./le qemu all-tests` inside the guest. You MUST NOT treat it as a blocking push gate or skip the focused QEMU proof for your change.
